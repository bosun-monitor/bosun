package name

import "testing"

func TestLengthValidation(t *testing.T) {
	testCases := []struct {
		testString string
		expectPass bool
	}{
		{"123", true},
		{"123abc", true},
		{"!@£$%^&", true},
		{"0123456789", true},
		{"", false},
		{"1", false},
		{"ab", false},
		{"01234567890", false},
	}

	validator := NewLengthValidator(3, 10)

	for _, testCase := range testCases {
		if validator.IsValid(testCase.testString) != testCase.expectPass {
			t.Errorf("Expected IsValid test for '%s' to yeild '%t' not '%t'",
				testCase.testString, testCase.expectPass, !testCase.expectPass)
		}
	}
}
