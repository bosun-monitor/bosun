package expr

import (
	"fmt"
	"math"
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
		tsdbs := &TSDBs{
			Influx: client.HTTPConfig{},
		}
		providers := &BosunProviders{}
		r, _, err := e.Execute(tsdbs, providers, nil, time.Now(), 0, false, t.Name())
		if err != nil {
			t.Error(err)
			break
		} else if len(r.Elements) != 1 {
			t.Error("bad r len", len(r.Elements))
			break
		} else if len(r.Elements[0].Group) != 0 {
			t.Error("bad group len", r.Elements[0].Group)
			break
		} else if math.IsNaN(float64(et.output)) && math.IsNaN(float64(r.Elements[0].Value.(Scalar))) {
			// ok
		} else if r.Elements[0].Value != et.output {
			t.Errorf("expected %v, got %v: %v\nast: %v", et.output, r.Elements[0].Value, et.input, e)
		}
	}
}

func TestExprParse(t *testing.T) {
	var exprTests = []struct {
		input string
		valid bool
		tags  string
	}{
		{`avg(series("test", "1m", 1))`, false, ""},
		{`avg(series("", 0, 1))`, true, ""},
		{`avg(series("a=foo", 0, 22))`, true, "a"},
		{`avg(series("a=foo,b=_whynot", 0, 1))`, true, "a,b"},
		{`avg(series("a=foo,b=_whynot", 0, 1)) + 1`, true, "a,b"},
	}

	for _, et := range exprTests {
		e, err := New(et.input, builtins)
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

func TestSetVariant(t *testing.T) {
	series := `series("key1=a,key2=b", 0, 1, 1, 3)`
	seriesAbs := `series("", 0, 1, 1, -3)`
	tests := []exprInOut{
		{
			fmt.Sprintf(`addtags(addtags(%v, "key3=a"), "key4=b") + 1`, series),
			ValueSet{
				Elements: ElementSlice{
					&Element{
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
			ValueSet{
				Elements: ElementSlice{
					&Element{
						Value: Number(4),
						Group: opentsdb.TagSet{"key1": "a", "key2": "b", "key3": "a", "key4": "b"},
					},
				},
			},
			false,
		},
		{
			fmt.Sprintf(`avg(addtags(addtags(avg(%v + 1), "key3=a"), "key4=b")) + 1`, series),
			ValueSet{},
			true,
		},

		{
			fmt.Sprintf(`1 + abs(%v)`, seriesAbs),
			ValueSet{
				Elements: ElementSlice{
					&Element{
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
			ValueSet{
				Elements: ElementSlice{
					&Element{
						Value: Number(2),
						Group: opentsdb.TagSet{},
					},
				},
			},
			false,
		},
	}
	for _, test := range tests {
		err := testExpression(test, t)
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
			ValueSet{
				Elements: ElementSlice{
					&Element{
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
			ValueSet{
				Elements: ElementSlice{
					&Element{
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
			ValueSet{
				Elements: ElementSlice{
					&Element{
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
		err := testExpression(test, t)
		if err != nil {
			t.Error(err)
		}
	}
}
