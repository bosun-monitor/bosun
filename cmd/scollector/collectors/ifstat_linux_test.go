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
			t.Fatalf("Should not fail: input: %v, expected: %v, got: %v, err: %v",
				input, expected, speed, err)
		}
	}
	_, err := getNicSpeed(strings.NewReader("blam"))
	if err == nil {
		t.Fatalf("should have failed: input: blam, err: %v", err)
	}
}

func TestParseProcNetDev(t *testing.T) {
	failures := []string{
		"Inter-|   Receive                                                |  Transmit",
		" face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed",
	}
	for _, fail := range failures {
		_, _, err := parseProcNetDev(fail)
		if err == nil {
			t.Fatalf("should have returned an error")
		}
	}

	success := "  eth0: 3872945130 27157097    2 21529    3     4          5      1093 4914714972 10290968    6    7    8     9      10        11"
	intf, vals, err := parseProcNetDev(success)
	if intf != "eth0" {
		t.Fatalf("wrong interface: got %v, wants: %v", intf, "eth0")
	}
	if len(vals) != 16 {
		t.Fatalf("wrong values")
	}
	if err != nil {
		t.Fatalf("should not have failed: err: %v", err)
	}
}
