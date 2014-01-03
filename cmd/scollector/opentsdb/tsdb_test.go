package opentsdb

import "testing"

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

func TestParseQuery(t *testing.T) {
	tests := []struct {
		query string
		error bool
	}{
		{"sum:10m-avg:proc.stat.cpu{t=v,o=k}", false},
		{"sum:10m-avg:rate:proc.stat.cpu", false},
		{"sum:10m-avg:rate:proc.stat.cpu{t=v,o=k}", false},
		{"sum:proc.stat.cpu", false},
		{"sum:rate:proc.stat.cpu{t=v,o=k}", false},

		{"", true},
		{"sum:cpu+", true},
		{"sum:cpu{}", true},
	}
	for _, q := range tests {
		_, err := ParseQuery(q.query)
		if err != nil && !q.error {
			t.Errorf("got error: %s: %s", q.query, err)
		} else if err == nil && q.error {
			t.Errorf("expected error: %s", q.query)
		}
	}
}

func TestParseRequest(t *testing.T) {
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
		_, err := ParseRequest(q.query)
		if err != nil && !q.error {
			t.Errorf("got error: %s: %s", q.query, err)
		} else if err == nil && q.error {
			t.Errorf("expected error: %s", q.query)
		}
	}
}
