package sched

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"testing"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/conf"
)

func init() {
	log.SetOutput(ioutil.Discard)
}

type schedState struct {
	key, status string
}

type schedTest struct {
	conf    string
	queries map[string]*opentsdb.Response
	// state -> active
	state map[schedState]bool
}

func testSched(t *testing.T, st *schedTest) {
	const addr = "localhost:18070"
	confs := "tsdbHost = " + addr + "\n" + st.conf
	start := time.Date(2000, time.January, 1, 12, 0, 0, 0, time.UTC)
	c, err := conf.New("testconf", confs)
	if err != nil {
		t.Error(err)
		t.Logf("conf:\n%s", confs)
		return
	}
	c.StateFile = ""
	mux := http.NewServeMux()
	mux.HandleFunc("/api/query", func(w http.ResponseWriter, r *http.Request) {
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
			resp = append(resp, q)
		}
		if err := json.NewEncoder(w).Encode(&resp); err != nil {
			log.Fatal(err)
		}
	})
	server := NewServer(addr, mux)
	go server.ListenAndServe()
	s := new(Schedule)
	s.Init(c)
	s.Check(start)
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
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCrit(t *testing.T) {
	testSched(t, &schedTest{
		conf: `alert a {
			crit = avg(q("avg:m{a=b}", "5m", "")) > 0
		}`,
		queries: map[string]*opentsdb.Response{
			`q("avg:m{a=b}", "2000/01/01-11:55:00", "2000/01/01-12:00:00")`: {
				Metric: "m",
				Tags:   opentsdb.TagSet{"a": "b"},
				DPS:    map[string]opentsdb.Point{"0": 1},
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
		queries: map[string]*opentsdb.Response{
			`q("sum:m{a=*}", "2000/01/01-11:59:00", "2000/01/01-12:00:00")`: {
				Metric: "m",
				Tags:   opentsdb.TagSet{"a": "b"},
				DPS:    map[string]opentsdb.Point{"0": 1},
			},
			`q("sum:m{a=*}", "9.4672434e+08", "9.467244e+08")`: {
				Metric: "m",
				Tags:   opentsdb.TagSet{"a": "c"},
				DPS:    map[string]opentsdb.Point{"0": 1},
			},
		},
	})
}
