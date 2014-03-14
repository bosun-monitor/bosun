package collectors

import (
	"io"

	"github.com/mjibson/snmp"
)

func snmp_subtree(host, community, oid string) (map[interface{}]interface{}, error) {
	rows, err := snmp.Walk(host, community, oid)
	if err != nil {
		return nil, err
	}
	m := make(map[interface{}]interface{})
	for rows.Next() {
		var a interface{}
		id, err := rows.Scan(&a)
		if err != nil {
			return nil, err
		}
		m[id] = a
	}
	if err := rows.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	return m, nil
}
