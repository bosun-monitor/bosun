package expr

import (
	"fmt"
	"testing"
)

func q(s string) Query {
	return Query{
		host:  TSDB_HOST,
		query: s,
	}
}

func _TestAvg(t *testing.T) {
	_, err := Avg(q("avg:proc.stat.cpu{type=*}"), "30s")
	if err != nil {
		t.Fatal(err)
	}
}

func _TestDev(t *testing.T) {
	_, err := Dev(q("avg:proc.stat.cpu{type=*}"), "30s")
	if err != nil {
		t.Fatal(err)
	}
}

func TestBand(t *testing.T) {
	r, err := Band(q("avg:proc.stat.cpu{type=*}"), "1h", "1w", 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range r {
		fmt.Println(a.Group, len(a.Values))
	}
}

/*
avg(band([avg:c], "1h", "1w", 8))
avg(band([avg:c, "1h"], "1w", 8))
avg([avg:c], "1h") // bad
avg([avg:c, "1h"])
*/
