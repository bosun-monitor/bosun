package ole

import (
	"syscall"
	"unsafe"
)

type IUnknown struct {
	lpVtbl *pIUnknownVtbl
}

type pIUnknownVtbl struct {
	pQueryInterface uintptr
	pAddRef         uintptr
	pRelease        uintptr
}

type UnknownLike interface {
	QueryInterface(iid *GUID) (disp *IDispatch, err error)
	AddRef() int32
	Release() int32
}

func (v *IUnknown) QueryInterface(iid *GUID) (disp *IDispatch, err error) {
	disp, err = queryInterface(v, iid)
	return
}

func (v *IUnknown) MustQueryInterface(iid *GUID) (disp *IDispatch) {
	disp, _ = queryInterface(v, iid)
	return
}

func (v *IUnknown) AddRef() int32 {
	return addRef(v)
}

func (v *IUnknown) Release() int32 {
	return release(v)
}

func queryInterface(unk *IUnknown, iid *GUID) (disp *IDispatch, err error) {
	hr, _, _ := syscall.Syscall(
		unk.lpVtbl.pQueryInterface,
		3,
		uintptr(unsafe.Pointer(unk)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&disp)))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func addRef(unk *IUnknown) int32 {
	ret, _, _ := syscall.Syscall(
		unk.lpVtbl.pAddRef,
		1,
		uintptr(unsafe.Pointer(unk)),
		0,
		0)
	return int32(ret)
}

func release(unk *IUnknown) int32 {
	ret, _, _ := syscall.Syscall(
		unk.lpVtbl.pRelease,
		1,
		uintptr(unsafe.Pointer(unk)),
		0,
		0)
	return int32(ret)
}
