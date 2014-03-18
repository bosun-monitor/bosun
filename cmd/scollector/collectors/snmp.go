package collectors

import (
	"fmt"
	"io"

	"github.com/mjibson/snmp"
)

// snmp_subtree takes an oid and returns all data exactly one level below it. It
// produces an error if there is more than one level below.
func snmp_subtree(host, community, oid string) (map[int]interface{}, error) {
	rows, err := snmp.Walk(host, community, oid)
	if err != nil {
		return nil, err
	}
	m := make(map[int]interface{})
	for rows.Next() {
		var a interface{}
		id, err := rows.Scan(&a)
		if err != nil {
			return nil, err
		}
		switch t := id.(type) {
		case int:
			m[t] = a
		default:
			return nil, fmt.Errorf("snmp subtree: only one level allowed")
		}
	}
	if err := rows.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	return m, nil
}

func snmp_oid(host, community, oid string) (float64, error) {
	var v float64
	err := snmp.Get(host, community, oid, &v)
	return v, err
}
