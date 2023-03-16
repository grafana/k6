package platform

import (
	"fmt"
	"io"
	"io/fs"
	"syscall"
)

// Readdirnames reads the names of the directory associated with file and
// returns a slice of up to n strings in an arbitrary order. This is a stateful
// function, so subsequent calls return any next values.
//
// If n > 0, Readdirnames returns at most n entries or an error.
// If n <= 0, Readdirnames returns all remaining entries or an error.
//
// # Errors
//
// If the error is non-nil, it is a syscall.Errno. io.EOF is not returned
// because implementations return that value inconsistently.
//
// For portability reasons, no error is returned when the file is closed or
// removed while open.
// See https://github.com/ziglang/zig/blob/0.10.1/lib/std/fs.zig#L635-L637
func Readdirnames(f fs.File, n int) (names []string, err error) {
	switch f := f.(type) {
	case readdirnamesFile:
		names, err = f.Readdirnames(n)
		if err = adjustReaddirErr(err); err != nil {
			return
		}
	case fs.ReadDirFile:
		var entries []fs.DirEntry
		entries, err = f.ReadDir(n)
		if err = adjustReaddirErr(err); err != nil {
			return
		}
		names = make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
	default:
		err = syscall.ENOTDIR
	}
	err = UnwrapOSError(err)
	return
}

// Dirent is an entry read from a directory.
//
// This is a portable variant of syscall.Dirent containing fields needed for
// WebAssembly ABI including WASI snapshot-01 and wasi-filesystem. Unlike
// fs.DirEntry, this may include the Ino.
type Dirent struct {
	// ^^ Dirent name matches syscall.Dirent

	// Name is the base name of the directory entry.
	Name string

	// Ino is the file serial number, or zero if not available.
	Ino uint64

	// Type is fs.FileMode masked on fs.ModeType. For example, zero is a
	// regular file, fs.ModeDir is a directory and fs.ModeIrregular is unknown.
	Type fs.FileMode
}

func (d *Dirent) String() string {
	return fmt.Sprintf("name=%s, type=%v, ino=%d", d.Name, d.Type, d.Ino)
}

// IsDir returns true if the Type is fs.ModeDir.
func (d *Dirent) IsDir() bool {
	return d.Type == fs.ModeDir
}

// Readdir reads the contents of the directory associated with file and returns
// a slice of up to n Dirent values in an arbitrary order. This is a stateful
// function, so subsequent calls return any next values.
//
// If n > 0, Readdir returns at most n entries or an error.
// If n <= 0, Readdir returns all remaining entries or an error.
//
// # Errors
//
// If the error is non-nil, it is a syscall.Errno. io.EOF is not returned
// because implementations return that value inconsistently.
//
// For portability reasons, no error is returned when the file is closed or
// removed while open.
// See https://github.com/ziglang/zig/blob/0.10.1/lib/std/fs.zig#L635-L637
func Readdir(f fs.File, n int) (dirents []*Dirent, err error) {
	// ^^ case format is to match POSIX and similar to os.File.Readdir

	switch f := f.(type) {
	case readdirFile:
		var fis []fs.FileInfo
		fis, err = f.Readdir(n)
		if err = adjustReaddirErr(err); err != nil {
			return
		}
		dirents = make([]*Dirent, 0, len(fis))

		// linux/darwin won't have to fan out to lstat, but windows will.
		var ino uint64
		for _, t := range fis {
			if ino, err = inoFromFileInfo(f, t); err != nil {
				return
			}
			dirents = append(dirents, &Dirent{Name: t.Name(), Ino: ino, Type: t.Mode().Type()})
		}
	case fs.ReadDirFile:
		var entries []fs.DirEntry
		entries, err = f.ReadDir(n)
		if err = adjustReaddirErr(err); err != nil {
			return
		}
		dirents = make([]*Dirent, 0, len(entries))
		for _, e := range entries {
			// By default, we don't attempt to read inode data
			dirents = append(dirents, &Dirent{Name: e.Name(), Type: e.Type()})
		}
	default:
		err = syscall.ENOTDIR
	}
	return
}

func adjustReaddirErr(err error) error {
	if err == io.EOF {
		return nil // e.g. Readdir on darwin returns io.EOF, but linux doesn't.
	} else if err = UnwrapOSError(err); err != nil {
		// Ignore errors when the file was closed or removed.
		switch err {
		case syscall.EIO, syscall.EBADF: // closed while open
			return nil
		case syscall.ENOENT: // Linux error when removed while open
			return nil
		}
		return err
	}
	return err
}
