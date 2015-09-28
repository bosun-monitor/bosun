package collectors

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestIfstatRE(t *testing.T) {
	// Expected values
	expectedValues := []string{"1", "2", "3", "4"}
	valuesStr := strings.Join(expectedValues, " ")

	// Interfaces that should match the regexp
	expectedInterfaces := []string{
		"eth0",
		"em10_5/10",
		"em10_5",
		"em0",
		"bond0",
		"p10p5_19/2",
		"p10p5_19",
		"p19p10",
		"if0",
		"lan",
		"tun0",
		"br0",
		"docker0",
		"vet09d6143",
		"veth1pl6143",
		"veth2e23fca",
	}

	for _, iface := range expectedInterfaces {
		str := fmt.Sprintf(" %s:%s", iface, valuesStr)

		m := ifstatRE.FindStringSubmatch(str)
		if len(m) != 3 {
			t.Errorf("interface %q not matched", iface)
			continue
		}

		gotInterface := m[1]
		gotValues := strings.Fields(m[2])

		if gotInterface != iface {
			t.Errorf("expected interface %q, got %q", iface, gotInterface)
		}

		if !reflect.DeepEqual(gotValues, expectedValues) {
			t.Errorf("expected values %+v, got %+v", expectedValues, gotValues)
		}
	}
}
