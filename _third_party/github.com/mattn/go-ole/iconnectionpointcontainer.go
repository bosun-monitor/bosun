package ole

import (
	"syscall"
	"unsafe"
)

type IConnectionPointContainer struct {
	lpVtbl *pIConnectionPointContainerVtbl
}

type pIConnectionPointContainerVtbl struct {
	pQueryInterface       uintptr
	pAddRef               uintptr
	pRelease              uintptr
	pEnumConnectionPoints uintptr
	pFindConnectionPoint  uintptr
}

func (v *IConnectionPointContainer) QueryInterface(iid *GUID) (disp *IDispatch, err error) {
	disp, err = queryInterface((*IUnknown)(unsafe.Pointer(v)), iid)
	return
}

func (v *IConnectionPointContainer) AddRef() int32 {
	return addRef((*IUnknown)(unsafe.Pointer(v)))
}

func (v *IConnectionPointContainer) Release() int32 {
	return release((*IUnknown)(unsafe.Pointer(v)))
}

func (v *IConnectionPointContainer) EnumConnectionPoints(points interface{}) (err error) {
	err = NewError(E_NOTIMPL)
	return
}

func (v *IConnectionPointContainer) FindConnectionPoint(iid *GUID, point **IConnectionPoint) (err error) {
	hr, _, _ := syscall.Syscall(
		uintptr(v.lpVtbl.pFindConnectionPoint),
		3,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(point)))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}
