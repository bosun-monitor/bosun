// +build !windows

package ole

// safeArrayAccessData returns raw array pointer.
func safeArrayAccessData(safearray *SafeArray) (uintptr, error) {
	return uintptr(0), NewError(E_NOTIMPL)
}

func safeArrayUnaccessData(safearray *SafeArray) error {
	return NewError(E_NOTIMPL)
}

func safeArrayAllocData(safearray *SafeArray) error {
	return NewError(E_NOTIMPL)
}

func safeArrayAllocDescriptor(dimensions uint32) (*SafeArray, error) {
	return nil, NewError(E_NOTIMPL)
}

func safeArrayAllocDescriptorEx(variantType VT, dimensions uint32) (*SafeArray, error) {
	return nil, NewError(E_NOTIMPL)
}

func safeArrayCopy(original *SafeArray) (*SafeArray, error) {
	return nil, NewError(E_NOTIMPL)
}

func safeArrayCopyData(original *SafeArray, duplicate *SafeArray) error {
	return NewError(E_NOTIMPL)
}

func safeArrayCreate(variantType VT, dimensions uint32, bounds *SafeArrayBound) (*SafeArray, error) {
	return nil, NewError(E_NOTIMPL)
}

func safeArrayCreateEx(variantType VT, dimensions uint32, bounds *SafeArrayBound, extra uintptr) (*SafeArray, error) {
	return nil, NewError(E_NOTIMPL)
}

func safeArrayCreateVector(variantType VT, lowerBound int32, length uint32) (*SafeArray, error) {
	return nil, NewError(E_NOTIMPL)
}

func safeArrayCreateVectorEx(variantType VT, lowerBound int32, length uint32, extra uintptr) (*SafeArray, error) {
	return nil, NewError(E_NOTIMPL)
}

func safeArrayDestroy(safearray *SafeArray) error {
	return NewError(E_NOTIMPL)
}

func safeArrayDestroyData(safearray *SafeArray) error {
	return NewError(E_NOTIMPL)
}

func safeArrayDestroyDescriptor(safearray *SafeArray) error {
	return NewError(E_NOTIMPL)
}

func safeArrayGetDim(safearray *SafeArray) (*uint32, error) {
	u := uint32(0)
	return &u, NewError(E_NOTIMPL)
}

func safeArrayGetElementSize(safearray *SafeArray) (*uint32, error) {
	u := uint32(0)
	return &u, NewError(E_NOTIMPL)
}

func safeArrayGetElement(safearray *SafeArray, index int64) (uintptr, error) {
	return uintptr(0), NewError(E_NOTIMPL)
}

func safeArrayGetElementString(safearray *SafeArray, index int64) (string, error) {
	return "", NewError(E_NOTIMPL)
}

func safeArrayGetIID(safearray *SafeArray) (*GUID, error) {
	return nil, NewError(E_NOTIMPL)
}

func safeArrayGetLBound(safearray *SafeArray, dimension uint32) (int64, error) {
	return int64(0), NewError(E_NOTIMPL)
}

func safeArrayGetUBound(safearray *SafeArray, dimension uint32) (int64, error) {
	return int64(0), NewError(E_NOTIMPL)
}

func safeArrayGetVartype(safearray *SafeArray) (uint16, error) {
	return uint16(0), NewError(E_NOTIMPL)
}

func safeArrayLock(safearray *SafeArray) error {
	return NewError(E_NOTIMPL)
}

func safeArrayUnlock(safearray *SafeArray) error {
	return NewError(E_NOTIMPL)
}

func safeArrayPutElement(safearray *SafeArray, index int64, element uintptr) error {
	return NewError(E_NOTIMPL)
}

func safeArrayGetRecordInfo(safearray *SafeArray) (interface{}, error) {
	return nil, NewError(E_NOTIMPL)
}

func safeArraySetRecordInfo(safearray *SafeArray, recordInfo interface{}) error {
	return NewError(E_NOTIMPL)
}
