// +build ignore

package snmp

import (
	_ "fmt"
	"testing"
	_ "time"
)

func TestWalk(t *testing.T) {
	s, err := Walk("localhost", "public", "IF-MIB::ifDescr", "IF-MIB::ifMtu")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	for s.Next() {
		var a, b interface{}
		_, err := s.Scan(&a, &b)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
			break
		}
		if a == nil || b == nil {
			t.Errorf("unexpected nil")
			break
		}
		if _, ok := a.([]byte); !ok {
			t.Errorf("unexpected response: %T", a)
			break
		}
		if _, ok := b.(int64); !ok {
			t.Errorf("unexpected response: %T", b)
			break
		}
		// t.Log(string(a.([]byte)), b)
	}
	if err := s.Err(); err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
}
