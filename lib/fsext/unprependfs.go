package fsext

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"
)

var _ afero.Lstater = (*ChangePathFs)(nil)

// ChangePathFs is a filesystem that wraps another afero.Fs and changes all given paths from all
// file and directory names, with a function, before calling the same method on the wrapped afero.Fs.
// Heavily based on afero.BasePathFs
type ChangePathFs struct {
	source afero.Fs
	fn     ChangePathFunc
}

// ChangePathFile is a file from ChangePathFs
type ChangePathFile struct {
	afero.File
	fn ChangePathFunc
}

// ChangePathFunc is the function that will be called by ChangePathFs to change the path
type ChangePathFunc func(name string) (path string, err error)

// NewTrimFilePathSeparatorFs is ChangePathFs that trims a Afero.FilePathSeparator from all paths
// Heavily based on afero.BasePathFs
func NewTrimFilePathSeparatorFs(source afero.Fs) *ChangePathFs {
	return &ChangePathFs{source: source, fn: ChangePathFunc(func(name string) (path string, err error) {
		if !strings.HasPrefix(name, afero.FilePathSeparator) {
			return name, os.ErrNotExist
		}

		return filepath.Clean(strings.TrimPrefix(name, afero.FilePathSeparator)), nil

	})}
}

// Name Returns the name of the file
func (f *ChangePathFile) Name() string {
	// error shouldn't be possible
	name, _ := f.fn(f.File.Name())
	return name
}

//Chtimes changes the access and modification times of the named file
func (b *ChangePathFs) Chtimes(name string, atime, mtime time.Time) (err error) {
	if name, err = b.fn(name); err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: err}
	}
	return b.source.Chtimes(name, atime, mtime)
}

// Chmod changes the mode of the named file to mode.
func (b *ChangePathFs) Chmod(name string, mode os.FileMode) (err error) {
	if name, err = b.fn(name); err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: err}
	}
	return b.source.Chmod(name, mode)
}

// Name return the name of this FileSystem
func (b *ChangePathFs) Name() string {
	return "ChangePathFs"
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (b *ChangePathFs) Stat(name string) (fi os.FileInfo, err error) {
	if name, err = b.fn(name); err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: err}
	}
	return b.source.Stat(name)
}

// Rename renames a file.
func (b *ChangePathFs) Rename(oldname, newname string) (err error) {
	if oldname, err = b.fn(oldname); err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: err}
	}
	if newname, err = b.fn(newname); err != nil {
		return &os.PathError{Op: "rename", Path: newname, Err: err}
	}
	return b.source.Rename(oldname, newname)
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (b *ChangePathFs) RemoveAll(name string) (err error) {
	if name, err = b.fn(name); err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: err}
	}
	return b.source.RemoveAll(name)
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (b *ChangePathFs) Remove(name string) (err error) {
	if name, err = b.fn(name); err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: err}
	}
	return b.source.Remove(name)
}

// OpenFile opens a file using the given flags and the given mode.
func (b *ChangePathFs) OpenFile(name string, flag int, mode os.FileMode) (f afero.File, err error) {
	if name, err = b.fn(name); err != nil {
		return nil, &os.PathError{Op: "openfile", Path: name, Err: err}
	}
	sourcef, err := b.source.OpenFile(name, flag, mode)
	if err != nil {
		return nil, err
	}
	return &ChangePathFile{File: sourcef, fn: b.fn}, nil
}

// Open opens a file, returning it or an error, if any happens.
func (b *ChangePathFs) Open(name string) (f afero.File, err error) {
	if name, err = b.fn(name); err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	sourcef, err := b.source.Open(name)
	if err != nil {
		return nil, err
	}
	return &ChangePathFile{File: sourcef, fn: b.fn}, nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (b *ChangePathFs) Mkdir(name string, mode os.FileMode) (err error) {
	if name, err = b.fn(name); err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}
	return b.source.Mkdir(name, mode)
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (b *ChangePathFs) MkdirAll(name string, mode os.FileMode) (err error) {
	if name, err = b.fn(name); err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}
	return b.source.MkdirAll(name, mode)
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens
func (b *ChangePathFs) Create(name string) (f afero.File, err error) {
	if name, err = b.fn(name); err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: err}
	}
	sourcef, err := b.source.Create(name)
	if err != nil {
		return nil, err
	}
	return &ChangePathFile{File: sourcef, fn: b.fn}, nil
}

// LstatIfPossible implements the afero.Lstater interface
func (b *ChangePathFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	name, err := b.fn(name)
	if err != nil {
		return nil, false, &os.PathError{Op: "lstat", Path: name, Err: err}
	}
	if lstater, ok := b.source.(afero.Lstater); ok {
		return lstater.LstatIfPossible(name)
	}
	fi, err := b.source.Stat(name)
	return fi, false, err
}
