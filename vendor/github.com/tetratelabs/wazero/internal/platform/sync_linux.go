//go:build linux

package platform

import (
	"io/fs"
	"syscall"
)

func fdatasync(f fs.File) (err error) {
	if fd, ok := f.(fdFile); ok {
		return syscall.Fdatasync(int(fd.Fd()))
	}

	// Attempt to sync everything, even if we only need to sync the data.
	if s, ok := f.(syncFile); ok {
		err = s.Sync()
	}
	return
}
