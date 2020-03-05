package name

import (
	"fmt"
	"strings"
	"testing"
)

const hostnamePattern = `^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9]))*$`

func generateString(length int) string {
	sb := strings.Builder{}
	for i := 0; i < length; i++ {
		sb.WriteString("a")
	}

	return sb.String()
}

func TestRegexpValidation(t *testing.T) {
	testCases := []struct {
		testString string
		expectPass bool
	}{
		{"host", true},
		{"host-name", true},
		{"machine.domain.com", true},
		{"abc123", true},
		{"123.com", true},
		{"192.168.0.12", true},
		{generateString(63), true},
		{fmt.Sprintf("%s.%s.%s.%s.%s", generateString(63), generateString(63), generateString(63), generateString(63), generateString(63)), true},

		{"", false},
		{"   ", false},
		{"-host", false},
		{"host-", false},
		{"host.", false},
		{"host|name", false},
		{"host name", false},
		{generateString(64), false},
		{"abc." + generateString(64), false},
	}

	validator, err := NewRegexpValidator(hostnamePattern)
	if err != nil {
		t.Error(err)
		return
	}

	for _, testCase := range testCases {
		if validator.IsValid(testCase.testString) != testCase.expectPass {
			t.Errorf("Expected IsValid test for '%s' to yeild '%t' not '%t'",
				testCase.testString, testCase.expectPass, !testCase.expectPass)
		}
	}
}
