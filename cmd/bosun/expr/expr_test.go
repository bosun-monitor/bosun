package expr

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"bosun.org/opentsdb"
	"github.com/influxdata/influxdb/client/v2"
)

func TestExprSimple(t *testing.T) {
	var exprTests = []struct {
		input  string
		output Scalar
	}{
		{"!1", 0},
		{"-2", -2},
		{"1.444-010+2*3e2-4/5+0xff", 847.644},
		{"1>2", 0},
		{"3>2", 1},
		{"1==1", 1},
		{"1==2", 0},
		{"1!=01", 0},
		{"1!=2", 1},
		{"1<2", 1},
		{"2<1", 0},
		{"1||0", 1},
		{"0||0", 0},
		{"1&&0", 0},
		{"1&&2", 1},
		{"1<=0", 0},
		{"1<=1", 1},
		{"1<=2", 1},
		{"1>=0", 1},
		{"1>=1", 1},
		{"1>=2", 0},
		{"-1 > 0", 0},
		{"-1 < 0", 1},
		{"30 % 3", 0},
		{"5 % 7", 5},
		{"25.5 % 5", .5},

		// NaN
		{"0 / 0", Scalar(math.NaN())},
		{"1 / 0", Scalar(math.Inf(1))},

		// short circuit
		{"0 && 0 / 0", 0},
		{"1 || 0 / 0", 1},
		{"1 && 0 / 0", Scalar(math.NaN())},
		{"0 || 0 / 0", Scalar(math.NaN())},
	}

	for _, et := range exprTests {
		e, err := New(et.input)
		if err != nil {
			t.Error(err)
			break
		}
		backends := &Backends{
			InfluxConfig: client.HTTPConfig{},
		}
		providers := &BosunProviders{}
		r, _, err := e.Execute(backends, providers, nil, time.Now(), 0, false)
		if err != nil {
			t.Error(err)
			break
		} else if len(r.Results) != 1 {
			t.Error("bad r len", len(r.Results))
			break
		} else if len(r.Results[0].Group) != 0 {
			t.Error("bad group len", r.Results[0].Group)
			break
		} else if math.IsNaN(float64(et.output)) && math.IsNaN(float64(r.Results[0].Value.(Scalar))) {
			// ok
		} else if r.Results[0].Value != et.output {
			t.Errorf("expected %v, got %v: %v\nast: %v", et.output, r.Results[0].Value, et.input, e)
		}
	}
}

func TestExprParse(t *testing.T) {
	var exprTests = []struct {
		input string
		valid bool
		tags  string
	}{
		{`avg(q("test", "1m", 1))`, false, ""},
		{`avg(q("avg:m", "1m", ""))`, true, ""},
		{`avg(q("avg:m{a=*}", "1m", ""))`, true, "a"},
		{`avg(q("avg:m{a=*,b=1}", "1m", ""))`, true, "a,b"},
		{`avg(q("avg:m{a=*,b=1}", "1m", "")) + 1`, true, "a,b"},
	}

	for _, et := range exprTests {
		e, err := New(et.input, TSDB)
		if et.valid && err != nil {
			t.Error(err)
		} else if !et.valid && err == nil {
			t.Errorf("expected invalid, but no error: %v", et.input)
		} else if et.valid {
			tags, err := e.Root.Tags()
			if err != nil {
				t.Error(err)
				continue
			}
			if et.tags != tags.String() {
				t.Errorf("%v: unexpected tags: got %v, expected %v", et.input, tags, et.tags)
			}
		}
	}
}

var queryTime = time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC)

