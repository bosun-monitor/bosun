/*
Package wmi provides a WQL interface for WMI on Windows.

Example code to print names of running processes:

	type Win32_Process struct {
		Name string
	}

	func main() {
		var dst []Win32_Process
		q := wmi.CreateQuery(&dst, "")
		err := wmi.Query(q, &dst)
		if err != nil {
			log.Fatal(err)
		}
		for i, v := range dst {
			println(i, v.Name)
		}
	}

*/
package wmi

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bosun-monitor/bosun/_third_party/github.com/mattn/go-ole"
	"github.com/bosun-monitor/bosun/_third_party/github.com/mattn/go-ole/oleutil"
)

var l = log.New(os.Stdout, "", log.LstdFlags)

var (
	ErrInvalidEntityType = errors.New("wmi: invalid entity type")
	lock                 sync.Mutex
)

// QueryNamespace invokes Query with the given namespace on the local machine.
func QueryNamespace(query string, dst interface{}, namespace string) error {
	return Query(query, dst, nil, namespace)
}

// Query runs the WQL query and appends the values to dst.
//
// dst must have type *[]S or *[]*S, for some struct type S. Fields selected in
// the query must have the same name in dst. Supported types are all signed and
// unsigned integers, time.Time, string, bool, or a pointer to one of those.
// Array types are not supported.
//
// By default, the local machine and default namespace are used. These can be
// changed using connectServerArgs. See
// http://msdn.microsoft.com/en-us/library/aa393720.aspx for details.
func Query(query string, dst interface{}, connectServerArgs ...interface{}) error {
	dv := reflect.ValueOf(dst)
	if dv.Kind() != reflect.Ptr || dv.IsNil() {
		return ErrInvalidEntityType
	}
	dv = dv.Elem()
	mat, elemType := checkMultiArg(dv)
	if mat == multiArgTypeInvalid {
		return ErrInvalidEntityType
	}

	lock.Lock()
	defer lock.Unlock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err != nil {
		oleerr := err.(*ole.OleError)
		// S_FALSE           = 0x00000001 // CoInitializeEx was already called on this thread
		if oleerr.Code() != ole.S_OK && oleerr.Code() != 0x00000001 {
			return err
		}
	} else {
		// Only invoke CoUninitialize if the thread was not initizlied before.
		// This will allow other go packages based on go-ole play along
		// with this library.
		defer ole.CoUninitialize()
	}

	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return err
	}
	defer unknown.Release()

	wmi, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer wmi.Release()

	// service is a SWbemServices
	serviceRaw, err := oleutil.CallMethod(wmi, "ConnectServer", connectServerArgs...)
	if err != nil {
		return err
	}
	service := serviceRaw.ToIDispatch()
	defer serviceRaw.Clear()

	// result is a SWBemObjectSet
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery", query)
	if err != nil {
		return err
	}
	result := resultRaw.ToIDispatch()
	defer resultRaw.Clear()

	count, err := oleInt64(result, "Count")
	if err != nil {
		return err
	}

	// Initialize a slice with Count capacity
	dv.Set(reflect.MakeSlice(dv.Type(), 0, int(count)))

	var errFieldMismatch error
	for i := int64(0); i < count; i++ {
		err := func() error {
			// item is a SWbemObject, but really a Win32_Process
			itemRaw, err := oleutil.CallMethod(result, "ItemIndex", i)
			if err != nil {
				return err
			}
			item := itemRaw.ToIDispatch()
			defer itemRaw.Clear()

			ev := reflect.New(elemType)
			if err = loadEntity(ev.Interface(), item); err != nil {
				if _, ok := err.(*ErrFieldMismatch); ok {
					// We continue loading entities even in the face of field mismatch errors.
					// If we encounter any other error, that other error is returned. Otherwise,
					// an ErrFieldMismatch is returned.
					errFieldMismatch = err
				} else {
					return err
				}
			}
			if mat != multiArgTypeStructPtr {
				ev = ev.Elem()
			}
			dv.Set(reflect.Append(dv, ev))
			return nil
		}()
		if err != nil {
			return err
		}
	}
	return errFieldMismatch
}

// ErrFieldMismatch is returned when a field is to be loaded into a different
// type than the one it was stored from, or when a field is missing or
// unexported in the destination struct.
// StructType is the type of the struct pointed to by the destination argument.
type ErrFieldMismatch struct {
	StructType reflect.Type
	FieldName  string
	Reason     string
}

