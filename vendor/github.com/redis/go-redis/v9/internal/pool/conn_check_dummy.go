//go:build !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd && !solaris && !illumos

package pool

import "syscall"

func connCheck(_ syscall.Conn) error {
	return nil
}
