// +build windows

package ole

import (
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	procCoInitialize, _       = modole32.FindProc("CoInitialize")
	procCoInitializeEx, _     = modole32.FindProc("CoInitializeEx")
	procCoUninitialize, _     = modole32.FindProc("CoUninitialize")
	procCoCreateInstance, _   = modole32.FindProc("CoCreateInstance")
	procCoTaskMemFree, _      = modole32.FindProc("CoTaskMemFree")
	procCLSIDFromProgID, _    = modole32.FindProc("CLSIDFromProgID")
	procCLSIDFromString, _    = modole32.FindProc("CLSIDFromString")
	procStringFromCLSID, _    = modole32.FindProc("StringFromCLSID")
	procStringFromIID, _      = modole32.FindProc("StringFromIID")
	procIIDFromString, _      = modole32.FindProc("IIDFromString")
	procGetUserDefaultLCID, _ = modkernel32.FindProc("GetUserDefaultLCID")
	procCopyMemory, _         = modkernel32.FindProc("RtlMoveMemory")
	procVariantInit, _        = modoleaut32.FindProc("VariantInit")
	procVariantClear, _       = modoleaut32.FindProc("VariantClear")
	procSysAllocString, _     = modoleaut32.FindProc("SysAllocString")
	procSysAllocStringLen, _  = modoleaut32.FindProc("SysAllocStringLen")
	procSysFreeString, _      = modoleaut32.FindProc("SysFreeString")
	procSysStringLen, _       = modoleaut32.FindProc("SysStringLen")
	procCreateDispTypeInfo, _ = modoleaut32.FindProc("CreateDispTypeInfo")
	procCreateStdDispatch, _  = modoleaut32.FindProc("CreateStdDispatch")
	procGetActiveObject, _    = modoleaut32.FindProc("GetActiveObject")

	procGetMessageW, _      = moduser32.FindProc("GetMessageW")
	procDispatchMessageW, _ = moduser32.FindProc("DispatchMessageW")
)

func coInitialize() (err error) {
	// http://msdn.microsoft.com/en-us/library/windows/desktop/ms678543(v=vs.85).aspx
	// Suggests that no value should be passed to CoInitialized.
	// Could just be Call() since the parameter is optional. <-- Needs testing to be sure.
	hr, _, _ := procCoInitialize.Call(uintptr(0))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func coInitializeEx(coinit uint32) (err error) {
	// http://msdn.microsoft.com/en-us/library/windows/desktop/ms695279(v=vs.85).aspx
	// Suggests that the first parameter is not only optional but should always be NULL.
	hr, _, _ := procCoInitializeEx.Call(uintptr(0), uintptr(coinit))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func CoInitialize(p uintptr) (err error) {
	// p is ignored and won't be used.
	// Avoid any variable not used errors.
	p = uintptr(0)
	return coInitialize()
}

func CoInitializeEx(p uintptr, coinit uint32) (err error) {
	// Avoid any variable not used errors.
	p = uintptr(0)
	return coInitializeEx(coinit)
}

func CoUninitialize() {
	procCoUninitialize.Call()
}

func CoTaskMemFree(memptr uintptr) {
	procCoTaskMemFree.Call(memptr)
}

func CLSIDFromProgID(progId string) (clsid *GUID, err error) {
	var guid GUID
	lpszProgID := uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(progId)))
	hr, _, _ := procCLSIDFromProgID.Call(lpszProgID, uintptr(unsafe.Pointer(&guid)))
	if hr != 0 {
		err = NewError(hr)
	}
	clsid = &guid
	return
}

func CLSIDFromString(str string) (clsid *GUID, err error) {
	var guid GUID
	lpsz := uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(str)))
	hr, _, _ := procCLSIDFromString.Call(lpsz, uintptr(unsafe.Pointer(&guid)))
	if hr != 0 {
		err = NewError(hr)
	}
	clsid = &guid
	return
}

func StringFromCLSID(clsid *GUID) (str string, err error) {
	var p *uint16
	hr, _, _ := procStringFromCLSID.Call(uintptr(unsafe.Pointer(clsid)), uintptr(unsafe.Pointer(&p)))
	if hr != 0 {
		err = NewError(hr)
	}
	str = LpOleStrToString(p)
	return
}

