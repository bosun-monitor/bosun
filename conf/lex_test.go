package conf

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"
)

func isValid(fname string, t *testing.T) bool {
	t.Log(fname)
	b, err := ioutil.ReadFile(fname)
	if err != nil {
		return false
	}
	l := lex(fname, string(b))
	for i := range l.items {
		t.Log("item", i)
		if i.typ == itemEOF {
			return true
		} else if i.typ == itemError {
			return false
		}
	}
	return false
}

func testDir(dirname string, valid bool, t *testing.T) {
	files, _ := ioutil.ReadDir(dirname)
	for _, f := range files {
		p := filepath.Join(dirname, f.Name())
		if isValid(p, t) != valid {
			t.Fatalf("%v: expected %v", p, valid)
		}
	}
}

func TestLex(t *testing.T) {
	testDir("test_valid", true, t)
	testDir("test_invalid", false, t)
}

func TestPrint(t *testing.T) {
	fname := "test_valid/4"
	c, err := ParseFile(fname)
	if err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Println(c)
	}
}
