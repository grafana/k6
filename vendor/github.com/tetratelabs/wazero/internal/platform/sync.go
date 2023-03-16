package platform

import "io/fs"

// Fdatasync is like syscall.Fdatasync except that's only defined in linux.
//
// Note: This returns with no error instead of syscall.ENOSYS when
// unimplemented. This prevents fake filesystems from erring.
func Fdatasync(f fs.File) (err error) {
	return fdatasync(f)
}
