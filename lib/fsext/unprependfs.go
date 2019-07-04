package fsext

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/afero"
)

var _ afero.Lstater = (*UnprependPathFs)(nil)

// UnprependPathFs is a filesystem that wraps another afero.Fs and unprepend a given path from all
// file and directory names before calling the same method on the wrapped afero.Fs.
// Heavily based on afero.BasePathFs
type UnprependPathFs struct {
	source afero.Fs
	path   string
}

// UnprependPathFile is a file from UnprependPathFs
type UnprependPathFile struct {
	afero.File
	path string
}

// Name Returns the name of the file
func (f *UnprependPathFile) Name() string {
	sourcename := f.File.Name()
	return strings.TrimPrefix(sourcename, filepath.Clean(f.path))
}

// NewUnprependPathFs returns a new UnprependPathFs that will unprepend
func NewUnprependPathFs(source afero.Fs, path string) *UnprependPathFs {
	return &UnprependPathFs{source: source, path: path}
}

func (b *UnprependPathFs) realPath(name string) (path string, err error) {
	if !strings.HasPrefix(name, b.path) {
		return name, os.ErrNotExist
	}

	return filepath.Clean(strings.TrimPrefix(name, b.path)), nil
}

//Chtimes changes the access and modification times of the named file
func (b *UnprependPathFs) Chtimes(name string, atime, mtime time.Time) (err error) {
	if name, err = b.realPath(name); err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: err}
	}
	return b.source.Chtimes(name, atime, mtime)
}

// Chmod changes the mode of the named file to mode.
func (b *UnprependPathFs) Chmod(name string, mode os.FileMode) (err error) {
	if name, err = b.realPath(name); err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: err}
	}
	return b.source.Chmod(name, mode)
}

// Name return the name of this FileSystem
func (b *UnprependPathFs) Name() string {
	return "UnprependPathFs"
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (b *UnprependPathFs) Stat(name string) (fi os.FileInfo, err error) {
	if name, err = b.realPath(name); err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: err}
	}
	return b.source.Stat(name)
}

// Rename renames a file.
func (b *UnprependPathFs) Rename(oldname, newname string) (err error) {
	if oldname, err = b.realPath(oldname); err != nil {
		return &os.PathError{Op: "rename", Path: oldname, Err: err}
	}
	if newname, err = b.realPath(newname); err != nil {
		return &os.PathError{Op: "rename", Path: newname, Err: err}
	}
	return b.source.Rename(oldname, newname)
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (b *UnprependPathFs) RemoveAll(name string) (err error) {
	if name, err = b.realPath(name); err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: err}
	}
	return b.source.RemoveAll(name)
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (b *UnprependPathFs) Remove(name string) (err error) {
	if name, err = b.realPath(name); err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: err}
	}
	return b.source.Remove(name)
}

// OpenFile opens a file using the given flags and the given mode.
func (b *UnprependPathFs) OpenFile(name string, flag int, mode os.FileMode) (f afero.File, err error) {
	if name, err = b.realPath(name); err != nil {
		return nil, &os.PathError{Op: "openfile", Path: name, Err: err}
	}
	sourcef, err := b.source.OpenFile(name, flag, mode)
	if err != nil {
		return nil, err
	}
	return &UnprependPathFile{sourcef, b.path}, nil
}

// Open opens a file, returning it or an error, if any happens.
func (b *UnprependPathFs) Open(name string) (f afero.File, err error) {
	if name, err = b.realPath(name); err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	sourcef, err := b.source.Open(name)
	if err != nil {
		return nil, err
	}
	return &UnprependPathFile{File: sourcef, path: b.path}, nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (b *UnprependPathFs) Mkdir(name string, mode os.FileMode) (err error) {
	if name, err = b.realPath(name); err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}
	return b.source.Mkdir(name, mode)
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (b *UnprependPathFs) MkdirAll(name string, mode os.FileMode) (err error) {
	if name, err = b.realPath(name); err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}
	return b.source.MkdirAll(name, mode)
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens
func (b *UnprependPathFs) Create(name string) (f afero.File, err error) {
	if name, err = b.realPath(name); err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: err}
	}
	sourcef, err := b.source.Create(name)
	if err != nil {
		return nil, err
	}
	return &UnprependPathFile{File: sourcef, path: b.path}, nil
}

// LstatIfPossible implements the afero.Lstater interface
func (b *UnprependPathFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	name, err := b.realPath(name)
	if err != nil {
		return nil, false, &os.PathError{Op: "lstat", Path: name, Err: err}
	}
	if lstater, ok := b.source.(afero.Lstater); ok {
		return lstater.LstatIfPossible(name)
	}
	fi, err := b.source.Stat(name)
	return fi, false, err
}
