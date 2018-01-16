package opentsdb

import (
	"testing"
)

func TestClean(t *testing.T) {
	clean := "aoeSNVT152-./_"
	if c, err := Clean(clean); c != clean {
		t.Error("was clean", clean)
	} else if err != nil {
		t.Fatal(err)
	}
	notclean := "euon@#$sar:.03   n  ]e/"
	notcleaned := "euonsar.03ne/"
	if c, err := Clean(notclean); c != notcleaned {
		t.Error("wasn't cleaned", notclean, "into:", c)
	} else if err != nil {
		t.Fatal(err)
	}
}

func TestEmptyPoint(t *testing.T) {
	d := DataPoint{}
	if d.Clean() == nil {
		t.Fatal("empty datapoint should not be cleanable")
	}
}

func TestParseQueryV2_1(t *testing.T) {
	tests := []struct {
		query string
		error bool
	}{
		{"sum:10m-avg:proc.stat.cpu{t=v,o=k}", false},
		{"sum:10m-avg:rate:proc.stat.cpu", false},
		{"sum:10m-avg:rate{counter,1,2}:proc.stat.cpu{t=v,o=k}", false},
		{"sum:proc.stat.cpu", false},
		{"sum:rate:proc.stat.cpu{t=v,o=k}", false},

		{"", true},
		{"sum:cpu+", true},
		{"sum:cpu{}", true},
		{"sum:stat{a=b=c}", true},
	}
	for _, q := range tests {
		_, err := ParseQuery(q.query, Version2_1)
		if err != nil && !q.error {
			t.Errorf("got error: %s: %s", q.query, err)
		} else if err == nil && q.error {
			t.Errorf("expected error: %s", q.query)
		}
	}
}

func TestParseQueryV2_2(t *testing.T) {
	tests := []struct {
		query string
		error bool
	}{
		{"sum:10m-avg:proc.stat.cpu{t=v,o=k}", false},
		{"sum:10m-avg:rate:proc.stat.cpu", false},
		{"sum:10m-avg:rate{counter,1,2}:proc.stat.cpu{t=v,o=k}", false},
		{"sum:10m-avg:rate{counter,1,2}:proc.stat.cpu{t=v,o=k}{t=wildcard(v*)}", false},
		{"sum:10m-avg:rate{counter,1,2}:proc.stat.cpu{}{t=wildcard(v*)}", false},
		{"sum:proc.stat.cpu", false},
		{"sum:rate:proc.stat.cpu{t=v,o=k}", false},

		{"", true},
		{"sum:cpu+", true},
		// This is supported in 2.2
		{"sum:cpu{}", false},
		// This wouldn't be valid, but we are more permissive and will let opentsdb
		// return errors. This is because there can be a regex filter now. Might
		// be issues with escaping there however
		{"sum:stat{a=b=c}", false},

		//test fill policies
		{"sum:10m-avg-zero:proc.stat.cpu{t=v,o=k}", false},
		{"sum:10m-avg-:proc.stat.cpu{t=v,o=k}", true},
		{"sum:10m-avg-none:rate:proc.stat.cpu", false},
		{"sum:10m-avg-:rate{counter,1,2}:proc.stat.cpu{t=v,o=k}", true},
	}
	for _, q := range tests {
		_, err := ParseQuery(q.query, Version2_2)
		if err != nil && !q.error {
			t.Errorf("got error: %s: %s", q.query, err)
		} else if err == nil && q.error {
			t.Errorf("expected error: %s", q.query)
		}
	}
}

func TestParseRequestV2_1(t *testing.T) {
	tests := []struct {
		query string
		error bool
	}{
		{"start=1&m=sum:c", false},
		{"start=1&m=sum:c&end=2", false},
		{"start=1&m=sum:10m-avg:rate:proc.stat.cpu{t=v,o=k}", false},

		{"start=&m=", true},
		{"m=sum:c", true},
		{"start=1", true},
	}
	for _, q := range tests {
		_, err := ParseRequest(q.query, Version2_1)
		if err != nil && !q.error {
			t.Errorf("got error: %s: %s", q.query, err)
		} else if err == nil && q.error {
			t.Errorf("expected error: %s", q.query)
		}
	}
}

func TestParseRequestV2_2(t *testing.T) {
	tests := []struct {
		query string
		error bool
	}{
		{"start=1&m=sum:c", false},
		{"start=1&m=sum:c&end=2", false},
		{"start=1&m=sum:10m-avg:rate:proc.stat.cpu{t=v,o=k}", false},
		{"start=1&m=sum:10m-avg:rate:proc.stat.cpu{}{t=v,o=k}", false},
		{"start=1&m=sum:10m-avg:rate:proc.stat.cpu{z=iwildcard(foo*)}{t=v,o=k}", false},

		{"start=&m=", true},
		{"m=sum:c", true},
		{"start=1", true},
	}
	for _, q := range tests {
		_, err := ParseRequest(q.query, Version2_2)
		if err != nil && !q.error {
			t.Errorf("got error: %s: %s", q.query, err)
		} else if err == nil && q.error {
			t.Errorf("expected error: %s", q.query)
		}
	}
}
func TestTagGroupParsing(t *testing.T) {
	tests := []struct {
		query  string
		groups string
	}{
		{"sum:10m-avg:proc.stat.cpu{}{t=v,o=k}", "{}"},
		{"sum:10m-avg:proc.stat.cpu{dc=uk}{t=v,o=k}", "{dc=}"},
	}
	for _, q := range tests {
		r, err := ParseQuery(q.query, Version2_2)
		if err == nil {
			if r.GroupByTags.String() != q.groups {
				t.Errorf("expected group tags %s got %s", q.groups, r.GroupByTags)
			}

		}
	}
}

