package expr

import (
	"fmt"
	"testing"
)

var s = &state{
	host: TSDB_HOST,
}

func TestFuncs(t *testing.T) {
	r, err := Query(s, "avg:proc.stat.cpu{type=*}", "1h")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range r {
		fmt.Println(a.Group)
	}
}
