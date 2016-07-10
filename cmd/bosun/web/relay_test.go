package web

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"testing"
	"time"

	"bosun.org/cmd/bosun/conf/native"
	"bosun.org/cmd/bosun/database"
	"bosun.org/cmd/bosun/database/test"
)

var testData database.DataAccess

func TestMain(m *testing.M) {
	var closeF func()
	testData, closeF = dbtest.StartTestRedis(9991)
	status := m.Run()
	closeF()
	os.Exit(status)
}

func TestRelay(t *testing.T) {
	schedule.DataAccess = testData
	schedule.Init(new(native.NativeConf))
	rs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(204)
	}))
	defer rs.Close()
	rurl, err := url.Parse(rs.URL)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(Relay(rurl.Host))
	defer ts.Close()

	body := []byte(`[{
		"timestamp": 1,
		"metric": "no-gzip-works",
		"value": 123.45,
		"tags": {
			"host": "host.no.gzip",
			"other": "something"
		}
	}]`)
	if _, err := http.Post(ts.URL, "application/json", bytes.NewBuffer(body)); err != nil {
		t.Fatal(err)
	}

	bodygzip := []byte(`[{
		"timestamp": 1,
		"metric": "gzip-works",
		"value": 345,
		"tags": {
			"host": "host.gzip",
			"gzipped": "yup"
		}
	}]`)
	buf := new(bytes.Buffer)
	gw := gzip.NewWriter(buf)
	gw.Write(bodygzip)
	gw.Flush()
	if _, err := http.Post(ts.URL, "application/json", bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Second)

	m, _ := schedule.Search.UniqueMetrics(0)
	sort.Strings(m)
	if len(m) != 2 || m[0] != "gzip-works" || m[1] != "no-gzip-works" {
		t.Errorf("bad um: %v", m)
	}
	m, _ = schedule.Search.TagValuesByMetricTagKey("gzip-works", "gzipped", 0)
	if len(m) != 1 || m[0] != "yup" {
		t.Errorf("bad tvbmtk: %v", m)
	}
	m, _ = schedule.Search.TagKeysByMetric("no-gzip-works")
	sort.Strings(m)
	if len(m) != 2 || m[0] != "host" || m[1] != "other" {
		t.Errorf("bad tkbm: %v", m)
	}
}
