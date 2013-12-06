package opentsdb

import "testing"

func TestClean(t *testing.T) {
	clean := "aoeSNVT152-./_"
	if Clean(clean) != clean {
		t.Error("was clean", clean)
	}
	notclean := "euon@#$sar:.03   n  ]e/"
	notcleaned := "euonsar.03ne/"
	if c := Clean(notclean); c != notcleaned {
		t.Error("wasn't cleaned", notclean, "into:", c)
	}
}
