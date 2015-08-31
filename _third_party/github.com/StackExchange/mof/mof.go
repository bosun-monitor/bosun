// Package mof parses and marshals Managed Object Format (MOF) structures.
package mof

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// Unmarshal parses the MOF-encoded data and stores the result in the value
// pointed to by v.
//
// Unmarshal allocates maps and slices as necessary.
//
// To unmarshal MOF into an interface value, Unmarshal stores a
// map[string]interface{} in the interface value.
//
// To unmarshal MOF into a struct, Unmarshal matches incoming object keys
// to either the struct field name or its tag, preferring an exact match
// but also accepting a case-insensitive match. The encoding/json package
// is used to perform this matching, so json struct tags are honored.
//
func Unmarshal(data []byte, v interface{}) error {
	t, err := parse(data)
	if err != nil {
		return err
	}
	dv := reflect.ValueOf(v)
	if dv.Kind() != reflect.Ptr || dv.IsNil() {
		return errors.New("mof: requires pointer type")
	}
	dv = dv.Elem()
	value := t.Root.Value()
	switch dv.Kind() {
	case reflect.Interface:
		dv.Set(reflect.ValueOf(value))
	case reflect.Struct:
		return populateStruct(v, value)
	default:
		return fmt.Errorf("unusable type: %T", v)
	}
	return nil
}

func populateStruct(dst, src interface{}) error {
	b, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
