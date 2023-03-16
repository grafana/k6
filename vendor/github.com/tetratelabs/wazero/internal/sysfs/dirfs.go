package sysfs

import (
	"io/fs"
	"os"
	"syscall"

	"github.com/tetratelabs/wazero/internal/platform"
)

func NewDirFS(dir string) FS {
	return &dirFS{
		dir:        dir,
		cleanedDir: ensureTrailingPathSeparator(dir),
	}
}

func ensureTrailingPathSeparator(dir string) string {
	if dir[len(dir)-1] != os.PathSeparator {
		return dir + string(os.PathSeparator)
	}
	return dir
}

type dirFS struct {
	UnimplementedFS
	dir string
	// cleanedDir is for easier OS-specific concatenation, as it always has
	// a trailing path separator.
	cleanedDir string
}

// String implements fmt.Stringer
func (d *dirFS) String() string {
	return d.dir
}

// Open implements the same method as documented on fs.FS
func (d *dirFS) Open(name string) (fs.File, error) {
	return fsOpen(d, name)
}

// OpenFile implements FS.OpenFile
func (d *dirFS) OpenFile(path string, flag int, perm fs.FileMode) (fs.File, error) {
	return platform.OpenFile(d.join(path), flag, perm)
}

// Lstat implements FS.Lstat
func (d *dirFS) Lstat(path string, stat *platform.Stat_t) error {
	return platform.Lstat(d.join(path), stat)
}

// Stat implements FS.Stat
func (d *dirFS) Stat(path string, stat *platform.Stat_t) error {
	return platform.Stat(d.join(path), stat)
}

// Mkdir implements FS.Mkdir
func (d *dirFS) Mkdir(name string, perm fs.FileMode) (err error) {
	err = os.Mkdir(d.join(name), perm)
	if err = platform.UnwrapOSError(err); err == syscall.ENOTDIR {
		err = syscall.ENOENT
	}
	return
}

// Chmod implements FS.Chmod
func (d *dirFS) Chmod(name string, perm fs.FileMode) error {
	err := os.Chmod(d.join(name), perm)
	return platform.UnwrapOSError(err)
}

// Rename implements FS.Rename
func (d *dirFS) Rename(from, to string) error {
	from, to = d.join(from), d.join(to)
	err := platform.Rename(from, to)
	return platform.UnwrapOSError(err)
}

// Readlink implements FS.Readlink
func (d *dirFS) Readlink(path string, buf []byte) (n int, err error) {
	// Note: do not use syscall.Readlink as that causes race on Windows.
	// In any case, syscall.Readlink does almost the same logic as os.Readlink.
	res, err := os.Readlink(d.join(path))
	if err != nil {
		err = platform.UnwrapOSError(err)
		return
	}

	// We need to copy here, but syscall.Readlink does copy internally, so the cost is the same.
	copy(buf, res)
	n = len(res)
	if n > len(buf) {
		n = len(buf)
	}
	platform.SanitizeSeparator(buf[:n])
	return
}

// Link implements FS.Link.
func (d *dirFS) Link(oldName, newName string) error {
	err := os.Link(d.join(oldName), d.join(newName))
	return platform.UnwrapOSError(err)
}

// Rmdir implements FS.Rmdir
func (d *dirFS) Rmdir(name string) error {
	err := syscall.Rmdir(d.join(name))
	return platform.UnwrapOSError(err)
}

// Unlink implements FS.Unlink
func (d *dirFS) Unlink(name string) (err error) {
	return platform.Unlink(d.join(name))
}

// Symlink implements FS.Symlink
func (d *dirFS) Symlink(oldName, link string) (err error) {
	// Note: do not resolve `oldName` relative to this dirFS. The link result is always resolved
	// when dereference the `link` on its usage (e.g. readlink, read, etc).
	// https://github.com/bytecodealliance/cap-std/blob/v1.0.4/cap-std/src/fs/dir.rs#L404-L409
	err = os.Symlink(oldName, d.join(link))
	return platform.UnwrapOSError(err)
}

// Utimes implements FS.Utimes
func (d *dirFS) Utimes(name string, atimeNsec, mtimeNsec int64) error {
	err := syscall.UtimesNano(d.join(name), []syscall.Timespec{
		syscall.NsecToTimespec(atimeNsec),
		syscall.NsecToTimespec(mtimeNsec),
	})
	return platform.UnwrapOSError(err)
}

// Truncate implements FS.Truncate
func (d *dirFS) Truncate(name string, size int64) error {
	// Use os.Truncate as syscall.Truncate doesn't exist on Windows.
	err := os.Truncate(d.join(name), size)
	return platform.UnwrapOSError(err)
}

func (d *dirFS) join(name string) string {
	switch name {
	case "", ".", "/":
		// cleanedDir includes an unnecessary delimiter for the root path.
		return d.cleanedDir[:len(d.cleanedDir)-1]
	}
	// TODO: Enforce similar to safefilepath.FromFS(name), but be careful as
	// relative path inputs are allowed. e.g. dir or name == ../
	return d.cleanedDir + name
}
