package expr

import (
	"fmt"
	"math"
	"testing"
	"time"

	"bosun.org/opentsdb"

	"github.com/influxdata/influxdb/client/v2"
)

type exprInOut struct {
	expr           string
	out            Results
	shouldParseErr bool
}

func testExpression(eio exprInOut, t *testing.T) error {
	e, err := New(eio.expr, builtins)
	if eio.shouldParseErr {
		if err == nil {
			return fmt.Errorf("no error when expected error on %v", eio.expr)
		}
		return nil
	}
	if err != nil {
		return err
	}
	backends := &Backends{
		InfluxConfig: client.HTTPConfig{},
	}
	providers := &BosunProviders{}
	r, _, err := e.Execute(backends, providers, nil, queryTime, 0, false, t.Name())
	if err != nil {
		return err
	}
	if _, err := eio.out.Equal(r); err != nil {
		return err
	}
	return nil
}

func TestDuration(t *testing.T) {
	d := exprInOut{
		`d("1h")`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Scalar(3600),
				},
			},
		},
		false,
	}
	err := testExpression(d, t)
	if err != nil {
		t.Error(err)
	}
}

func TestToDuration(t *testing.T) {
	inputs := []int{
		0,
		1,
		60,
		3600,
		86400,
		604800,
		31536000,
	}
	outputs := []string{
		"0ms",
		"1s",
		"1m",
		"1h",
		"1d",
		"1w",
		"1y",
	}

	for i := range inputs {
		d := exprInOut{
			fmt.Sprintf(`tod(%d)`, inputs[i]),
			Results{
				Results: ResultSlice{
					&Result{
						Value: String(outputs[i]),
					},
				},
			},
			false,
		}
		err := testExpression(d, t)
		if err != nil {
			t.Error(err)
		}
	}
}

func TestUngroup(t *testing.T) {
	dictum := `series("foo=bar", 0, ungroup(last(series("foo=baz", 0, 1))))`
	err := testExpression(exprInOut{
		dictum,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 1,
					},
					Group: opentsdb.TagSet{"foo": "bar"},
				},
			},
		},
		false,
	}, t)

	if err != nil {
		t.Error(err)
	}
}

func TestMerge(t *testing.T) {
	seriesA := `series("foo=bar", 0, 1)`
	seriesB := `series("foo=baz", 0, 1)`
	err := testExpression(exprInOut{
		fmt.Sprintf("merge(%v, %v)", seriesA, seriesB),
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 1,
					},
					Group: opentsdb.TagSet{"foo": "bar"},
				},
				&Result{
					Value: Series{
						time.Unix(0, 0): 1,
					},
					Group: opentsdb.TagSet{"foo": "baz"},
				},
			},
		},
		false,
	}, t)
	if err != nil {
		t.Error(err)
	}

	//Should Error due to identical groups in merge
	err = testExpression(exprInOut{
		fmt.Sprintf("merge(%v, %v)", seriesA, seriesA),
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 1,
					},
					Group: opentsdb.TagSet{"foo": "bar"},
				},
				&Result{
					Value: Series{
						time.Unix(0, 0): 1,
					},
					Group: opentsdb.TagSet{"foo": "bar"},
				},
			},
		},
		false,
	}, t)
	if err == nil {
		t.Errorf("error expected due to identical groups in merge but did not get one")
	}
}

func TestTimedelta(t *testing.T) {
	for _, i := range []struct {
		input    string
		expected Series
	}{
		{
			`timedelta(series("foo=bar", 1466133600, 1, 1466133610, 1, 1466133710, 1))`,
			Series{
				time.Unix(1466133610, 0): 10,
				time.Unix(1466133710, 0): 100,
			},
		},
		{
			`timedelta(series("foo=bar", 1466133600, 1))`,
			Series{
				time.Unix(1466133600, 0): 0,
			},
		},
	} {

		err := testExpression(exprInOut{
			i.input,
			Results{
				Results: ResultSlice{
					&Result{
						Value: i.expected,
						Group: opentsdb.TagSet{"foo": "bar"},
					},
				},
			},
			false,
		}, t)

		if err != nil {
			t.Error(err)
		}
	}
}

