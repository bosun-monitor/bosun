package sched

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/bosun-monitor/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/bosun-monitor/bosun/conf"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

type schedState struct {
	key, status string
}

type schedTest struct {
	conf    string
	queries map[string]opentsdb.ResponseSet
	// state -> active
	state map[schedState]bool
}

func testSched(t *testing.T, st *schedTest) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req opentsdb.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Fatal(err)
		}
		var resp opentsdb.ResponseSet
		for _, rq := range req.Queries {
			qs := fmt.Sprintf(`q("%s", "%v", "%v")`, rq, req.Start, req.End)
			q := st.queries[qs]
			if q == nil {
				t.Errorf("unknown query: %s", qs)
				return
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
	start := time.Date(2000, time.January, 1, 12, 0, 0, 0, time.UTC)
	c, err := conf.New("testconf", confs)
	if err != nil {
		t.Error(err)
		t.Logf("conf:\n%s", confs)
		return
	}
	c.StateFile = ""
	time.Sleep(time.Millisecond * 250)
	s := new(Schedule)
	s.Init(c)
	s.Check(nil, start)
	groups, err := s.MarshalGroups("")
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
}

func TestCrit(t *testing.T) {
	testSched(t, &schedTest{
		conf: `alert a {
			crit = avg(q("avg:m{a=b}", "5m", "")) > 0
		}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("avg:m{a=b}", "2000/01/01-11:55:00", "2000/01/01-12:00:00")`: {
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
}

func TestBandDisableUnjoined(t *testing.T) {
	testSched(t, &schedTest{
		conf: `alert a {
			$sum = "sum:m{a=*}"
			$band = band($sum, "1m", "1h", 1)
			crit = avg(q($sum, "1m", "")) > avg($band) + dev($band)
		}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("sum:m{a=*}", "2000/01/01-11:59:00", "2000/01/01-12:00:00")`: {
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
	testSched(t, &schedTest{
		conf: `alert a {
			crit = count("sum:m{a=*}", "1m", "") != 2
		}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("sum:m{a=*}", "2000/01/01-11:59:00", "2000/01/01-12:00:00")`: {
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
