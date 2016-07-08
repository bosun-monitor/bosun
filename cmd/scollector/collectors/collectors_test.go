package collectors

import (
	"bosun.org/opentsdb"
	"testing"
)

func TestIsDigit(t *testing.T) {
	if IsDigit("1a3") {
		t.Error("1a3: expected false")
	}
	if !IsDigit("029") {
		t.Error("029: expected true")
	}
}

func TestAddTS_Invalid(t *testing.T) {
	mdp := &opentsdb.MultiDataPoint{}
	ts := opentsdb.TagSet{"srv": "%%%"}
	Add(mdp, "aaaa", 42, ts, "", "", "") //don't have a good way to tesst this automatically, but I look for a log message with this line number in it.
	if len(*mdp) != 0 {
		t.Fatal("Shouldn't have added invalid tags.")
	}
}
