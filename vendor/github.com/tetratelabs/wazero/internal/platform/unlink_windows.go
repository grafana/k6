//go:build windows

package platform

import (
	"os"
	"syscall"
)

func Unlink(name string) (err error) {
	err = syscall.Unlink(name)
	if err == nil {
		return
	}
	err = UnwrapOSError(err)
	if err == syscall.EPERM {
		lstat, errLstat := os.Lstat(name)
		if errLstat == nil && lstat.Mode()&os.ModeSymlink != 0 {
			err = UnwrapOSError(os.Remove(name))
		} else {
			err = syscall.EISDIR
		}
	}
	return
}
