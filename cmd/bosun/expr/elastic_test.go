package expr

import "testing"

type layoutValue struct {
	layout      string
	value       string
	shouldError bool
}

func TestParseWeek(t *testing.T) {
	inputs := []layoutValue{
		layoutValue{"2006.52", "2016.51", false},
		layoutValue{"2006__52", "2016__20", false},
		layoutValue{"2006.51", "2016.20", true}, // 51 is not a valid specifier
		layoutValue{"2006.52", "2016.63", true}, // 63 is not a valid ISO week)
		layoutValue{"2006.52", "2016.6", true},  // hrmm..?
	}
	for _, input := range inputs {
		_, err := parseWeek(input.layout, input.value)
		if input.shouldError && err == nil {
			t.Error("test should have errored on input %v", input)
		}
		if !input.shouldError && err != nil {
			t.Error(err)
		}
	}
}
