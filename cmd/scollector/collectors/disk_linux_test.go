package collectors

import "testing"

func TestFilterVolumes(t *testing.T) {
	in := []string{"/dev/zero", "/etc/passwd"}
	out := []string{"/dev/zero"}
	got := filterVolumes(in)
	if !compTab(got, out) {
		t.Fatalf("filterVolume: got: %v, expected: %v", got, out)
	}
}
