package conf

import (
	"io/ioutil"
	"os"
	"regexp"
	"testing"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
)

func TestPrint(t *testing.T) {
	fname := "test.conf"
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("env", "1"); err != nil {
		t.Fatal(err)
	}
	c, err := New(fname, string(b))
	if err != nil {
		t.Fatal(err)
	}
	if w := c.Alerts["os.high_cpu"].Warn.Text; w != `avg(q("avg:rate:os.cpu{host=ny-nexpose01}", "2m", "")) > 80` {
		t.Error("bad warn:", w)
	}
	if w := c.Alerts["m"].Crit.Text; w != `avg(q("", "", "")) > 1` {
		t.Errorf("bad crit: %v", w)
	}
	if w := c.Alerts["braceTest"].Crit.Text; w != `avg(q("o{t}", "", "")) > 1` {
		t.Errorf("bad crit: %v", w)
	}
}

func TestInvalid(t *testing.T) {
	fname := "broken.conf"
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}
	_, err = New(fname, string(b))
	if err == nil {
		t.Error("expected error")
	}
}

func TestSquelch(t *testing.T) {
	type squelchTest struct {
		tags      opentsdb.TagSet
		squelches map[string]*regexp.Regexp
		expect    bool
	}
	tests := []squelchTest{
		{
			opentsdb.TagSet{"host": "test"},
			map[string]*regexp.Regexp{
				"host": regexp.MustCompile("es"),
			},
			true,
		},
	}
	for _, test := range tests {
		got := squelched(test.tags, test.squelches)
		if got != test.expect {
			t.Errorf("for %v: %v, got %v, expected %v", test.tags, test.squelches, got, test.expect)
		}
	}
}
