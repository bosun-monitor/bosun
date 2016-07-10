package conf

import (
	"regexp"
	"testing"

	"bosun.org/opentsdb"
)

func TestSquelch(t *testing.T) {
	s := Squelches{
		[]Squelch{
			map[string]*regexp.Regexp{
				"x": regexp.MustCompile("ab"),
				"y": regexp.MustCompile("bc"),
			},
			map[string]*regexp.Regexp{
				"x": regexp.MustCompile("ab"),
				"z": regexp.MustCompile("de"),
			},
		},
	}
	type squelchTest struct {
		tags   opentsdb.TagSet
		expect bool
	}
	tests := []squelchTest{
		{
			opentsdb.TagSet{
				"x": "ab",
			},
			false,
		},
		{
			opentsdb.TagSet{
				"x": "abe",
				"y": "obcx",
			},
			true,
		},
		{
			opentsdb.TagSet{
				"x": "abe",
				"z": "obcx",
			},
			false,
		},
		{
			opentsdb.TagSet{
				"x": "abe",
				"z": "ouder",
			},
			true,
		},
		{
			opentsdb.TagSet{
				"x": "ae",
				"y": "bc",
				"z": "de",
			},
			false,
		},
	}
	for _, test := range tests {
		got := s.Squelched(test.tags)
		if got != test.expect {
			t.Errorf("for %v got %v, expected %v", test.tags, got, test.expect)
		}
	}
}
