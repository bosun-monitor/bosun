// +build windows

package oleutil

import (
	"reflect"
	"syscall"
	"unsafe"

	"bosun.org/_third_party/github.com/mattn/go-ole"
)

func CreateObject(progId string) (unknown *ole.IUnknown, err error) {
	var clsid *ole.GUID
	clsid, err = ole.CLSIDFromProgID(progId)
	if err != nil {
		clsid, err = ole.CLSIDFromString(progId)
		if err != nil {
			return
		}
	}

	unknown, err = ole.CreateInstance(clsid, ole.IID_IUnknown)
	if err != nil {
		return
	}
	return
}

func GetActiveObject(progId string) (unknown *ole.IUnknown, err error) {
	var clsid *ole.GUID
	clsid, err = ole.CLSIDFromProgID(progId)
	if err != nil {
		clsid, err = ole.CLSIDFromString(progId)
		if err != nil {
			return
		}
	}

	unknown, err = ole.GetActiveObject(clsid, ole.IID_IUnknown)
	if err != nil {
		return
	}
	return
}

func CallMethod(disp *ole.IDispatch, name string, params ...interface{}) (result *ole.VARIANT, err error) {
	var dispid []int32
	dispid, err = disp.GetIDsOfName([]string{name})
	if err != nil {
		return
	}
	result, err = disp.Invoke(dispid[0], ole.DISPATCH_METHOD, params...)
	return
}

func MustCallMethod(disp *ole.IDispatch, name string, params ...interface{}) (result *ole.VARIANT) {
	r, err := CallMethod(disp, name, params...)
	if err != nil {
		panic(err.Error())
	}
	return r
}

func GetProperty(disp *ole.IDispatch, name string, params ...interface{}) (result *ole.VARIANT, err error) {
	var dispid []int32
	dispid, err = disp.GetIDsOfName([]string{name})
	if err != nil {
		return
	}
	result, err = disp.Invoke(dispid[0], ole.DISPATCH_PROPERTYGET, params...)
	return
}

func MustGetProperty(disp *ole.IDispatch, name string, params ...interface{}) (result *ole.VARIANT) {
	r, err := GetProperty(disp, name, params...)
	if err != nil {
		panic(err.Error())
	}
	return r
}

func PutProperty(disp *ole.IDispatch, name string, params ...interface{}) (result *ole.VARIANT, err error) {
	var dispid []int32
	dispid, err = disp.GetIDsOfName([]string{name})
	if err != nil {
		return
	}
	result, err = disp.Invoke(dispid[0], ole.DISPATCH_PROPERTYPUT, params...)
	return
}

func MustPutProperty(disp *ole.IDispatch, name string, params ...interface{}) (result *ole.VARIANT) {
	r, err := PutProperty(disp, name, params...)
	if err != nil {
		panic(err.Error())
	}
	return r
}

type stdDispatch struct {
	lpVtbl  *stdDispatchVtbl
	ref     int32
	iid     *ole.GUID
	iface   interface{}
	funcMap map[string]int32
}

type stdDispatchVtbl struct {
	pQueryInterface   uintptr
	pAddRef           uintptr
	pRelease          uintptr
	pGetTypeInfoCount uintptr
	pGetTypeInfo      uintptr
	pGetIDsOfNames    uintptr
	pInvoke           uintptr
}

func dispQueryInterface(this *ole.IUnknown, iid *ole.GUID, punk **ole.IUnknown) uint32 {
	pthis := (*stdDispatch)(unsafe.Pointer(this))
	*punk = nil
	if ole.IsEqualGUID(iid, ole.IID_IUnknown) ||
		ole.IsEqualGUID(iid, ole.IID_IDispatch) {
		dispAddRef(this)
		*punk = this
		return ole.S_OK
	}
	if ole.IsEqualGUID(iid, pthis.iid) {
		dispAddRef(this)
		*punk = this
		return ole.S_OK
	}
	return ole.E_NOINTERFACE
}

