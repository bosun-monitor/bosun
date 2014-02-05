package expr

import "testing"

func TestExprSimple(t *testing.T) {
	var exprTests = []struct {
		input  string
		output Number
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
	}

	for _, et := range exprTests {
		e, err := New(et.input)
		if err != nil {
			t.Error(err)
			break
		}
		r, err := e.Execute("", nil)
		if err != nil {
			t.Error(err)
			break
		} else if len(r) != 1 {
			t.Error("bad r len", len(r))
			break
		} else if len(r[0].Group) != 0 {
			t.Error("bad group len", r[0].Group)
			break
		} else if r[0].Value != et.output {
			t.Errorf("expected %v, got %v: %v\nast: %v", et.output, r[0].Value, et.input, e)
		}
	}
}

const TSDB_HOST = "ny-devtsdb04:4242"

func TestExprQuery(t *testing.T) {
	e, err := New(`avg(q("avg:proc.stat.cpu{host=*,type=idle}")) > avg(q("avg:proc.stat.cpu{host=*}", "5m"))`)
	if err != nil {
		t.Fatal(err)
	}
	_, err := e.Execute(TSDB_HOST, nil)
	if err != nil {
		t.Fatal(err)
	}
}
