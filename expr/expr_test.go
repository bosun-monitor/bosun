package expr

import (
	"testing"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
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
	}

	for _, et := range exprTests {
		e, err := New(et.input)
		if err != nil {
			t.Error(err)
			break
		}
		r, _, err := e.Execute(opentsdb.Host(""), nil, time.Now(), 0, false, nil, nil)
		if err != nil {
			t.Error(err)
			break
		} else if len(r.Results) != 1 {
			t.Error("bad r len", len(r.Results))
			break
		} else if len(r.Results[0].Group) != 0 {
			t.Error("bad group len", r.Results[0].Group)
			break
		} else if r.Results[0].Value != et.output {
			t.Errorf("expected %v, got %v: %v\nast: %v", et.output, r.Results[0].Value, et.input, e)
		}
	}
}

func TestExprParse(t *testing.T) {
	var exprTests = []struct {
		input string
		valid bool
	}{
		{`avg(q("test", "1m", 1))`, false},
	}

	for _, et := range exprTests {
		_, err := New(et.input)
		if et.valid && err != nil {
			t.Error(err)
		} else if !et.valid && err == nil {
			t.Errorf("expected invalid, but no error: %v", et.input)
		}
	}
}

/*
const TSDBHost = "ny-devtsdb04:4242"

func TestExprQuery(t *testing.T) {
	e, err := New(`forecastlr(q("avg:os.cpu{host=ny-lb05}", "1m", ""), -10)`)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = e.Execute(opentsdb.Host(TSDBHost), nil)
	if err != nil {
		t.Fatal(err)
	}
}
*/
