package platform

import (
	"syscall"
	"unsafe"
)

var procGetConsoleMode = kernel32.NewProc("GetConsoleMode")

func isTerminal(fd uintptr) bool {
	var st uint32
	r, _, e := syscall.Syscall(procGetConsoleMode.Addr(), 2, uintptr(fd), uintptr(unsafe.Pointer(&st)), 0)
	return r != 0 && e == 0
}
