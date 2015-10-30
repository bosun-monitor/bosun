package sched

import (
	"testing"

	"bosun.org/cmd/bosun/expr"
	"bosun.org/opentsdb"
)

func TestGroupSets_Single(t *testing.T) {
	states := States{"a{host=foo}": &State{Alert: "a", Group: opentsdb.TagSet{"host": "foo"}, Subject: "aaa"}}
	groups := states.GroupSets(5)
	if len(groups) != 1 {
		t.Fatalf("Expected 1 group. Found %d.", len(groups))
	}
	if len(groups["a{host=foo}"]) == 0 {
		t.Fatal("Expected alert key but couldn't find it.")
	}
}

func TestGroupSets_AboveAndBelow(t *testing.T) {
	aks := map[string]string{
		"a1{host=a}": "a1 on a",
		"a2{host=a}": "a2 on a",
		"a3{host=a}": "a3 on a",
		"a4{host=a}": "a4 on a",
	}
	states := States{}
	for a, sub := range aks {
		ak, err := expr.ParseAlertKey(a)
		if err != nil {
			t.Fatal(err)
		}
		states[ak] = &State{Alert: ak.Name(), Group: ak.Group(), Subject: sub}
	}

	groups := states.GroupSets(5)
	if len(groups) != 4 {
		t.Fatalf("Expected 4 unique groups, but found %d.", len(groups))
	}

	groups = states.GroupSets(4)
	if len(groups) != 1 {
		t.Fatalf("Expected 1 unique group, but found %d.", len(groups))
	}
}

func TestGroupSets_ByAlert(t *testing.T) {
	aks := map[string]string{
		"a{host=a}": "a on a",
		"a{host=b}": "a on b",
		"a{host=c}": "a on c",
		"a{host=d}": "a on d",
	}
	states := States{}
	for a, sub := range aks {
		ak, err := expr.ParseAlertKey(a)
		if err != nil {
			t.Fatal(err)
		}
		states[ak] = &State{Alert: ak.Name(), Group: ak.Group(), Subject: sub}
	}

	groups := states.GroupSets(5)
	if len(groups) != 4 {
		t.Fatalf("Expected 4 unique groups, but found %d.", len(groups))
	}

	groups = states.GroupSets(4)
	if len(groups) != 1 {
		t.Fatalf("Expected 1 unique group, but found %d.", len(groups))
	}
}
