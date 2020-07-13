package name

import (
	"testing"
	"unicode"
)

func createBasicVaidator(t *testing.T) RuneLevelValidator {
	isValidTest := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' || r == '/'
	}
	validator, err := NewBasicValidator(false, isValidTest)
	if err != nil {
		t.Error(err)
		return nil
	}

	return validator
}

func TestBasicValidation(t *testing.T) {
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

	validator := createBasicVaidator(t)

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

	validator := createBasicVaidator(t)

	for _, testCase := range testCases {
		if validator.IsRuneValid(testCase.testRune) != testCase.expectPass {
			t.Errorf("Expected rune '%c' to be [valid=%t] but it was [valid=%t]",
				testCase.testRune, testCase.expectPass, !testCase.expectPass)
		}
	}
}
