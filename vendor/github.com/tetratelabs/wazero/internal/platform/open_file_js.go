package platform

import (
	"io/fs"
	"os"
)

// See the comments on the same constants in open_file_windows.go
const (
	O_DIRECTORY = 1 << 29
	O_NOFOLLOW  = 1 << 30
)

func OpenFile(name string, flag int, perm fs.FileMode) (f *os.File, err error) {
	flag &= ^(O_DIRECTORY | O_NOFOLLOW) // erase placeholders
	f, err = os.OpenFile(name, flag, perm)
	err = UnwrapOSError(err)
	return
}