func (e *ErrFieldMismatch) Error() string {
	return fmt.Sprintf("wmi: cannot load field %q into a %q: %s",
		e.FieldName, e.StructType, e.Reason)
}

var timeType = reflect.TypeOf(time.Time{})

// loadEntity loads a SWbemObject into a struct pointer.
func loadEntity(dst interface{}, src *ole.IDispatch) (errFieldMismatch error) {
	v := reflect.ValueOf(dst).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		isPtr := f.Kind() == reflect.Ptr
		if isPtr {
			ptr := reflect.New(f.Type().Elem())
			f.Set(ptr)
			f = f.Elem()
		}
		n := v.Type().Field(i).Name
		if !f.CanSet() {
			return &ErrFieldMismatch{
				StructType: f.Type(),
				FieldName:  n,
				Reason:     "CanSet() is false",
			}
		}
		prop, err := oleutil.GetProperty(src, n)
		if err != nil {
			errFieldMismatch = &ErrFieldMismatch{
				StructType: f.Type(),
				FieldName:  n,
				Reason:     "no such struct field",
			}
			continue
		}
		defer prop.Clear()

		switch val := prop.Value(); reflect.ValueOf(val).Kind() {
		case reflect.Int64:
			iv := val.(int64)
			switch f.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				f.SetInt(iv)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				f.SetUint(uint64(iv))
			default:
				return &ErrFieldMismatch{
					StructType: f.Type(),
					FieldName:  n,
					Reason:     "not an integer class",
				}
			}
		case reflect.String:
			sv := val.(string)
			iv, err := strconv.ParseInt(sv, 10, 64)
			switch f.Kind() {
			case reflect.String:
				f.SetString(sv)
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if err != nil {
					return err
				}
				f.SetInt(iv)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if err != nil {
					return err
				}
				f.SetUint(uint64(iv))
			case reflect.Struct:
				switch f.Type() {
				case timeType:
					if len(sv) == 25 {
						sv = sv[:22] + "0" + sv[22:]
					}
					t, err := time.Parse("20060102150405.000000-0700", sv)
					if err != nil {
						return err
					}
					f.Set(reflect.ValueOf(t))
				}
			}
		case reflect.Bool:
			bv := val.(bool)
			switch f.Kind() {
			case reflect.Bool:
				f.SetBool(bv)
			default:
				return &ErrFieldMismatch{
					StructType: f.Type(),
					FieldName:  n,
					Reason:     "not a bool",
				}
			}
		default:
			typeof := reflect.TypeOf(val)
			if isPtr && typeof == nil {
				break
			}
			return &ErrFieldMismatch{
				StructType: f.Type(),
				FieldName:  n,
				Reason:     "unsupported type",
			}
		}
	}
	return errFieldMismatch
}

type multiArgType int

const (
	multiArgTypeInvalid multiArgType = iota
	multiArgTypeStruct
	multiArgTypeStructPtr
)

// checkMultiArg checks that v has type []S, []*S for some struct type S.
//
// It returns what category the slice's elements are, and the reflect.Type
// that represents S.
func checkMultiArg(v reflect.Value) (m multiArgType, elemType reflect.Type) {
	if v.Kind() != reflect.Slice {
		return multiArgTypeInvalid, nil
	}
	elemType = v.Type().Elem()
	switch elemType.Kind() {
	case reflect.Struct:
		return multiArgTypeStruct, elemType
	case reflect.Ptr:
		elemType = elemType.Elem()
		if elemType.Kind() == reflect.Struct {
			return multiArgTypeStructPtr, elemType
		}
	}
	return multiArgTypeInvalid, nil
}

func oleInt64(item *ole.IDispatch, prop string) (int64, error) {
	v, err := oleutil.GetProperty(item, prop)
	if err != nil {
		return 0, err
	}
	defer v.Clear()

	i := int64(v.Val)
	return i, nil
}

// CreateQuery returns a WQL query string that queries all columns of src. where
// is an optional string that is appended to the query, to be used with WHERE
// clauses. In such a case, the "WHERE" string should appear at the beginning.
func CreateQuery(src interface{}, where string) string {
	var b bytes.Buffer
	b.WriteString("SELECT ")
	s := reflect.Indirect(reflect.ValueOf(src))
	t := s.Type()
	if s.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return ""
	}
	var fields []string
	for i := 0; i < t.NumField(); i++ {
		fields = append(fields, t.Field(i).Name)
	}
	b.WriteString(strings.Join(fields, ", "))
	b.WriteString(" FROM ")
	b.WriteString(t.Name())
	b.WriteString(" " + where)
	return b.String()
}
