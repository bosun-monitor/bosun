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

func testExpression(eio exprInOut) error {
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
	r, _, err := e.Execute(backends, providers, nil, queryTime, 0, false)
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
	err := testExpression(d)
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
		err := testExpression(d)
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
	})

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
	})
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
	})
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
		})

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
		})

		if err != nil {
			t.Error(err)
		}
	}
}

func TestAggr(t *testing.T) {
	seriesA := `series("foo=bar", 0, 1, 100, 2)`
	seriesB := `series("foo=baz", 0, 3, 100, 4)`
	seriesC := `series("foo=bat", 0, 5, 100, 6)`

	// test median aggregator
	err := testExpression(exprInOut{
		fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"p.50\")", seriesA, seriesB, seriesC),
		Results{
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
		false,
	})
	if err != nil {
		t.Error(err)
	}

	// test average aggregator
	err = testExpression(exprInOut{
		fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"avg\")", seriesA, seriesB, seriesC),
		Results{
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
		false,
	})
	if err != nil {
		t.Error(err)
	}

	// test min aggregator
	err = testExpression(exprInOut{
		fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"min\")", seriesA, seriesB, seriesC),
		Results{
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
		false,
	})
	if err != nil {
		t.Error(err)
	}

	// test max aggregator
	err = testExpression(exprInOut{
		fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"max\")", seriesA, seriesB, seriesC),
		Results{
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
		false,
	})
	if err != nil {
		t.Error(err)
	}

	// check that min == p0
	err = testExpression(exprInOut{
		fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"p0\")", seriesA, seriesB, seriesC),
		Results{
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
		false,
	})
	if err != nil {
		t.Error(err)
	}

	// check that sum aggregator sums up the aligned points in the series
	err = testExpression(exprInOut{
		fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"sum\")", seriesA, seriesB, seriesC),
		Results{
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
		false,
	})
	if err != nil {
		t.Error(err)
	}

	// check that unknown aggregator errors out
	err = testExpression(exprInOut{
		fmt.Sprintf("aggr(merge(%v, %v, %v), \"\", \"unknown\")", seriesA, seriesB, seriesC),
		Results{},
		false,
	})
	if err == nil {
		t.Errorf("expected unknown aggregator to return error")
	}
}

func TestAggrWithGroups(t *testing.T) {
	seriesA := `series("color=blue,type=apple,name=bob", 0, 1)`
	seriesB := `series("color=blue,type=apple", 1, 3)`
	seriesC := `series("color=green,type=apple", 0, 5)`

	// test aggregator with single group
	err := testExpression(exprInOut{
		fmt.Sprintf("aggr(merge(%v, %v, %v), \"color\", \"p.50\")", seriesA, seriesB, seriesC),
		Results{
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
		false,
	})
	if err != nil {
		t.Error(err)
	}

	// test aggregator with multiple groups
	err = testExpression(exprInOut{
		fmt.Sprintf("aggr(merge(%v, %v, %v), \"color,type\", \"p.50\")", seriesA, seriesB, seriesC),
		Results{
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
		false,
	})
	if err != nil {
		t.Error(err)
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
	_, _, err = e.Execute(backends, providers, nil, queryTime, 0, false)
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
