//go:build !windows

package platform

import "syscall"

func Unlink(name string) error {
	err := syscall.Unlink(name)
	if err = UnwrapOSError(err); err == syscall.EPERM {
		err = syscall.EISDIR
	}
	return err
}
