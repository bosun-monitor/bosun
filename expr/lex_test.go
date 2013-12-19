package conf

import "testing"

type lexTest struct {
	s string
	v bool
}

func isValid(s string) bool {
	l := lex("", s)
	for i := range l.items {
		if i.typ == itemEOF {
			return true
		} else if i.typ == itemError {
			return false
		}
	}
	return false
}

func TestLex(t *testing.T) {
	type lexTest struct {
		s string
		v bool
	}
	valids := []string{
		"1",
		"1 > 0",
		"2 + 3",
		"3 - 4",
		"4 * 5",
		"5 / 6",
		"!10",
		"7 && 8",
		"9 || 10",
		"11 < 12",
		"13 > 14",
		"15 == 16",
		"17 != 18",
		"19 <= 20",
		"21 >= 21",
		"+1",
		"-1",
		".0",
		"-.7e-9",
		"+1 + +1",
		"-1 - -1",
		"0x1",
		`avg([m=sum:sys.cpu.user{host=*}], "1m") < 0.7`,
		`avg([m=sum:sys.cpu.user{host=*-web*}], "1m") < 0.2 || avg([m=sum:sys.cpu.user{host=*-web*}], "1m") > 0.4`,
	}
	invalids := []string{
		"",
		".",
		"avg(1)",
	}
	var lexTests []lexTest
	for _, v := range valids {
		lexTests = append(lexTests, lexTest{v, true})
	}
	for _, v := range invalids {
		lexTests = append(lexTests, lexTest{v, false})
	}
	for _, test := range lexTests {
		v := isValid(test.s)
		if v != test.v {
			t.Errorf("expected %v: %v", test.v, test.s)
		}
	}
}
