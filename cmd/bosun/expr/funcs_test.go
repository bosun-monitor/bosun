package expr

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"bosun.org/opentsdb"

	"github.com/influxdata/influxdb/client"
)

type exprInOut struct {
	expr string
	out  Results
}

func testExpression(eio exprInOut) error {
	e, err := New(eio.expr, builtins)
	if err != nil {
		return err
	}
	backends := &Backends{
		InfluxConfig: client.Config{},
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
		})

		if err != nil {
			t.Error(err)
		}
	}
}

func TestQuickSelect(t *testing.T) {
	for testCase, i := range []struct {
		description string
		input       sort.IntSlice
		number      int
		max         bool
		expected    sort.IntSlice
	}{
		{
			"sanity check that returns 3 min results",
			[]int{1, 2, 4, 5, 6, 7, 8},
			3,
			false,
			[]int{1, 2, 4},
		},
		{
			"case with multiple of the same number in return",
			[]int{1, 2, 4, 1, 5, 1, 6, 7, 8, 1},
			3,
			false,
			[]int{1, 1, 1},
		},
		{
			"sanity check that returns 3 max results",
			[]int{99, 2, 4, 1, 5, 1, 6, 7, 8, 1, 0, 100},
			3,
			true,
			[]int{99, 100, 8},
		},
		{
			"case where requested results are longer than array",
			[]int{1},
			3,
			true,
			[]int{1},
		},
	} {

		quickSelect(i.input, i.number, i.max)

		// quickSelect doesn't provide order
		// lets just sort for ease of test
		// also quickSelect doesn't care if
		// you ask for more results than it has
		found := i.input
		if i.number < len(i.input) {
			found = i.input[:i.number]
		}

		expected := i.expected
		sort.Sort(found)
		sort.Sort(expected)

		if !reflect.DeepEqual(
			found,
			expected,
		) {
			t.Errorf(
				"Test case %d: found %v expected %v\n",
				testCase,
				i.input[:i.number],
				i.expected,
			)
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
		})

		if err != nil {
			t.Error(err)
		}
	}
}