func TestTail(t *testing.T) {
	for _, i := range []struct {
		input    string
		expected Series
	}{
		{
			`tail(series("foo=bar", 1466133600, 1, 1466133610, 1, 1466133710, 1), 2)`,
			Series{
				time.Unix(1466133610, 0): 1,
				time.Unix(1466133710, 0): 1,
			},
		},
		{
			`tail(series("foo=bar", 1466133600, 1), 2)`,
			Series{
				time.Unix(1466133600, 0): 1,
			},
		},
		{
			`tail(series("foo=bar", 1466133600, 1, 1466133610, 1, 1466133710, 1), last(series("foo=bar", 1466133600, 2)))`,
			Series{
				time.Unix(1466133610, 0): 1,
				time.Unix(1466133710, 0): 1,
			},
		},
	} {

		err := testExpression(exprInOut{
			i.input,
			Results{
				Results: ResultSlice{
					&Result{
						Value: i.expected,
						Group: opentsdb.TagSet{"foo": "bar"},
					},
				},
			},
			false,
		}, t)

		if err != nil {
			t.Error(err)
		}
	}
}

func TestAggr(t *testing.T) {
	seriesA := `series("foo=bar", 0, 1, 100, 2)`
	seriesB := `series("foo=baz", 0, 3, 100, 4)`
	seriesC := `series("foo=bat", 0, 5, 100, 6)`

	seriesGroupsA := `series("color=blue,type=apple,name=bob", 0, 1)`
	seriesGroupsB := `series("color=blue,type=apple", 1, 3)`
	seriesGroupsC := `series("color=green,type=apple", 0, 5)`

	seriesMathA := `series("color=blue,type=apple,name=bob", 0, 1)`
	seriesMathB := `series("color=blue,type=apple", 1, 3)`
	seriesMathC := `series("color=green,type=apple", 0, 5)`

	aggrTestCases := []struct {
		name      string
		expr      string
		want      Results
		shouldErr bool
	}{
		{
			name: "median aggregator",
			expr: fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"p.50\")", seriesA, seriesB, seriesC),
			want: Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0):   3,
							time.Unix(100, 0): 4,
						},
						Group: opentsdb.TagSet{},
					},
				},
			},
			shouldErr: false,
		},
		{
			name: "average aggregator",
			expr: fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"avg\")", seriesA, seriesB, seriesC),
			want: Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0):   3,
							time.Unix(100, 0): 4,
						},
						Group: opentsdb.TagSet{},
					},
				},
			},
			shouldErr: false,
		},
		{
			name: "min aggregator",
			expr: fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"min\")", seriesA, seriesB, seriesC),
			want: Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0):   1,
							time.Unix(100, 0): 2,
						},
						Group: opentsdb.TagSet{},
					},
				},
			},
			shouldErr: false,
		},
		{
			name: "max aggregator",
			expr: fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"max\")", seriesA, seriesB, seriesC),
			want: Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0):   5,
							time.Unix(100, 0): 6,
						},
						Group: opentsdb.TagSet{},
					},
				},
			},
			shouldErr: false,
		},
		{
			name: "check p0 == min",
			expr: fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"p0\")", seriesA, seriesB, seriesC),
			want: Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0):   1,
							time.Unix(100, 0): 2,
						},
						Group: opentsdb.TagSet{},
					},
				},
			},
			shouldErr: false,
		},
		{
			name: "check that sum aggregator sums up the aligned points in the series",
			expr: fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"sum\")", seriesA, seriesB, seriesC),
			want: Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0):   9,
							time.Unix(100, 0): 12,
						},
						Group: opentsdb.TagSet{},
					},
				},
			},
			shouldErr: false,
		},
		{
			name:      "check that unknown aggregator errors out",
			expr:      fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"unknown\")", seriesA, seriesB, seriesC),
			want:      Results{},
			shouldErr: true,
		},
		{
			name: "single group",
			expr: fmt.Sprintf("aggr(merge(%v, %v, %v), \"color\", \"p.50\")", seriesGroupsA, seriesGroupsB, seriesGroupsC),
			want: Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0): 1,
							time.Unix(1, 0): 3,
						},
						Group: opentsdb.TagSet{"color": "blue"},
					},
					&Result{
						Value: Series{
							time.Unix(0, 0): 5,
						},
						Group: opentsdb.TagSet{"color": "green"},
					},
				},
			},
			shouldErr: false,
		},
		{
			name: "multiple groups",
			expr: fmt.Sprintf("aggr(merge(%v, %v, %v), \"color,type\", \"p.50\")", seriesGroupsA, seriesGroupsB, seriesGroupsC),
			want: Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0): 1,
							time.Unix(1, 0): 3,
						},
						Group: opentsdb.TagSet{"color": "blue", "type": "apple"},
					},
					&Result{
						Value: Series{
							time.Unix(0, 0): 5,
						},
						Group: opentsdb.TagSet{"color": "green", "type": "apple"},
					},
				},
			},
			shouldErr: false,
		},
		{
			name: "aggregator with no groups and math operation",
			expr: fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"p.50\") * 2", seriesMathA, seriesMathB, seriesMathC),
			want: Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0): 10,
							time.Unix(1, 0): 6,
						},
						Group: opentsdb.TagSet{},
					},
				},
			},
			shouldErr: false,
		},
		{
			name: "aggregator with one group and math operation",
			expr: fmt.Sprintf("aggr(merge(%v, %v, %v), \"color\", \"p.50\") * 2", seriesMathA, seriesMathB, seriesMathC),
			want: Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0): 2,
							time.Unix(1, 0): 6,
						},
						Group: opentsdb.TagSet{"color": "blue"},
					},
					&Result{
						Value: Series{
							time.Unix(0, 0): 10,
						},
						Group: opentsdb.TagSet{"color": "green"},
					},
				},
			},
			shouldErr: false,
		},
		{
			name: "aggregator with multiple groups and math operation",
			expr: fmt.Sprintf("aggr(merge(%v, %v, %v), \"color,type\", \"p.50\") * 2", seriesMathA, seriesMathB, seriesMathC),
			want: Results{
				Results: ResultSlice{
					&Result{
						Value: Series{
							time.Unix(0, 0): 2,
							time.Unix(1, 0): 6,
						},
						Group: opentsdb.TagSet{"color": "blue", "type": "apple"},
					},
					&Result{
						Value: Series{
							time.Unix(0, 0): 10,
						},
						Group: opentsdb.TagSet{"color": "green", "type": "apple"},
					},
				},
			},
			shouldErr: false,
		},
	}

	for _, tc := range aggrTestCases {
		err := testExpression(exprInOut{
			expr:           tc.expr,
			out:            tc.want,
			shouldParseErr: false,
		}, t)
		if !tc.shouldErr && err != nil {
			t.Errorf("Case %q: Got error: %v", tc.name, err)
		} else if tc.shouldErr && err == nil {
			t.Errorf("Case %q: Expected parse error, but got nil", tc.name)
		}
	}
}

