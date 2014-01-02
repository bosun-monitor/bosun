package expr

import "testing"

type exprTest struct {
	input  string
	output float64
}

var exprTests = []exprTest{
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
}

func TestExpr(t *testing.T) {
	for _, et := range exprTests {
		e, err := New(et.input)
		if err != nil {
			t.Error(err)
			break
		}
		r, err := e.Execute("")
		if err != nil {
			t.Error(err)
		} else if r != et.output {
			t.Errorf("expected %v, got %v: %v", et.output, r, et.input)
		}
	}
}
