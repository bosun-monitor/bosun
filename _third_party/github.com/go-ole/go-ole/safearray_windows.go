// +build windows

package ole

import (
	"unsafe"
)

var (
	procSafeArrayAccessData, _        = modoleaut32.FindProc("SafeArrayAccessData")
	procSafeArrayAllocData, _         = modoleaut32.FindProc("SafeArrayAllocData")
	procSafeArrayAllocDescriptor, _   = modoleaut32.FindProc("SafeArrayAllocDescriptor")
	procSafeArrayAllocDescriptorEx, _ = modoleaut32.FindProc("SafeArrayAllocDescriptorEx")
	procSafeArrayCopy, _              = modoleaut32.FindProc("SafeArrayCopy")
	procSafeArrayCopyData, _          = modoleaut32.FindProc("SafeArrayCopyData")
	procSafeArrayCreate, _            = modoleaut32.FindProc("SafeArrayCreate")
	procSafeArrayCreateEx, _          = modoleaut32.FindProc("SafeArrayCreateEx")
	procSafeArrayCreateVector, _      = modoleaut32.FindProc("SafeArrayCreateVector")
	procSafeArrayCreateVectorEx, _    = modoleaut32.FindProc("SafeArrayCreateVectorEx")
	procSafeArrayDestroy, _           = modoleaut32.FindProc("SafeArrayDestroy")
	procSafeArrayDestroyData, _       = modoleaut32.FindProc("SafeArrayDestroyData")
	procSafeArrayDestroyDescriptor, _ = modoleaut32.FindProc("SafeArrayDestroyDescriptor")
	procSafeArrayGetDim, _            = modoleaut32.FindProc("SafeArrayGetDim")
	procSafeArrayGetElement, _        = modoleaut32.FindProc("SafeArrayGetElement")
	procSafeArrayGetElemsize, _       = modoleaut32.FindProc("SafeArrayGetElemsize")
	procSafeArrayGetIID, _            = modoleaut32.FindProc("SafeArrayGetIID")
	procSafeArrayGetLBound, _         = modoleaut32.FindProc("SafeArrayGetLBound")
	procSafeArrayGetUBound, _         = modoleaut32.FindProc("SafeArrayGetUBound")
	procSafeArrayGetVartype, _        = modoleaut32.FindProc("SafeArrayGetVartype")
	procSafeArrayLock, _              = modoleaut32.FindProc("SafeArrayLock")
	procSafeArrayPtrOfIndex, _        = modoleaut32.FindProc("SafeArrayPtrOfIndex")
	procSafeArrayUnaccessData, _      = modoleaut32.FindProc("SafeArrayUnaccessData")
	procSafeArrayUnlock, _            = modoleaut32.FindProc("SafeArrayUnlock")
	procSafeArrayPutElement, _        = modoleaut32.FindProc("SafeArrayPutElement")
	//procSafeArrayRedim, _             = modoleaut32.FindProc("SafeArrayRedim") // TODO
	//procSafeArraySetIID, _            = modoleaut32.FindProc("SafeArraySetIID") // TODO
	//procSafeArrayGetRecordInfo, _     = modoleaut32.FindProc("SafeArrayGetRecordInfo") // TODO
	//procSafeArraySetRecordInfo, _     = modoleaut32.FindProc("SafeArraySetRecordInfo") // TODO
)

// Returns Raw Array
// Todo: Test
func safeArrayAccessData(safearray *SafeArray) (element uintptr, err error) {
	err = convertHresultToError(
		procSafeArrayAccessData.Call(
			uintptr(unsafe.Pointer(safearray)),
			uintptr(unsafe.Pointer(&element))))
	return
}

func safeArrayUnaccessData(safearray *SafeArray) (err error) {
	err = convertHresultToError(procSafeArrayUnaccessData.Call(uintptr(unsafe.Pointer(safearray))))
	return
}

func safeArrayAllocData(safearray *SafeArray) (err error) {
	err = convertHresultToError(procSafeArrayAllocData.Call(uintptr(unsafe.Pointer(safearray))))
	return
}

func safeArrayAllocDescriptor(dimensions uint32) (safearray *SafeArray, err error) {
	err = convertHresultToError(
		procSafeArrayAllocDescriptor.Call(uintptr(dimensions), uintptr(unsafe.Pointer(&safearray))))
	return
}

func safeArrayAllocDescriptorEx(variantType VT, dimensions uint32) (safearray *SafeArray, err error) {
	err = convertHresultToError(
		procSafeArrayAllocDescriptorEx.Call(
			uintptr(variantType),
			uintptr(dimensions),
			uintptr(unsafe.Pointer(&safearray))))
	return
}

func safeArrayCopy(original *SafeArray) (safearray *SafeArray, err error) {
	err = convertHresultToError(
		procSafeArrayCopy.Call(
			uintptr(unsafe.Pointer(original)),
			uintptr(unsafe.Pointer(&safearray))))
	return
}

func safeArrayCopyData(original *SafeArray, duplicate *SafeArray) (err error) {
	err = convertHresultToError(
		procSafeArrayCopyData.Call(
			uintptr(unsafe.Pointer(original)),
			uintptr(unsafe.Pointer(duplicate))))
	return
}

func safeArrayCreate(variantType VT, dimensions uint32, bounds *SafeArrayBound) (safearray *SafeArray, err error) {
	sa, _, err := procSafeArrayCreate.Call(
		uintptr(variantType),
		uintptr(dimensions),
		uintptr(unsafe.Pointer(bounds)))
	safearray = (*SafeArray)(unsafe.Pointer(&sa))
	return
}

