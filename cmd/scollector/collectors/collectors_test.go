package collectors

import (
	"testing"

	"bosun.org/opentsdb"
)

func TestIsDigit(t *testing.T) {
	numbers := []string{"029", "1", "400"}
	notNumbers := []string{"1a3", " 3", "-1", "3.0", "am"}
	for _, s := range notNumbers {
		if IsDigit(s) {
			t.Errorf("%s: not expected to be a digit", s)
		}
	}
	for _, n := range numbers {
		if !IsDigit(n) {
			t.Errorf("%s: expected to be a digit", n)
		}
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