func dispAddRef(this *ole.IUnknown) int32 {
	pthis := (*stdDispatch)(unsafe.Pointer(this))
	pthis.ref++
	return pthis.ref
}

func dispRelease(this *ole.IUnknown) int32 {
	pthis := (*stdDispatch)(unsafe.Pointer(this))
	pthis.ref--
	return pthis.ref
}

func dispGetIDsOfNames(this *ole.IUnknown, iid *ole.GUID, wnames []*uint16, namelen int, lcid int, pdisp []int32) uintptr {
	pthis := (*stdDispatch)(unsafe.Pointer(this))
	names := make([]string, len(wnames))
	for i := 0; i < len(names); i++ {
		names[i] = ole.LpOleStrToString(wnames[i])
	}
	for n := 0; n < namelen; n++ {
		if id, ok := pthis.funcMap[names[n]]; ok {
			pdisp[n] = id
		}
	}
	return ole.S_OK
}

func dispGetTypeInfoCount(pcount *int) uintptr {
	if pcount != nil {
		*pcount = 0
	}
	return ole.S_OK
}

func dispGetTypeInfo(ptypeif *uintptr) uintptr {
	return ole.E_NOTIMPL
}

func dispInvoke(this *ole.IDispatch, dispid int32, riid *ole.GUID, lcid int, flags int16, dispparams *ole.DISPPARAMS, result *ole.VARIANT, pexcepinfo *ole.EXCEPINFO, nerr *uint) uintptr {
	pthis := (*stdDispatch)(unsafe.Pointer(this))
	found := ""
	for name, id := range pthis.funcMap {
		if id == dispid {
			found = name
		}
	}
	if found != "" {
		rv := reflect.ValueOf(pthis.iface).Elem()
		rm := rv.MethodByName(found)
		rr := rm.Call([]reflect.Value{})
		println(len(rr))
		return ole.S_OK
	}
	return ole.E_NOTIMPL
}

func ConnectObject(disp *ole.IDispatch, iid *ole.GUID, idisp interface{}) (cookie uint32, err error) {
	unknown, err := disp.QueryInterface(ole.IID_IConnectionPointContainer)
	if err != nil {
		return
	}

	container := (*ole.IConnectionPointContainer)(unsafe.Pointer(unknown))
	var point *ole.IConnectionPoint
	err = container.FindConnectionPoint(iid, &point)
	if err != nil {
		return
	}
	if edisp, ok := idisp.(*ole.IUnknown); ok {
		cookie, err = point.Advise(edisp)
		container.Release()
		if err != nil {
			return
		}
	}
	rv := reflect.ValueOf(disp).Elem()
	if rv.Type().Kind() == reflect.Struct {
		dest := &stdDispatch{}
		dest.lpVtbl = &stdDispatchVtbl{}
		dest.lpVtbl.pQueryInterface = syscall.NewCallback(dispQueryInterface)
		dest.lpVtbl.pAddRef = syscall.NewCallback(dispAddRef)
		dest.lpVtbl.pRelease = syscall.NewCallback(dispRelease)
		dest.lpVtbl.pGetTypeInfoCount = syscall.NewCallback(dispGetTypeInfoCount)
		dest.lpVtbl.pGetTypeInfo = syscall.NewCallback(dispGetTypeInfo)
		dest.lpVtbl.pGetIDsOfNames = syscall.NewCallback(dispGetIDsOfNames)
		dest.lpVtbl.pInvoke = syscall.NewCallback(dispInvoke)
		dest.iface = disp
		dest.iid = iid
		cookie, err = point.Advise((*ole.IUnknown)(unsafe.Pointer(dest)))
		container.Release()
		if err != nil {
			point.Release()
			return
		}
	}

	container.Release()

	return 0, ole.NewError(ole.E_INVALIDARG)
}
