package collectors

import "testing"

func TestIsDigit(t *testing.T) {
	if IsDigit("1a3") {
		t.Error("1a3: expected false")
	}
	if !IsDigit("029") {
		t.Error("029: expected true")
	}
}
