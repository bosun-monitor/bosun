package win32

import "syscall"

var (
	//advapi32 = syscall.NewLazyDLL("advapi32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	//psapi    = syscall.NewLazyDLL("psapi.dll")
	//userenv  = syscall.NewLazyDLL("userenv.dll")
	//user32   = syscall.NewLazyDLL("user32.dll")
)
