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
	_, err = New(fname, string(b))
	if err != nil {
		t.Error(err)
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