func TestParseFilters(t *testing.T) {
	tests := []struct {
		query   string
		filters Filters
	}{
		{"sum:10m-avg:rate{counter,1,2}:proc.stat.cpu{t=v,o=k}",
			Filters{
				Filter{
					Type:    "literal_or",
					TagK:    "t",
					Filter:  "v",
					GroupBy: true,
				},
				Filter{
					Type:    "literal_or",
					TagK:    "o",
					Filter:  "k",
					GroupBy: true,
				},
			},
		},
		{"sum:10m-avg:rate:proc.stat.cpu{}{t=v,o=k}",
			Filters{
				Filter{
					Type:    "literal_or",
					TagK:    "t",
					Filter:  "v",
					GroupBy: false,
				},
				Filter{
					Type:    "literal_or",
					TagK:    "o",
					Filter:  "k",
					GroupBy: false,
				},
			},
		},
		{"sum:10m-avg:rate:proc.stat.cpu{t=v}{o=k}",
			Filters{
				Filter{
					Type:    "literal_or",
					TagK:    "t",
					Filter:  "v",
					GroupBy: true,
				},
				Filter{
					Type:    "literal_or",
					TagK:    "o",
					Filter:  "k",
					GroupBy: false,
				},
			},
		},
		{"sum:10m-avg:rate:proc.stat.cpu{t=v}{o=iwildcard(foo*)}",
			Filters{
				Filter{
					Type:    "literal_or",
					TagK:    "t",
					Filter:  "v",
					GroupBy: true,
				},
				Filter{
					Type:    "iwildcard",
					TagK:    "o",
					Filter:  "foo*",
					GroupBy: false,
				},
			},
		},
	}
	for _, q := range tests {
		parsedQuery, err := ParseQuery(q.query, Version2_2)
		if err != nil {
			t.Errorf("error parsing query %s: %s", q.query, err)
			continue
		}
		if len(q.filters) != len(parsedQuery.Filters) {
			t.Errorf("expected %d filters, got: %d", len(q.filters), len(parsedQuery.Filters))
		} else {
			for i, f := range parsedQuery.Filters {
				if q.filters[i] != f {
					t.Errorf("expected parsed filter %+v, got: %+v", q.filters[i], f)
				}
			}
		}
	}
}

func TestQueryString(t *testing.T) {
	tests := []struct {
		in  Query
		out string
	}{
		{
			Query{
				Aggregator: "avg",
				Metric:     "test.metric",
				Rate:       true,
				RateOptions: RateOptions{
					Counter:    true,
					CounterMax: 1,
					ResetValue: 2,
				},
			},
			"avg:rate{counter,1,2}:test.metric",
		},
		{
			Query{
				Aggregator: "avg",
				Metric:     "test.metric",
				Rate:       true,
				RateOptions: RateOptions{
					Counter:    true,
					CounterMax: 1,
				},
			},
			"avg:rate{counter,1}:test.metric",
		},
		{
			Query{
				Aggregator: "avg",
				Metric:     "test.metric",
				Rate:       true,
				RateOptions: RateOptions{
					Counter: true,
				},
			},
			"avg:rate{counter}:test.metric",
		},
		{
			Query{
				Aggregator: "avg",
				Metric:     "test.metric",
				Rate:       true,
				RateOptions: RateOptions{
					CounterMax: 1,
					ResetValue: 2,
				},
			},
			"avg:rate:test.metric",
		},
		{
			Query{
				Aggregator: "avg",
				Metric:     "test.metric",
				RateOptions: RateOptions{
					Counter:    true,
					CounterMax: 1,
					ResetValue: 2,
				},
			},
			"avg:test.metric",
		},
	}
	for _, q := range tests {
		s := q.in.String()
		if s != q.out {
			t.Errorf(`got "%s", expected "%s"`, s, q.out)
		}
	}
}

func TestValidTSDBString(t *testing.T) {
	tests := map[string]bool{
		"abcXYZ012_./-": true,

		"":    false,
		"a|c": false,
		"a=b": false,
	}
	for s, v := range tests {
		r := ValidTSDBString(s)
		if v != r {
			t.Errorf("%v: got %v, expected %v", s, r, v)
		}
	}
}

func TestValidTags(t *testing.T) {
	tests := map[string]bool{
		"a=b|c,d=*": true,

		"":        false,
		"a=b,a=c": false,
		"a=b=c":   false,
	}
	for s, v := range tests {
		_, err := ParseTags(s)
		r := err == nil
		if v != r {
			t.Errorf("%v: got %v, expected %v", s, r, v)
		}
	}
}

func TestAllSubsets(t *testing.T) {
	ts, _ := ParseTags("a=1,b=2,c=3,d=4")
	subsets := ts.AllSubsets()
	if len(subsets) != 15 {
		t.Fatal("Expect 15 subsets")
	}
}

func TestReplace(t *testing.T) {
	tests := []struct{ in, out string }{
		{"abc", "abc"},
		{"ny-web01", "ny-web01"},
		{"_./", "_./"},
		{"%%%a", ".a"},
	}
	for i, test := range tests {
		out, err := Replace(test.in, ".")
		if err != nil {
			t.Errorf("Test %d: %s", i, err)
		}
		if out != test.out {
			t.Errorf("Test %d: %s != %s", i, out, test.out)
		}
	}
}

func BenchmarkReplace_Noop(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Replace("abcdefghijklmnopqrstuvwxyz", "")
	}
}

func BenchmarkReplace_Something(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Replace("abcdef&hij@@$$opq#stuvw*yz", "")
	}
}
