package expr

import "testing"

func _TestAvg(t *testing.T) {
	_, err := Avg("avg:proc.stat.cpu{type=*}", "30s")
	if err != nil {
		t.Fatal(err)
	}
}

func _TestDev(t *testing.T) {
	_, err := Dev("avg:proc.stat.cpu{type=*}", "30s")
	if err != nil {
		t.Fatal(err)
	}
}
