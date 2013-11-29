package conf

import (
	"testing"
)

func TestLex(t *testing.T) {
	type lexTest struct {
		input string
		valid bool
	}
	tests := []lexTest{
		{
			`[testeoo] ntoh = eo02oen`,
			true,
		},
		{
			`etnohu=2323eee oenuhoaetnsu oentu
[testeoo]
ntoh = eo02oen
oenoet=2309,.0uh  o0.,
[ok]
yessir=,.,o ehountes`,
			true,
		},
		{
			`yeasnot`,
			false,
		},
		{
			`[aoenth`,
			false,
		},
		{
			`aoenth=`,
			true,
		},
		{
			`oaesunth=aoesunth`,
			true,
		},
	}

Loop:
	for _, test := range tests {
		l := lex("test", test.input)
		for i := range l.items {
			if i.typ == itemEOF {
				break
			} else if i.typ == itemError {
				if test.valid {
					t.Fatal("Produced error on valid input:", test.input)
				} else {
					continue Loop
				}
			}
		}
		if !test.valid {
			t.Fatal("Expected an error:", test.input)
		}
	}
}
