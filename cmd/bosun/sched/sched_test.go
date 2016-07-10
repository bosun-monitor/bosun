package sched

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"
	"time"

	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/native"
	"bosun.org/cmd/bosun/database"
	"bosun.org/cmd/bosun/database/test"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"github.com/MiniProfiler/go/miniprofiler"
)

func init() {
	slog.Set(&slog.StdLog{Log: log.New(ioutil.Discard, "", log.LstdFlags)})
	log.SetOutput(ioutil.Discard)
}

type schedState struct {
	key, status string
}

type schedTest struct {
	conf    string
	queries map[string]opentsdb.ResponseSet
	// state -> active
	state   map[schedState]bool
	touched map[models.AlertKey]time.Time
}

// test-only function to check all alerts immediately.
func check(s *Schedule, t time.Time) {
	names := []string{}
	for a := range s.Conf.GetAlerts() {
		names = append(names, a)
	}
	sort.Strings(names)
	for _, n := range names {
		a := s.Conf.GetAlerts()[n]
		fmt.Println(a)
		s.ctx.runTime = t
		s.checkAlert(a)
	}
}

var db database.DataAccess

func setup() func() {
	testDb, closer := dbtest.StartTestRedis(9992)
	db = testDb
	return closer
}

func initSched(c conf.ConfProvider) (*Schedule, error) {
	s := new(Schedule)
	s.DataAccess = db
	err := s.Init(c)
	return s, err
}

func testSched(t *testing.T, st *schedTest) (s *Schedule) {
	bosunStartupTime = time.Date(1900, 0, 0, 0, 0, 0, 0, time.UTC) //pretend we've been running for a while.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req opentsdb.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Fatal(err)
		}
		var resp opentsdb.ResponseSet
		for _, rq := range req.Queries {
			qs := fmt.Sprintf(`q("%s", "%v", "%v")`, rq, req.Start, req.End)
			q, ok := st.queries[qs]
			if !ok {
				t.Errorf("unknown query: %s", qs)
				return
			}
			if q == nil {
				return // Put nil entry in map to simulate opentsdb error.
			}
			resp = append(resp, q...)
		}
		if err := json.NewEncoder(w).Encode(&resp); err != nil {
			log.Fatal(err)
		}
	}))
	defer ts.Close()
	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	confs := "tsdbHost = " + u.Host + "\n" + st.conf
	c, err := native.NewNativeConf("testconf", confs)
	if err != nil {
		t.Error(err)
		t.Logf("conf:\n%s", confs)
		return
	}

	time.Sleep(time.Millisecond * 250)
	s, _ = initSched(c)
	for ak, time := range st.touched {
		s.DataAccess.State().TouchAlertKey(ak, time)
	}
	check(s, queryTime)
	groups, err := s.MarshalGroups(new(miniprofiler.Profile), "")
	if err != nil {
		t.Error(err)
		return
	}
	var check func(g *StateGroup)
	check = func(g *StateGroup) {
		for _, c := range g.Children {
			check(c)
		}
		if g.AlertKey == "" {
			return
		}
		ss := schedState{string(g.AlertKey), g.Status.String()}
		v, ok := st.state[ss]
		if !ok {
			t.Errorf("unexpected state: %s, %s", g.AlertKey, g.Status)
			return
		}
		if v != g.Active {
			t.Errorf("bad active: %s, %s", g.AlertKey, g.Status)
			return
		}
		delete(st.state, ss)
	}
	for _, v := range groups.Groups.NeedAck {
		check(v)
	}
	for _, v := range groups.Groups.Acknowledged {
		check(v)
	}
	for k := range st.state {
		t.Errorf("unused state: %s", k)
	}
	return s
}

var queryTime = time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)
var window5Min = `"9.467277e+08", "9.46728e+08"`