func safeArrayCreateEx(variantType VT, dimensions uint32, bounds *SafeArrayBound, extra uintptr) (safearray *SafeArray, err error) {
	sa, _, err := procSafeArrayCreateEx.Call(
		uintptr(variantType),
		uintptr(dimensions),
		uintptr(unsafe.Pointer(bounds)),
		extra)
	safearray = (*SafeArray)(unsafe.Pointer(sa))
	return
}

func safeArrayCreateVector(variantType VT, lowerBound int32, length uint32) (safearray *SafeArray, err error) {
	sa, _, err := procSafeArrayCreateVector.Call(
		uintptr(variantType),
		uintptr(lowerBound),
		uintptr(length))
	safearray = (*SafeArray)(unsafe.Pointer(sa))
	return
}

func safeArrayCreateVectorEx(variantType VT, lowerBound int32, length uint32, extra uintptr) (safearray *SafeArray, err error) {
	sa, _, err := procSafeArrayCreateVectorEx.Call(
		uintptr(variantType),
		uintptr(lowerBound),
		uintptr(length),
		extra)
	safearray = (*SafeArray)(unsafe.Pointer(sa))
	return
}

func safeArrayDestroy(safearray *SafeArray) (err error) {
	err = convertHresultToError(procSafeArrayDestroy.Call(uintptr(unsafe.Pointer(safearray))))
	return
}

func safeArrayDestroyData(safearray *SafeArray) (err error) {
	err = convertHresultToError(procSafeArrayDestroyData.Call(uintptr(unsafe.Pointer(safearray))))
	return
}

func safeArrayDestroyDescriptor(safearray *SafeArray) (err error) {
	err = convertHresultToError(procSafeArrayDestroyDescriptor.Call(uintptr(unsafe.Pointer(safearray))))
	return
}

func safeArrayGetDim(safearray *SafeArray) (dimensions *uint32, err error) {
	l, _, err := procSafeArrayGetDim.Call(uintptr(unsafe.Pointer(safearray)))
	dimensions = (*uint32)(unsafe.Pointer(l))
	return
}

func safeArrayGetElementSize(safearray *SafeArray) (length *uint32, err error) {
	l, _, err := procSafeArrayGetElemsize.Call(uintptr(unsafe.Pointer(safearray)))
	length = (*uint32)(unsafe.Pointer(l))
	return
}

func safeArrayGetElement(safearray *SafeArray, index int64) (element uintptr, err error) {
	err = convertHresultToError(
		procSafeArrayGetElement.Call(
			uintptr(unsafe.Pointer(safearray)),
			uintptr(unsafe.Pointer(&index)),
			uintptr(unsafe.Pointer(&element))))
	return
}

func safeArrayGetElementString(safearray *SafeArray, index int64) (str string, err error) {
	var element *int16
	err = convertHresultToError(
		procSafeArrayGetElement.Call(
			uintptr(unsafe.Pointer(safearray)),
			uintptr(unsafe.Pointer(&index)),
			uintptr(unsafe.Pointer(&element))))
	str = BstrToString(*(**uint16)(unsafe.Pointer(&element)))
	SysFreeString(element)
	return
}

func safeArrayGetIID(safearray *SafeArray) (guid *GUID, err error) {
	err = convertHresultToError(
		procSafeArrayGetIID.Call(
			uintptr(unsafe.Pointer(safearray)),
			uintptr(unsafe.Pointer(&guid))))
	return
}

func safeArrayGetLBound(safearray *SafeArray, dimension uint32) (lowerBound int64, err error) {
	err = convertHresultToError(
		procSafeArrayGetLBound.Call(
			uintptr(unsafe.Pointer(safearray)),
			uintptr(dimension),
			uintptr(unsafe.Pointer(&lowerBound))))
	return
}

func safeArrayGetUBound(safearray *SafeArray, dimension uint32) (upperBound int64, err error) {
	err = convertHresultToError(
		procSafeArrayGetUBound.Call(
			uintptr(unsafe.Pointer(safearray)),
			uintptr(dimension),
			uintptr(unsafe.Pointer(&upperBound))))
	return
}

func safeArrayGetVartype(safearray *SafeArray) (varType uint16, err error) {
	err = convertHresultToError(
		procSafeArrayGetVartype.Call(
			uintptr(unsafe.Pointer(safearray)),
			uintptr(unsafe.Pointer(&varType))))
	return
}

func safeArrayLock(safearray *SafeArray) (err error) {
	err = convertHresultToError(procSafeArrayLock.Call(uintptr(unsafe.Pointer(safearray))))
	return
}

func safeArrayUnlock(safearray *SafeArray) (err error) {
	err = convertHresultToError(procSafeArrayUnlock.Call(uintptr(unsafe.Pointer(safearray))))
	return
}

func safeArrayPutElement(safearray *SafeArray, index int64, element uintptr) (err error) {
	err = convertHresultToError(
		procSafeArrayPutElement.Call(
			uintptr(unsafe.Pointer(safearray)),
			uintptr(unsafe.Pointer(&index)),
			uintptr(unsafe.Pointer(element))))
	return
}

/*
// TODO: Must implement IRecordInfo interface for this to return.
func safeArrayGetRecordInfo(safearray *SafeArray) (recordInfo interface{}, err error) {
	err = convertHresultToError(
		procSafeArrayGetRecordInfo.Call(
			uintptr(unsafe.Pointer(safearray)),
			uintptr(unsafe.Pointer(&recordInfo))))
	return
}

// TODO: Must implement IRecordInfo interface for this to work.
func safeArraySetRecordInfo(safearray *SafeArray, recordInfo interface{}) (err error) {
	err = convertHresultToError(
		procSafeArraySetRecordInfo.Call(
			uintptr(unsafe.Pointer(safearray)),
			uintptr(unsafe.Pointer(recordInfo))))
	return
}
*/