func TestAggrNaNHandling(t *testing.T) {
	// test behavior when NaN is encountered.
	seriesD := `series("foo=bar", 0, 0 / 0, 100, 1)`
	seriesE := `series("foo=baz", 0, 1, 100, 3)`

	// expect NaN points to be dropped
	eio := exprInOut{
		fmt.Sprintf("aggr(merge(%v, %v), \"\", \"p.90\")", seriesD, seriesE),
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0):   math.NaN(),
						time.Unix(100, 0): 2,
					},
					Group: opentsdb.TagSet{},
				},
			},
		},
		false,
	}
	e, err := New(eio.expr, builtins)
	if err != nil {
		t.Fatal(err.Error())
	}
	backends := &Backends{
		InfluxConfig: client.HTTPConfig{},
	}
	providers := &BosunProviders{}
	_, _, err = e.Execute(backends, providers, nil, queryTime, 0, false, t.Name())
	if err != nil {
		t.Fatal(err.Error())
	}

	results := eio.out.Results
	if len(results) != 1 {
		t.Errorf("got len(results) == %d, want 1", len(results))
	}
	val0 := results[0].Value.(Series)[time.Unix(0, 0)]
	if !math.IsNaN(val0) {
		t.Errorf("got first point = %f, want NaN", val0)
	}
	val1 := results[0].Value.(Series)[time.Unix(100, 0)]
	if val1 != 2.0 {
		t.Errorf("got second point = %f, want %f", val1, 2.0)
	}
}
