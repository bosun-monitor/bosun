package win32

type JobObjectExtendedLimitInformation struct {
	BasicLimitInformation JobObjectBasicLimitInformation
	IoInfo                IoCounters
	ProcessMemoryLimit    uintptr // SIZE_T
	JobMemoryLimit        uintptr // SIZE_T
	PeakProcessMemoryUsed uintptr // SIZE_T
	PeakJobMemoryUsed     uintptr // SIZE_T
}
