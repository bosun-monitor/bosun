package snmp

import (
	"fmt"

	"bosun.org/snmp/mib"
)

// Get is a wrapper for SNMP.Get.
func Get(host, community string, nameval ...interface{}) error {
	s, err := New(host, community)
	if err != nil {
		return err
	}
	return s.Get(nameval...)
}

// Get retrieves an object by its name. Nameval is a pair of: object name or OID
// (string), and the corresponding target value (pointer to int, string, etc.).
// To retrieve multiple objects in a single transaction, provide multiple name,
// value pairs.
func (s *SNMP) Get(nameval ...interface{}) error {
	switch n := len(nameval); {
	case n == 0:
		return nil
	case n%2 == 1:
		panic("snmp.Get: odd-sized nameval")
	}
	bindings, err := fromPairs(nameval)
	if err != nil {
		return err
	}
	req := &request{
		Type:     "Get",
		Bindings: bindings,
		ID:       <-nextID,
	}
	resp, err := s.do(req)
	if err != nil {
		return err
	}
	if err := check(resp, req); err != nil {
		return err
	}
	for i, b := range resp.Bindings {
		if have, want := b.Name, req.Bindings[i].Name; !have.Equal(want) {
			return fmt.Errorf("snmp: get %v: invalid response: name mismatch", want)
		}
		v := nameval[2*i+1]
		if err := b.unmarshal(v); err != nil {
			return err
		}
	}
	return nil
}

// fromPairs creates bindings from the (name, value) pairs.
func fromPairs(nameval []interface{}) ([]binding, error) {
	var bindings []binding
	for i := 0; i < len(nameval); i += 2 {
		s, ok := nameval[i].(string)
		if !ok {
			panic("pair name not a string")
		}
		oid, err := mib.Lookup(s)
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, binding{Name: oid})
	}
	return bindings, nil
}
