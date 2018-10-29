package expr

import (
	"testing"
	"time"

	"bosun.org/opentsdb"
)

func TestMap(t *testing.T) {
	err := testExpression(exprInOut{
		`map(series("test=test", 0, 1, 1, 3), expr(v()+1))`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 2,
						time.Unix(1, 0): 4,
					},
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		false,
	}, t)
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`avg(map(series("test=test", 0, 1, 1, 3), expr(v()+1)))`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Number(3),
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		false,
	}, t)
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`1 + avg(map(series("test=test", 0, 1, 1, 3), expr(v()+1))) + 1`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Number(5),
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		false,
	}, t)
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`max(map(series("test=test", 0, 1, 1, 3), expr(v()+v())))`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Number(6),
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		false,
	}, t)
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`map(series("test=test", 0, -2, 1, 3), expr(1+1))`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 2,
						time.Unix(1, 0): 2,
					},
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		false,
	}, t)
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`map(series("test=test", 0, -2, 1, 3), expr(abs(v())))`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 2,
						time.Unix(1, 0): 3,
					},
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		false,
	}, t)
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`map(series("test=test", 0, -2, 1, 3), expr(series("test=test", 0, v())))`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{},
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		true, // expect parse error here, series result not valid as TypeNumberExpr
	}, t)
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`v()`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{},
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		true, // v() is not valid outside a map expression
	}, t)
	if err != nil {
		t.Error(err)
	}

	err = testExpression(exprInOut{
		`map(series("test=test", 0, -2, 1, 0), expr(!v()))`,
		Results{
			Results: ResultSlice{
				&Result{
					Value: Series{
						time.Unix(0, 0): 0,
						time.Unix(1, 0): 1,
					},
					Group: opentsdb.TagSet{"test": "test"},
				},
			},
		},
		false,
	}, t)
	if err != nil {
		t.Error(err)
	}
}
