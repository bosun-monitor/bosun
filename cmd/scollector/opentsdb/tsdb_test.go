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
	}
}
