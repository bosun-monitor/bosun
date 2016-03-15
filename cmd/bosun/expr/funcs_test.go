package expr

import (
	"testing"
	"time"

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
	r, _, err := e.Execute(nil, nil, nil, nil, client.Config{}, nil, nil, time.Now(), 0, false, nil, nil, nil)
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
	d := exprInOut{
		`tod(3600*2)`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: String("2h"),
				},
			},
		},
	}
	err := testExpression(d)
	if err != nil {
		t.Error(err)
	}
}
