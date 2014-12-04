package ole

import (
	"unicode/utf16"
	"unsafe"
)

func IsEqualGUID(guid1 *GUID, guid2 *GUID) bool {
	return guid1.Data1 == guid2.Data1 &&
		guid1.Data2 == guid2.Data2 &&
		guid1.Data3 == guid2.Data3 &&
		guid1.Data4[0] == guid2.Data4[0] &&
		guid1.Data4[1] == guid2.Data4[1] &&
		guid1.Data4[2] == guid2.Data4[2] &&
		guid1.Data4[3] == guid2.Data4[3] &&
		guid1.Data4[4] == guid2.Data4[4] &&
		guid1.Data4[5] == guid2.Data4[5] &&
		guid1.Data4[6] == guid2.Data4[6] &&
		guid1.Data4[7] == guid2.Data4[7]
}

func BytePtrToString(p *byte) string {
	a := (*[10000]uint8)(unsafe.Pointer(p))
	i := 0
	for a[i] != 0 {
		i++
	}
	return string(a[:i])
}

func UTF16PtrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	a := (*[10000]uint16)(unsafe.Pointer(p))
	i := 0
	for a[i] != 0 {
		i++
	}
	return string(utf16.Decode(a[:i]))
}

func convertHresultToError(hr uintptr, r2 uintptr, ignore error) (err error) {
	if hr != 0 {
		err = NewError(hr)
	}
	return
}
