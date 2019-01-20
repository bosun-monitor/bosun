package opentsdb

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"bosun.org/cmd/bosun/expr"
	opentsdb "bosun.org/opentsdb"
	"github.com/influxdata/influxdb/client/v2"
)

var queryTime = time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)

func TestQueryExpr(t *testing.T) {
	queries := map[string]opentsdb.ResponseSet{
		`q("avg:m{a=*}", "9.467277e+08", "9.46728e+08")`: {
			{
				Metric: "m",
				Tags:   opentsdb.TagSet{"a": "b"},
				DPS:    map[string]opentsdb.Point{"0": 0, "1": 3},
			},
			{
				Metric: "m",
				Tags:   opentsdb.TagSet{"a": "c"},
				DPS:    map[string]opentsdb.Point{"5": 1, "7": 4},
			},
		},

		`q("avg:m{a=*}", "9.467241e+08", "9.467244e+08")`: {
			{
				Metric: "m",
				Tags:   opentsdb.TagSet{"a": "b"},
				DPS:    map[string]opentsdb.Point{"0": 1, "1": 2},
			},
			{
				Metric: "m",
				Tags:   opentsdb.TagSet{"a": "c"},
				DPS:    map[string]opentsdb.Point{"3": 7, "1": 8},
			},
		},
		`q("avg:m{a=*}", "9.467205e+08", "9.467208e+08")`: {
			{
				Metric: "m",
				Tags:   opentsdb.TagSet{"a": "b"},
				DPS:    map[string]opentsdb.Point{"2": 6, "3": 4},
			},
			{
				Metric: "m",
				Tags:   opentsdb.TagSet{"a": "d"},
				DPS:    map[string]opentsdb.Point{"8": 8, "9": 9},
			},
		},
	}
	d := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	tests := map[string]map[string]expr.Value{
		`window("avg:m{a=*}", "5m", "1h", 2, "max")`: {
			"a=b": expr.Series{
				d:                      2,
				d.Add(time.Second * 2): 6,
			},
			"a=c": expr.Series{
				d.Add(time.Second * 1): 8,
			},
			"a=d": expr.Series{
				d.Add(time.Second * 8): 9,
			},
		},
		`window("avg:m{a=*}", "5m", "1h", 2, "avg")`: {
			"a=b": expr.Series{
				d:                      1.5,
				d.Add(time.Second * 2): 5,
			},
			"a=c": expr.Series{
				d.Add(time.Second * 1): 7.5,
			},
			"a=d": expr.Series{
				d.Add(time.Second * 8): 8.5,
			},
		},
		`over("avg:m{a=*}", "5m", "1h", 3)`: {
			"a=b,shift=0s": expr.Series{
				d:                      0,
				d.Add(time.Second * 1): 3,
			},
			"a=b,shift=1h0m0s": expr.Series{
				d.Add(time.Hour):                 1,
				d.Add(time.Hour + time.Second*1): 2,
			},
			"a=b,shift=2h0m0s": expr.Series{
				d.Add(time.Hour*2 + time.Second*2): 6,
				d.Add(time.Hour*2 + time.Second*3): 4,
			},
			"a=c,shift=0s": expr.Series{
				d.Add(time.Second * 5): 1,
				d.Add(time.Second * 7): 4,
			},
			"a=c,shift=1h0m0s": expr.Series{
				d.Add(time.Hour + time.Second*3): 7,
				d.Add(time.Hour + time.Second*1): 8,
			},
			"a=d,shift=2h0m0s": expr.Series{
				d.Add(time.Hour*2 + time.Second*8): 8,
				d.Add(time.Hour*2 + time.Second*9): 9,
			},
		},
		`band("avg:m{a=*}", "5m", "1h", 2)`: {
			"a=b": expr.Series{
				d:                      1,
				d.Add(time.Second * 1): 2,
				d.Add(time.Second * 2): 6,
				d.Add(time.Second * 3): 4,
			},
			"a=c": expr.Series{
				d.Add(time.Second * 3): 7,
				d.Add(time.Second * 1): 8,
			},
			"a=d": expr.Series{
				d.Add(time.Second * 8): 8,
				d.Add(time.Second * 9): 9,
			},
		},
		`shiftBand("avg:m{a=*}", "5m", "1h", 2)`: {
			"a=b,shift=1h0m0s": expr.Series{
				d.Add(time.Hour):                 1,
				d.Add(time.Hour + time.Second*1): 2,
			},
			"a=b,shift=2h0m0s": expr.Series{
				d.Add(time.Hour*2 + time.Second*2): 6,
				d.Add(time.Hour*2 + time.Second*3): 4,
			},
			"a=c,shift=1h0m0s": expr.Series{
				d.Add(time.Hour + time.Second*3): 7,
				d.Add(time.Hour + time.Second*1): 8,
			},
			"a=d,shift=2h0m0s": expr.Series{
				d.Add(time.Hour*2 + time.Second*8): 8,
				d.Add(time.Hour*2 + time.Second*9): 9,
			},
		},
		`abs(-1)`: {"": expr.Number(1)},
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req opentsdb.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			log.Fatal(err)
		}
		var resp opentsdb.ResponseSet
		for _, rq := range req.Queries {
			qs := fmt.Sprintf(`q("%s", "%v", "%v")`, rq, req.Start, req.End)
			q, ok := queries[qs]
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

	for exprText, expected := range tests {
		e, err := expr.New(exprText, ExprFuncs)
		if err != nil {
			t.Fatal(err)
		}
		tsdbs := &expr.TSDBs{
			OpenTSDB: &opentsdb.LimitContext{Host: u.Host, Limit: 1e10, TSDBVersion: opentsdb.Version2_1},
			Influx:   client.HTTPConfig{},
		}
		providers := &expr.BosunProviders{}
		results, _, err := e.Execute(tsdbs, providers, nil, queryTime, 0, false, t.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range results.Elements {
			tag := r.Group.Tags()
			ex := expected[tag]
			if ex == nil {
				t.Errorf("missing tag %v", tag)
				continue
			}
			switch val := r.Value.(type) {
			case expr.Series:
				ex, ok := ex.(expr.Series)
				if !ok {
					t.Errorf("%v: bad type %T", exprText, ex)
					continue
				}
				if len(val) != len(ex) {
					t.Errorf("unmatched values in %v", tag)
					continue
				}
				for k, v := range ex {
					got := val[k]
					if got != v {
						t.Errorf("%v, %v: got %v, expected %v", tag, k, got, v)
					}
				}
			case expr.Number:
				ex, ok := ex.(expr.Number)
				if !ok {
					t.Errorf("%v: bad type %T", exprText, ex)
					continue
				}
				if ex != val {
					t.Errorf("%v: got %v, expected %v", exprText, r.Value, ex)
				}
			default:
				t.Errorf("%v: unknown type %T", exprText, r.Value)
			}
		}
	}
}
