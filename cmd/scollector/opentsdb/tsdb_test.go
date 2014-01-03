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

func TestParse(t *testing.T) {
	tests := []struct {
		query string
		error bool
	}{
		{"m=sum:10m-avg:proc.stat.cpu{t=v,o=k}", false},
		{"m=sum:10m-avg:rate:proc.stat.cpu", false},
		{"m=sum:10m-avg:rate:proc.stat.cpu{t=v,o=k}", false},
		{"m=sum:proc.stat.cpu", false},
		{"m=sum:rate:proc.stat.cpu{t=v,o=k}", false},

		{"m=", true},
		{"m=sum:cpu+", true},
		{"m=sum:cpu{}", true},

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
