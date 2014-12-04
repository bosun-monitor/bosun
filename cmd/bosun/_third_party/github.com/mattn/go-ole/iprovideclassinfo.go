package ole

import (
	"syscall"
	"unsafe"
)

type IProvideClassInfo struct {
	lpVtbl *pIProvideClassInfoVtbl
}

type pIProvideClassInfoVtbl struct {
	pQueryInterface uintptr
	pAddRef         uintptr
	pRelease        uintptr
	pGetClassInfo   uintptr
}

func (v *IProvideClassInfo) QueryInterface(iid *GUID) (disp *IDispatch, err error) {
	disp, err = queryInterface((*IUnknown)(unsafe.Pointer(v)), iid)
	return
}

func (v *IProvideClassInfo) AddRef() int32 {
	return addRef((*IUnknown)(unsafe.Pointer(v)))
}

func (v *IProvideClassInfo) Release() int32 {
	return release((*IUnknown)(unsafe.Pointer(v)))
}

func (v *IProvideClassInfo) GetClassInfo() (cinfo *ITypeInfo, err error) {
	cinfo, err = getClassInfo(v)
	return
}

func getClassInfo(disp *IProvideClassInfo) (tinfo *ITypeInfo, err error) {
	hr, _, _ := syscall.Syscall(
		disp.lpVtbl.pGetClassInfo,
		2,
		uintptr(unsafe.Pointer(disp)),
		uintptr(unsafe.Pointer(&tinfo)),
		0)
	if hr != 0 {
		err = NewError(hr)
	}
	return
}
