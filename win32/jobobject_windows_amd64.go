package win32

import (
	"os"
	"unsafe"
)

type JobObjectExtendedLimitInformation struct {
	BasicLimitInformation JobObjectBasicLimitInformation
	IoInfo                IoCounters
	ProcessMemoryLimit    uintptr // SIZE_T
	JobMemoryLimit        uintptr // SIZE_T
	PeakProcessMemoryUsed uintptr // SIZE_T
	PeakJobMemoryUsed     uintptr // SIZE_T
}

func VerSetConditionMask(lConditionMask uint64, typeBitMask uint32, conditionMask uint8) uint64 {
	r1, _, _ := procVerSetConditionMask.Call(uintptr(lConditionMask), uintptr(typeBitMask), uintptr(conditionMask))
	return uint64(r1)
}

func VerifyWindowsInfoW(vi OSVersionInfoEx, typeMask uint32, conditionMask uint64) (bool, error) {
	vi.OSVersionInfoSize = uint32(unsafe.Sizeof(vi))

	r1, _, e1 := procVerifyVersionInfoW.Call(uintptr(unsafe.Pointer(&vi)), uintptr(typeMask), uintptr(conditionMask))
	if r1 != 0 {
		return true, nil
	}
	if r1 == 0 && e1 == ERROR_OLD_WIN_VERSION {
		return false, nil
	}
	return false, os.NewSyscallError("VerifyVersionInfoW", e1)
}
