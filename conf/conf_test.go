package conf

import (
	"io/ioutil"
	"testing"
)

func TestPrint(t *testing.T) {
	fname := "test.conf"
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(fname, string(b))
	if err != nil {
		t.Error(err)
	}
	if w := c.Alerts["os.high_cpu"].Warn.Text; w != `avg(q("avg:rate:os.cpu{host=ny-nexpose01}", "2m", "")) > 80` {
		t.Error("bad warn:", w)
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
