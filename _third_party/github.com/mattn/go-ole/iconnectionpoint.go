package ole

import (
	"syscall"
	"unsafe"
)

type IConnectionPoint struct {
	lpVtbl *pIConnectionPointVtbl
}

type pIConnectionPointVtbl struct {
	pQueryInterface              uintptr
	pAddRef                      uintptr
	pRelease                     uintptr
	pGetConnectionInterface      uintptr
	pGetConnectionPointContainer uintptr
	pAdvise                      uintptr
	pUnadvise                    uintptr
	pEnumConnections             uintptr
}

func (v *IConnectionPoint) QueryInterface(iid *GUID) (disp *IDispatch, err error) {
	disp, err = queryInterface((*IUnknown)(unsafe.Pointer(v)), iid)
	return
}

func (v *IConnectionPoint) AddRef() int32 {
	return addRef((*IUnknown)(unsafe.Pointer(v)))
}

func (v *IConnectionPoint) Release() int32 {
	return release((*IUnknown)(unsafe.Pointer(v)))
}

func (v *IConnectionPoint) GetConnectionInterface(piid **GUID) int32 {
	return release((*IUnknown)(unsafe.Pointer(v)))
}

func (v *IConnectionPoint) Advise(unknown *IUnknown) (cookie uint32, err error) {
	hr, _, _ := syscall.Syscall(
		uintptr(v.lpVtbl.pAdvise),
		3,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(unknown)),
		uintptr(unsafe.Pointer(&cookie)))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func (v *IConnectionPoint) Unadvise(cookie uint32) (err error) {
	hr, _, _ := syscall.Syscall(
		uintptr(v.lpVtbl.pUnadvise),
		2,
		uintptr(unsafe.Pointer(v)),
		uintptr(cookie),
		0)
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func (v *IConnectionPoint) EnumConnections(p *unsafe.Pointer) (err error) {
	return NewError(E_NOTIMPL)
}
