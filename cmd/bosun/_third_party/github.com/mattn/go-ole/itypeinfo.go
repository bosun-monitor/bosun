package ole

import (
	"syscall"
	"unsafe"
)

type ITypeInfo struct {
	lpVtbl *pITypeInfoVtbl
}

type pITypeInfoVtbl struct {
	pQueryInterface       uintptr
	pAddRef               uintptr
	pRelease              uintptr
	pGetTypeAttr          uintptr
	pGetTypeComp          uintptr
	pGetFuncDesc          uintptr
	pGetVarDesc           uintptr
	pGetNames             uintptr
	pGetRefTypeOfImplType uintptr
	pGetImplTypeFlags     uintptr
	pGetIDsOfNames        uintptr
	pInvoke               uintptr
	pGetDocumentation     uintptr
	pGetDllEntry          uintptr
	pGetRefTypeInfo       uintptr
	pAddressOfMember      uintptr
	pCreateInstance       uintptr
	pGetMops              uintptr
	pGetContainingTypeLib uintptr
	pReleaseTypeAttr      uintptr
	pReleaseFuncDesc      uintptr
	pReleaseVarDesc       uintptr
}

func (v *ITypeInfo) QueryInterface(iid *GUID) (disp *IDispatch, err error) {
	disp, err = queryInterface((*IUnknown)(unsafe.Pointer(v)), iid)
	return
}

func (v *ITypeInfo) AddRef() int32 {
	return addRef((*IUnknown)(unsafe.Pointer(v)))
}

func (v *ITypeInfo) Release() int32 {
	return release((*IUnknown)(unsafe.Pointer(v)))
}

func (v *ITypeInfo) GetTypeAttr() (tattr *TYPEATTR, err error) {
	hr, _, _ := syscall.Syscall(
		uintptr(v.lpVtbl.pGetTypeAttr),
		2,
		uintptr(unsafe.Pointer(v)),
		uintptr(unsafe.Pointer(&tattr)),
		0)
	if hr != 0 {
		err = NewError(hr)
	}
	return
}
