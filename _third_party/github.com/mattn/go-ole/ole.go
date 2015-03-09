// +build windows

package ole

import (
	"fmt"
	"strings"
	"syscall"
	"unicode/utf16"
)

type OleError struct {
	hr          uintptr
	description string
	subError    error
}

func errstr(errno int) string {
	// ask windows for the remaining errors
	var flags uint32 = syscall.FORMAT_MESSAGE_FROM_SYSTEM | syscall.FORMAT_MESSAGE_ARGUMENT_ARRAY | syscall.FORMAT_MESSAGE_IGNORE_INSERTS
	b := make([]uint16, 300)
	n, err := syscall.FormatMessage(flags, 0, uint32(errno), 0, b, nil)
	if err != nil {
		return fmt.Sprintf("error %d (FormatMessage failed with: %v)", errno, err)
	}
	// trim terminating \r and \n
	for ; n > 0 && (b[n-1] == '\n' || b[n-1] == '\r'); n-- {
	}
	return string(utf16.Decode(b[:n]))
}

func NewError(hr uintptr) *OleError {
	return &OleError{hr: hr}
}

func NewErrorWithDescription(hr uintptr, description string) *OleError {
	return &OleError{hr: hr, description: description}
}

func NewErrorWithSubError(hr uintptr, description string, err error) *OleError {
	return &OleError{hr: hr, description: description, subError: err}
}

func (v *OleError) Code() uintptr {
	return uintptr(v.hr)
}

func (v *OleError) String() string {
	if v.description != "" {
		return errstr(int(v.hr)) + " (" + v.description + ")"
	}
	return errstr(int(v.hr))
}

func (v *OleError) Error() string {
	return v.String()
}

func (v *OleError) Description() string {
	return v.description
}

func (v *OleError) SubError() error {
	return v.subError
}

type DISPPARAMS struct {
	rgvarg            uintptr
	rgdispidNamedArgs uintptr
	cArgs             uint32
	cNamedArgs        uint32
}

type EXCEPINFO struct {
	wCode             uint16
	wReserved         uint16
	bstrSource        *uint16
	bstrDescription   *uint16
	bstrHelpFile      *uint16
	dwHelpContext     uint32
	pvReserved        uintptr
	pfnDeferredFillIn uintptr
	scode             uint32
}

func (e EXCEPINFO) String() string {
	var src, desc, hlp string
	if e.bstrSource == nil {
		src = "<nil>"
	} else {
		src = BstrToString(e.bstrSource)
	}

	if e.bstrDescription == nil {
		desc = "<nil>"
	} else {
		desc = BstrToString(e.bstrDescription)
	}

	if e.bstrHelpFile == nil {
		hlp = "<nil>"
	} else {
		hlp = BstrToString(e.bstrHelpFile)
	}

	return fmt.Sprintf(
		"wCode: %#x, bstrSource: %v, bstrDescription: %v, bstrHelpFile: %v, dwHelpContext: %#x, scode: %#x",
		e.wCode, src, desc, hlp, e.dwHelpContext, e.scode,
	)
}

func (e EXCEPINFO) Error() string {
	if e.bstrDescription != nil {
		return strings.TrimSpace(BstrToString(e.bstrDescription))
	}

	src := "Unknown"
	if e.bstrSource != nil {
		src = BstrToString(e.bstrSource)
	}

	code := e.scode
	if e.wCode != 0 {
		code = uint32(e.wCode)
	}

	return fmt.Sprintf("%v: %#x", src, code)
}

type PARAMDATA struct {
	Name *int16
	Vt   uint16
}

type METHODDATA struct {
	Name     *uint16
	Data     *PARAMDATA
	Dispid   int32
	Meth     uint32
	CC       int32
	CArgs    uint32
	Flags    uint16
	VtReturn uint32
}

type INTERFACEDATA struct {
	MethodData *METHODDATA
	CMembers   uint32
}

type Point struct {
	X int32
	Y int32
}

type Msg struct {
	Hwnd    uint32
	Message uint32
	Wparam  int32
	Lparam  int32
	Time    uint32
	Pt      Point
}

type TYPEDESC struct {
	Hreftype uint32
	VT       uint16
}

type IDLDESC struct {
	DwReserved uint32
	WIDLFlags  uint16
}

type TYPEATTR struct {
	Guid             GUID
	Lcid             uint32
	dwReserved       uint32
	MemidConstructor int32
	MemidDestructor  int32
	LpstrSchema      *uint16
	CbSizeInstance   uint32
	Typekind         int32
	CFuncs           uint16
	CVars            uint16
	CImplTypes       uint16
	CbSizeVft        uint16
	CbAlignment      uint16
	WTypeFlags       uint16
	WMajorVerNum     uint16
	WMinorVerNum     uint16
	TdescAlias       TYPEDESC
	IdldescType      IDLDESC
}
