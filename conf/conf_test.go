package conf

import (
	"fmt"
	"io/ioutil"
	"testing"
)

func TestPrint(t *testing.T) {
	fname := "parse/test_valid/4"
	fname = "../dev.conf"
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(fname, string(b))
	if err != nil {
		t.Error(err)
	} else {
		fmt.Println(c)
	}
}
