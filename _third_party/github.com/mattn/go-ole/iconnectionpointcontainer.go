// +build windows

package ole

import (
	"syscall"
	"unsafe"
)

type IConnectionPointContainer struct {
	IUnknown
}

type IConnectionPointContainerVtbl struct {
	IUnknownVtbl
	EnumConnectionPoints uintptr
	FindConnectionPoint  uintptr
}

func (v *IConnectionPointContainer) VTable() *IConnectionPointContainerVtbl {
	return (*IConnectionPointContainerVtbl)(unsafe.Pointer(v.RawVTable))
}

func (v *IConnectionPointContainer) EnumConnectionPoints(points interface{}) (err error) {
	err = NewError(E_NOTIMPL)
	return
}

func (v *IConnectionPointContainer) FindConnectionPoint(iid *GUID, point **IConnectionPoint) (err error) {
	hr, _, _ := syscall.Syscall(
		v.VTable().FindConnectionPoint,
		3,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(point)))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}
