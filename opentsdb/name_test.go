package opentsdb

import (
	"bosun.org/name"
	"testing"
)

func createVaidator(t *testing.T) name.RuneLevelProcessor {
	validator, err := NewOpenTsdbNameProcessor("")
	if err != nil {
		t.Error(err)
		return nil
	}

	return validator
}

func TestNameValidation(t *testing.T) {
	testCases := []struct {
		testString string
		expectPass bool
	}{
		{"abc", true},
		{"one.two.three", true},
		{"1/2/3/4", true},
		{"abc-123/456_xyz", true},
		{"", false},
		{" ", false},
		{"abc$", false},
		{"abc!xyz", false},
	}

	validator := createVaidator(t)

	for _, testCase := range testCases {
		if validator.IsValid(testCase.testString) != testCase.expectPass {
			t.Errorf("Expected IsValid test for '%s' to yeild '%t' not '%t'",
				testCase.testString, testCase.expectPass, !testCase.expectPass)
		}
	}
}

func TestRuneLevelValidation(t *testing.T) {
	testCases := []struct {
		testRune   rune
		expectPass bool
	}{
		{'a', true},
		{'Z', true},
		{'0', true},
		{'9', true},
		{'-', true},
		{'_', true},
		{'.', true},
		{'/', true},

		{'£', false},
		{'$', false},
		{'?', false},
	}

	validator := createVaidator(t)

	for _, testCase := range testCases {
		if validator.IsRuneValid(testCase.testRune) != testCase.expectPass {
			t.Errorf("Expected rune '%c' to be [valid=%t] but it was [valid=%t]",
				testCase.testRune, testCase.expectPass, !testCase.expectPass)
		}
	}
}

func TestFormat(t *testing.T) {
	testCases := []struct {
		test     string
		expected string
	}{
		{"abc", "abc"},
		{"a.b/c_d-e", "a.b/c_d-e"},
		{"one two three", "onetwothree"},
		{"   one    two    three   ", "onetwothree"},
		{"a$b£c:d", "abcd"},
	}

	validator := createVaidator(t)

	for _, testCase := range testCases {
		formatted, err := validator.FormatName(testCase.test)
		if err != nil {
			t.Error(err)
		}

		if formatted != testCase.expected {
			t.Errorf("Expected '%s' but got '%s'", testCase.expected, formatted)
		}
	}
}