func TestQueryExpr(t *testing.T) {
	queries := map[string]opentsdb.ResponseSet{
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
	tests := map[string]map[string]Value{
		`window("avg:m{a=*}", "5m", "1h", 2, "max")`: {
			"a=b": Series{
				d: 2,
				d.Add(time.Second * 2): 6,
			},
			"a=c": Series{
				d.Add(time.Second * 1): 8,
			},
			"a=d": Series{
				d.Add(time.Second * 8): 9,
			},
		},
		`window("avg:m{a=*}", "5m", "1h", 2, "avg")`: {
			"a=b": Series{
				d: 1.5,
				d.Add(time.Second * 2): 5,
			},
			"a=c": Series{
				d.Add(time.Second * 1): 7.5,
			},
			"a=d": Series{
				d.Add(time.Second * 8): 8.5,
			},
		},
		`abs(-1)`: {"": Number(1)},
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
		e, err := New(exprText, TSDB)
		if err != nil {
			t.Fatal(err)
		}
		backends := &Backends{
			TSDBContext:  &opentsdb.LimitContext{Host: u.Host, Limit: 1e10, TSDBVersion: opentsdb.Version2_1},
			InfluxConfig: client.HTTPConfig{},
		}
		providers := &BosunProviders{}
		results, _, err := e.Execute(backends, providers, nil, queryTime, 0, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range results.Results {
			tag := r.Group.Tags()
			ex := expected[tag]
			if ex == nil {
				t.Errorf("missing tag %v", tag)
				continue
			}
			switch val := r.Value.(type) {
			case Series:
				ex, ok := ex.(Series)
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
			case Number:
				ex, ok := ex.(Number)
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

func TestSetVariant(t *testing.T) {
	series := `series("key1=a,key2=b", 0, 1, 1, 3)`
	seriesAbs := `series("", 0, 1, 1, -3)`
	tests := []exprInOut{
		{
			fmt.Sprintf(`addtags(addtags(%v, "key3=a"), "key4=b") + 1`, series),
			Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0): 2,
							time.Unix(1, 0): 4,
						},
						Group: opentsdb.TagSet{"key1": "a", "key2": "b", "key3": "a", "key4": "b"},
					},
				},
			},
			false,
		},
		{
			fmt.Sprintf(`addtags(addtags(avg(%v + 1), "key3=a"), "key4=b") + 1`, series),
			Results{
				Results: ResultSlice{
					&Result{
						Value: Number(4),
						Group: opentsdb.TagSet{"key1": "a", "key2": "b", "key3": "a", "key4": "b"},
					},
				},
			},
			false,
		},
		{
			fmt.Sprintf(`avg(addtags(addtags(avg(%v + 1), "key3=a"), "key4=b")) + 1`, series),
			Results{},
			true,
		},

		{
			fmt.Sprintf(`1 + abs(%v)`, seriesAbs),
			Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0): 2,
							time.Unix(1, 0): 4,
						},
						Group: opentsdb.TagSet{},
					},
				},
			},
			false,
		},
		{
			fmt.Sprintf(`1 + abs(avg(%v))`, seriesAbs),
			Results{
				Results: ResultSlice{
					&Result{
						Value: Number(2),
						Group: opentsdb.TagSet{},
					},
				},
			},
			false,
		},
	}
	for _, test := range tests {
		err := testExpression(test)
		if err != nil {
			t.Error(err)
		}
	}
}

func TestSeriesOperations(t *testing.T) {
	seriesA := `series("key=a", 0, 1, 1, 2, 2, 1, 3, 4)`
	seriesB := `series("key=a", 0, 1,       2, 0, 3, 4)`
	seriesC := `series("key=a", 4, 1,       6, 0, 7, 4)`
	template := "%v %v %v"
	tests := []exprInOut{
		{
			fmt.Sprintf(template, seriesA, "+", seriesB),
			Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0): 2,
							time.Unix(2, 0): 1,
							time.Unix(3, 0): 8,
						},
						Group: opentsdb.TagSet{"key": "a"},
					},
				},
			},
			false,
		},
		{
			fmt.Sprintf(template, seriesA, "+", seriesC),
			Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							// Should be empty
						},
						Group: opentsdb.TagSet{"key": "a"},
					},
				},
			},
			false,
		},
		{
			fmt.Sprintf(template, seriesA, "/", seriesB),
			Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0): 1,
							time.Unix(2, 0): math.Inf(1),
							time.Unix(3, 0): 1,
						},
						Group: opentsdb.TagSet{"key": "a"},
					},
				},
			},
			false,
		},
	}
	for _, test := range tests {
		err := testExpression(test)
		if err != nil {
			t.Error(err)
		}
	}
}
