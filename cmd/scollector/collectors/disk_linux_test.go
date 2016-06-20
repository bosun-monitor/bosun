package collectors

import (
	"strings"
	"testing"
)

func TestMdadmLinux(t *testing.T) {
	// all those lines should not return anything
	emptyTests := []string{
		"",
		"md",
		"md123 :",
		"md125: rebuild raid1 sda2[9]",
	}

	// all those tests should return some md
	goodTests := []string{
		"md125 : rebuilding raid5 sda1[1]",
		"md123 : active raid1\n" +
			"md124 : active raid1",
	}

	for _, s := range emptyTests {
		str := strings.NewReader(s)
		md, err := mdadmLinux(str)
		if len(md) != 0 || err != nil {
			t.Fatalf("should not return any md: %s, returned: %s", str, md)
		}
	}

	for _, s := range goodTests {
		str := strings.NewReader(s)
		md, err := mdadmLinux(str)
		if len(md) == 0 || err != nil {
			t.Fatalf("should return some md: %s, returned: %s", str, md)
		}
	}

}
