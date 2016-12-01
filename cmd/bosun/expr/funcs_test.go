package expr

import (
	"fmt"
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
