package sysfs

import (
	"io/fs"
	"syscall"

	"github.com/tetratelabs/wazero/internal/platform"
)

// UnimplementedFS is an FS that returns syscall.ENOSYS for all functions,
// This should be embedded to have forward compatible implementations.
type UnimplementedFS struct{}

// String implements fmt.Stringer
func (UnimplementedFS) String() string {
	return "Unimplemented:/"
}

// Open implements the same method as documented on fs.FS
func (UnimplementedFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: syscall.ENOSYS}
}

// OpenFile implements FS.OpenFile
func (UnimplementedFS) OpenFile(path string, flag int, perm fs.FileMode) (fs.File, error) {
	return nil, syscall.ENOSYS
}

// Lstat implements FS.Lstat
func (UnimplementedFS) Lstat(path string, stat *platform.Stat_t) error {
	return syscall.ENOSYS
}

// Stat implements FS.Stat
func (UnimplementedFS) Stat(path string, stat *platform.Stat_t) error {
	return syscall.ENOSYS
}

// Mkdir implements FS.Mkdir
func (UnimplementedFS) Mkdir(path string, perm fs.FileMode) error {
	return syscall.ENOSYS
}

// Chmod implements FS.Chmod
func (UnimplementedFS) Chmod(path string, perm fs.FileMode) error {
	return syscall.ENOSYS
}

// Rename implements FS.Rename
func (UnimplementedFS) Rename(from, to string) error {
	return syscall.ENOSYS
}

// Rmdir implements FS.Rmdir
func (UnimplementedFS) Rmdir(path string) error {
	return syscall.ENOSYS
}

// Readlink implements FS.Readlink
func (UnimplementedFS) Readlink(string, []byte) (int, error) {
	return 0, syscall.ENOSYS
}

// Link implements FS.Link
func (UnimplementedFS) Link(_, _ string) error {
	return syscall.ENOSYS
}

// Symlink implements FS.Symlink
func (UnimplementedFS) Symlink(_, _ string) error {
	return syscall.ENOSYS
}

// Unlink implements FS.Unlink
func (UnimplementedFS) Unlink(path string) error {
	return syscall.ENOSYS
}

// Utimes implements FS.Utimes
func (UnimplementedFS) Utimes(path string, atimeNsec, mtimeNsec int64) error {
	return syscall.ENOSYS
}

// Truncate implements FS.Truncate
func (UnimplementedFS) Truncate(string, int64) error {
	return syscall.ENOSYS
}
