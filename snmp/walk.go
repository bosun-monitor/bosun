package snmp

import (
	"fmt"
	"io"

	"bosun.org/snmp/asn1"
	"bosun.org/snmp/mib"
)

// Walk is a wrapper for SNMP.Walk.
func Walk(host, community string, oids ...string) (*Rows, error) {
	s, err := New(host, community)
	if err != nil {
		return nil, err
	}
	return s.Walk(oids...)
}

// Rows is the result of a walk. Its cursor starts before the first
// row of the result set. Use Next to advance through the rows:
//
//     rows, err := snmp.Walk(host, community, "ifName")
//     ...
//     for rows.Next() {
//         var name []byte
//         err = rows.Scan(&name)
//         ...
//     }
//     err = rows.Err() // get any error encountered during iteration
//     ...
type Rows struct {
	avail    []row
	last     row
	walkFn   walkFunc
	headText []string
	head     []asn1.ObjectIdentifier
	err      error
	request  requestFunc
}

// row represents individual row.
type row struct {
	instance []int
	bindings []binding
}

// Walk executes a query against host authenticated by the community string,
// retrieving the MIB sub-tree defined by the the given root oids.
func (s *SNMP) Walk(oids ...string) (*Rows, error) {
	rows := &Rows{
		avail:    nil,
		walkFn:   walkN,
		headText: oids,
		head:     lookup(oids...),
		request:  s.do,
	}
	for _, oid := range rows.head {
		rows.last.bindings = append(rows.last.bindings, binding{Name: oid})
	}
	return rows, nil
}

// Next prepares the next result row for reading with the Scan method.
// It returns true on success, false if there is no next result row.
// Every call to Scan, even the first one, must be preceded by a call
// to Next.
func (rows *Rows) Next() bool {
	if len(rows.avail) > 0 {
		return true
	}

	if rows.err != nil {
		if rows.err == io.EOF {
			rows.err = nil
		}
		return false
	}

	row, err := rows.walkFn(rows.last.bindings, rows.request)
	if err != nil {
		if err == io.EOF {
			rows.err = err
		} else {
			rows.err = fmt.Errorf("snmp.Walk: %v", err)
		}
		return false
	}
	rows.avail = row

	for i, r := range rows.avail {
		eof := 0
		for i, b := range r.bindings {
			if !hasPrefix(b.Name, rows.head[i]) {
				eof++
			}
		}
		if eof > 0 {
			if eof < len(r.bindings) {
				rows.err = fmt.Errorf("invalid response: pre-mature end of a column")
				return false
			}
			rows.avail = rows.avail[:i]
			rows.err = io.EOF
			break
		}
	}

	return len(rows.avail) > 0
}

// Scan copies the columns in the current row into the values pointed at by v.
// On success, the id return variable will hold the row id of the current row.
// It is typically an integer or a string.
func (rows *Rows) Scan(v ...interface{}) (id interface{}, err error) {
	if len(v) != len(rows.last.bindings) {
		panic("snmp.Scan: invalid argument count")
	}

	cur := rows.avail[0]
	rows.avail = rows.avail[1:]

	last := rows.last
	rows.last = cur

	for i, a := range last.bindings {
		b := cur.bindings[i]
		if !a.less(b) {
			return nil, fmt.Errorf("invalid response: %v: unordered binding: req=%+v >= resp=%+v",
				rows.headText[i], a.Name, b.Name)
		}
	}

	for i, b := range cur.bindings {
		if err := b.unmarshal(v[i]); err != nil {
			return nil, err
		}
	}

	var want []int
	for i, b := range cur.bindings {
		offset := len(rows.head[i])
		// BUG: out of bounds access
		have := b.Name[offset:]
		if i == 0 {
			want = have
			continue
		}
		if len(have) != len(want) || !hasPrefix(have, want) {
			return nil, fmt.Errorf("invalid response: inconsistent instances")
		}
	}
	id = convertInstance(want)

	return id, nil
}

// convertInstance optionally converts the object instance id from the
// general []byte form to simplified form: either a simple int, or a
// string.
func convertInstance(x []int) interface{} {
	switch {
	case len(x) == 1:
		return x[0]
	default:
		s, ok := toStringInt(x)
		if !ok {
			return x
		}
		return s
	}
}

// Err returns the error, if any, that was encountered during iteration.
func (rows *Rows) Err() error {
	return rows.err
}

type requestFunc func(*request) (*response, error)

// walkFunc is a function that can request one or more rows.
type walkFunc func([]binding, requestFunc) ([]row, error)

// walk1 requests one row.
func walk1(have []binding, rf requestFunc) ([]row, error) {
	req := &request{
		Type:     "GetNext",
		ID:       <-nextID,
		Bindings: have,
	}
	resp, err := rf(req)
	if err != nil {
		return nil, err
	}
	if err := check(resp, req); err != nil {
		return nil, err
	}
	r := row{bindings: resp.Bindings}
	return []row{r}, nil
}

// walkN requests a range of rows.
func walkN(have []binding, rf requestFunc) ([]row, error) {
	req := &request{
		Type:           "GetBulk",
		ID:             <-nextID,
		Bindings:       have,
		NonRepeaters:   0,
		MaxRepetitions: 15,
	}
	resp, err := rf(req)
	if err != nil {
		return nil, err
	}
	if err := check(resp, req); err != nil {
		return nil, err
	}
	received := resp.Bindings
	sent := req.Bindings
	if len(received)%len(sent) != 0 {
		return nil, fmt.Errorf("invalid response: truncated bindings list")
	}
	var list []row
	for len(received) > 0 {
		list = append(list, row{bindings: received[:len(sent)]})
		received = received[len(sent):]
	}
	if len(list) > req.MaxRepetitions {
		return nil, fmt.Errorf("invalid response: peer violated MaxRepetitions, received %d rows, expected at most %d",
			len(list), req.MaxRepetitions)
	}
	return list, nil
}

// lookup maps oids in their symbolic format into numeric format.
func lookup(oids ...string) []asn1.ObjectIdentifier {
	list := make([]asn1.ObjectIdentifier, 0, len(oids))
	for _, o := range oids {
		oid, err := mib.Lookup(o)
		if err != nil {
			panic(err)
		}
		list = append(list, oid)
	}
	return list
}
