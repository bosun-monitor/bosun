package conf

import (
	"fmt"
	"testing"
)

func TestMerge(t *testing.T) {
	var orig = `
		#comment
		alert a{
			#hugethinggggggggggg
			crit = 3 > 2
		}
		#anothercomment
		alert b{
			#comment in here
			crit = 2 > 1
		}
		alert c{
			crit = 5 != 0
		}
		#def
	`
	var second = `alert q{
			crit = 3 != 6
		}
	alert b{
		# I made this bigger
			crit = 5 > 100
		# by adding comments
		}
		alert a{
warn = 2>2
		}
		`
	c, err := New("test", orig)
	if err != nil {
		t.Fatal(err)
	}
	rule, err := New("mergeThis", second)
	if err != nil {
		t.Fatal(err)
	}
	c2 := c.Merge(rule)
	fmt.Println(c2.RawText)

}
