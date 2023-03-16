//go:build darwin || linux || freebsd

package platform

import (
	"syscall"
	"unsafe"
)

func isTerminal(fd uintptr) bool {
	var val syscall.Termios
	_, _, err := syscall.Syscall6(syscall.SYS_IOCTL, fd, ioctlReadTermios, uintptr(unsafe.Pointer(&val)), 0, 0, 0)
	return err == 0
}
