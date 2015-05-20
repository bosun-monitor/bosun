// +build !windows

package ole

func queryInterface(unk *IUnknown, iid *GUID) (disp *IDispatch, err error) {
	return nil, NewError(E_NOTIMPL)
}

func addRef(unk *IUnknown) int32 {
	return 0
}

func release(unk *IUnknown) int32 {
	return 0
}
