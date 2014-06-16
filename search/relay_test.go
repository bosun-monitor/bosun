package search

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"log"
	"net/http"
	"sort"
	"testing"
)

func TestRelay(t *testing.T) {
	relayAddr := ":52366"
	addr := ":52367"
	httpAddr := fmt.Sprintf("http://%s/api/put", addr)
	http.HandleFunc("/", Handle(addr))
	go func() {
		err := http.ListenAndServe(addr, nil)
		log.Fatal(err)
	}()
	relayMux := http.NewServeMux()
	relayMux.HandleFunc("/", relayHandle)
	go func() {
		err := http.ListenAndServe(relayAddr, relayMux)
		log.Fatal(err)
	}()

	body := []byte(`[{
		"timestamp": 1,
		"metric": "test.metric",
		"value": 123.45,
		"tags": {
			"host": "relay-test-host",
			"other": "something"
		}
	}]`)
	if _, err := http.Post(httpAddr, "application/json", bytes.NewBuffer(body)); err != nil {
		t.Fatal(err)
	}

	bodygzip := []byte(`[{
		"timestamp": 1,
		"metric": "gzip.test",
		"value": "345",
		"tags": {
			"host": "host-gzip-test",
			"gzipped": "yup"
		}
	}]`)
	buf := new(bytes.Buffer)
	gw := gzip.NewWriter(buf)
	gw.Write(bodygzip)
	gw.Flush()
	if _, err := http.Post(httpAddr, "application/json", bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatal(err)
	}

	m := UniqueMetrics()
	sort.Strings(m)
	if len(m) != 2 || m[0] != "gzip.test" || m[1] != "test.metric" {
		t.Errorf("bad um: %v", m)
	}
	m = TagValuesByMetricTagKey("gzip.test", "gzipped")
	if len(m) != 1 || m[0] != "yup" {
		t.Errorf("bad tvbmtk: %v", m)
	}
	m = TagKeysByMetric("test.metric")
	sort.Strings(m)
	if len(m) != 2 || m[0] != "host" || m[1] != "other" {
		t.Errorf("bad tkbm: %v", m)
	}
}

func relayHandle(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(204)
}
