package sched

import (
	"encoding/json"
	"log"
	"net/http"
	"testing"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/conf"
)

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
			q := st.queries[rq.String()]
			if q == nil {
				t.Errorf("unknown query: %s", rq)
				return
			}
			resp = append(resp, q)

		}
		if err := json.NewEncoder(w).Encode(&resp); err != nil {
			log.Fatal(err)
		}
	})
	go func() {
		log.Fatal(http.ListenAndServe(addr, mux))
	}()
	s := new(Schedule)
	s.Init(c)
	s.Check()
	groups, err := s.MarshalGroups("a")
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
			t.Errorf("unfound state: %s, %s", g.AlertKey, g.Status)
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
		queries: map[string]*opentsdb.Response{
			"avg:m{a=b}": {
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