func IIDFromString(progId string) (clsid *GUID, err error) {
	var guid GUID
	lpsz := uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(progId)))
	hr, _, _ := procIIDFromString.Call(lpsz, uintptr(unsafe.Pointer(&guid)))
	if hr != 0 {
		err = NewError(hr)
	}
	clsid = &guid
	return
}

func StringFromIID(iid *GUID) (str string, err error) {
	var p *uint16
	hr, _, _ := procStringFromIID.Call(uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&p)))
	if hr != 0 {
		err = NewError(hr)
	}
	str = LpOleStrToString(p)
	return
}

func CreateInstance(clsid *GUID, iid *GUID) (unk *IUnknown, err error) {
	if iid == nil {
		iid = IID_IUnknown
	}
	hr, _, _ := procCoCreateInstance.Call(
		uintptr(unsafe.Pointer(clsid)),
		0,
		CLSCTX_SERVER,
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&unk)))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func GetActiveObject(clsid *GUID, iid *GUID) (unk *IUnknown, err error) {
	if iid == nil {
		iid = IID_IUnknown
	}
	hr, _, _ := procGetActiveObject.Call(
		uintptr(unsafe.Pointer(clsid)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&unk)))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func VariantInit(v *VARIANT) (err error) {
	hr, _, _ := procVariantInit.Call(uintptr(unsafe.Pointer(v)))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func VariantClear(v *VARIANT) (err error) {
	hr, _, _ := procVariantClear.Call(uintptr(unsafe.Pointer(v)))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func SysAllocString(v string) (ss *int16) {
	pss, _, _ := procSysAllocString.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(v))))
	ss = (*int16)(unsafe.Pointer(pss))
	return
}

func SysAllocStringLen(v string) (ss *int16) {
	utf16 := utf16.Encode([]rune(v + "\x00"))
	ptr := &utf16[0]

	pss, _, _ := procSysAllocStringLen.Call(uintptr(unsafe.Pointer(ptr)), uintptr(len(utf16)-1))
	ss = (*int16)(unsafe.Pointer(pss))
	return
}

func SysFreeString(v *int16) (err error) {
	hr, _, _ := procSysFreeString.Call(uintptr(unsafe.Pointer(v)))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func SysStringLen(v *int16) uint32 {
	l, _, _ := procSysStringLen.Call(uintptr(unsafe.Pointer(v)))
	return uint32(l)
}

func CreateStdDispatch(unk *IUnknown, v uintptr, ptinfo *IUnknown) (disp *IDispatch, err error) {
	hr, _, _ := procCreateStdDispatch.Call(
		uintptr(unsafe.Pointer(unk)),
		v,
		uintptr(unsafe.Pointer(ptinfo)),
		uintptr(unsafe.Pointer(&disp)))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func CreateDispTypeInfo(idata *INTERFACEDATA) (pptinfo *IUnknown, err error) {
	hr, _, _ := procCreateDispTypeInfo.Call(
		uintptr(unsafe.Pointer(idata)),
		uintptr(GetUserDefaultLCID()),
		uintptr(unsafe.Pointer(&pptinfo)))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func copyMemory(dest unsafe.Pointer, src unsafe.Pointer, length uint32) {
	procCopyMemory.Call(uintptr(dest), uintptr(src), uintptr(length))
}

func GetUserDefaultLCID() (lcid uint32) {
	ret, _, _ := procGetUserDefaultLCID.Call()
	lcid = uint32(ret)
	return
}

func GetMessage(msg *Msg, hwnd uint32, MsgFilterMin uint32, MsgFilterMax uint32) (ret int32, err error) {
	r0, _, err := procGetMessageW.Call(uintptr(unsafe.Pointer(msg)), uintptr(hwnd), uintptr(MsgFilterMin), uintptr(MsgFilterMax))
	ret = int32(r0)
	return
}

func DispatchMessage(msg *Msg) (ret int32) {
	r0, _, _ := procDispatchMessageW.Call(uintptr(unsafe.Pointer(msg)))
	ret = int32(r0)
	return
}
