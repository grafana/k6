//go:build !windows && !js

package platform

import (
	"io/fs"
	"os"
	"syscall"
)

// Simple aliases to constants in the syscall package for portability with
// platforms which do not have them (e.g. windows)
const (
	O_DIRECTORY = syscall.O_DIRECTORY
	O_NOFOLLOW  = syscall.O_NOFOLLOW
)

// OpenFile is like os.OpenFile except it returns syscall.Errno
func OpenFile(name string, flag int, perm fs.FileMode) (f *os.File, err error) {
	f, err = os.OpenFile(name, flag, perm)
	err = UnwrapOSError(err)
	return
}
