package collectors

import (
	"strings"
	"testing"
)

func TestNicSpeed(t *testing.T) {
	notFail := map[string]string{
		"1000\n":    "1000",
		"1000":      "1000",
		"1000\n200": "1000",
	}

	for input, expected := range notFail {
		speed, err := getNicSpeed(strings.NewReader(input))
		if err != nil || expected != speed {
			t.Logf("Should not fail: input: %v, expected: %v, got: %v, err: %v",
				input, expected, speed, err)
		}
	}
	_, err := getNicSpeed(strings.NewReader("blam"))
	if err == nil {
		t.Logf("should have failed: input: blam, err: %v", err)
	}
}