func TestCrit(t *testing.T) {
	defer setup()()
	s := testSched(t, &schedTest{
		conf: `alert a {
			crit = avg(q("avg:m{a=b}", "5m", "")) > 0
		}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("avg:m{a=b}", ` + window5Min + `)`: {
				{
					Metric: "m",
					Tags:   opentsdb.TagSet{"a": "b"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
			},
		},
		state: map[schedState]bool{
			schedState{"a{a=b}", "critical"}: true,
		},
	})
	if !s.AlertSuccessful("a") {
		t.Fatal("Expected alert a to be successful")
	}
}

func TestBandDisableUnjoined(t *testing.T) {
	defer setup()()
	testSched(t, &schedTest{
		conf: `alert a {
			$sum = "sum:m{a=*}"
			$band = band($sum, "1m", "1h", 1)
			crit = avg(q($sum, "1m", "")) > avg($band) + dev($band)
		}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("sum:m{a=*}", "9.4672794e+08", "9.46728e+08")`: {
				{
					Metric: "m",
					Tags:   opentsdb.TagSet{"a": "b"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
			},
			`q("sum:m{a=*}", "9.4672434e+08", "9.467244e+08")`: {
				{
					Metric: "m",
					Tags:   opentsdb.TagSet{"a": "c"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
			},
		},
	})
}

func TestCount(t *testing.T) {
	defer setup()()
	testSched(t, &schedTest{
		conf: `alert a {
			crit = count("sum:m{a=*}", "5m", "") != 2
		}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("sum:m{a=*}", ` + window5Min + `)`: {
				{
					Metric: "m",
					Tags:   opentsdb.TagSet{"a": "b"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
				{
					Metric: "m",
					Tags:   opentsdb.TagSet{"a": "c"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
			},
		},
	})
}

func TestUnknown(t *testing.T) {
	defer setup()()
	testSched(t, &schedTest{
		conf: `alert a {
			crit = avg(q("avg:m{a=*}", "5m", "")) > 0
		}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("avg:m{a=*}", ` + window5Min + `)`: {},
		},
		state: map[schedState]bool{
			schedState{"a{a=b}", "unknown"}: true,
		},
		touched: map[models.AlertKey]time.Time{
			"a{a=b}": queryTime.Add(-10 * time.Minute),
			"a{a=c}": queryTime.Add(-9 * time.Minute),
		},
	})
}

func TestUnknown_HalfFreq(t *testing.T) {
	defer setup()()
	testSched(t, &schedTest{
		conf: `alert a {
			crit = avg(q("avg:m{a=*}", "5m", "")) > 0
			runEvery = 2
		}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("avg:m{a=*}", ` + window5Min + `)`: {},
		},
		state: map[schedState]bool{
			schedState{"a{a=b}", "unknown"}: true,
		},
		touched: map[models.AlertKey]time.Time{
			"a{a=b}": queryTime.Add(-20 * time.Minute),
			"a{a=c}": queryTime.Add(-19 * time.Minute),
		},
	})
}

func TestUnknown_WithError(t *testing.T) {
	defer setup()()

	s := testSched(t, &schedTest{
		conf: `alert a {
			crit = avg(q("avg:m{a=*}", "5m", "")) > 0
		}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("avg:m{a=*}", ` + window5Min + `)`: nil,
		},
		state: map[schedState]bool{},
		touched: map[models.AlertKey]time.Time{
			"a{a=b}": queryTime.Add(-10 * time.Minute),
		},
	})

	if s.AlertSuccessful("a") {
		t.Fatal("Expected alert a to be in a failed state")
	}
}

func TestRename(t *testing.T) {
	defer setup()()
	testSched(t, &schedTest{
		conf: `
		alert ping.host {
  
    $q = max(rename(q("sum:bosun.ping.timeout{dst_host=*,host=ny-kbrandt02}", "5m", ""), "host=source,dst_host=host"))
    warn = $q
}

		alert os.cpu {
    			depends = max(rename(q("sum:bosun.ping.timeout{dst_host=*,host=ny-kbrandt02}", "5m", ""), "host=source,dst_host=host"))
    			$q = avg(q("avg:os.cpu{host=*}", "5m", ""))
    			warn = $q < 99
			}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("sum:bosun.ping.timeout{dst_host=*,host=ny-kbrandt02}", ` + window5Min + `)`: {
				{
					Metric: "bosun.ping.timeout",
					Tags:   opentsdb.TagSet{"host": "ny-kbrandt02", "dst_host": "ny-web01"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
				{
					Metric: "bosun.ping.timeout",
					Tags:   opentsdb.TagSet{"host": "ny-kbrandt02", "dst_host": "ny-web02"},
					DPS:    map[string]opentsdb.Point{"0": 0},
				},
				{
					Metric: "bosun.ping.timeout",
					Tags:   opentsdb.TagSet{"host": "ny-kbrandt02", "dst_host": "ny-kbrandt02"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
			},
			`q("avg:os.cpu{host=*}", ` + window5Min + `)`: {
				{
					Metric: "os.cpu",
					Tags:   opentsdb.TagSet{"host": "ny-web01"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
				{
					Metric: "os.cpu",
					Tags:   opentsdb.TagSet{"host": "ny-web02"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
			},
		},
		state: map[schedState]bool{
			schedState{"ping.host{host=ny-kbrandt02,source=ny-kbrandt02}", "warning"}: true,
			schedState{"ping.host{host=ny-web01,source=ny-kbrandt02}", "warning"}:     true,
			schedState{"os.cpu{host=ny-web02}", "warning"}:                            true,
		},
	})
}

func TestUnknownsAreNormal(t *testing.T) {
	defer setup()()
	testSched(t, &schedTest{
		conf: `alert a {
            unknownIsNormal = true
            crit = avg(q("avg:m{a=*}", "5m", "")) > 0
		}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("avg:m{a=*}", ` + window5Min + `)`: {},
		},
		state: map[schedState]bool{},
		touched: map[models.AlertKey]time.Time{
			"a{a=b}": queryTime.Add(-10 * time.Minute),
			"a{a=c}": queryTime.Add(-9 * time.Minute),
		},
	})
}
