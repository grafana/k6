/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

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
	originalName string
}

// NewChangePathFs return a ChangePathFs where all paths will be change with the provided funcs
func NewChangePathFs(source afero.Fs, fn ChangePathFunc) *ChangePathFs {
	return &ChangePathFs{source: source, fn: fn}
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
	return f.originalName
}

//Chtimes changes the access and modification times of the named file
func (b *ChangePathFs) Chtimes(name string, atime, mtime time.Time) (err error) {
	var newName string
	if newName, err = b.fn(name); err != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: err}
	}
	return b.source.Chtimes(newName, atime, mtime)
}

// Chmod changes the mode of the named file to mode.
func (b *ChangePathFs) Chmod(name string, mode os.FileMode) (err error) {
	var newName string
	if newName, err = b.fn(name); err != nil {
		return &os.PathError{Op: "chmod", Path: name, Err: err}
	}
	return b.source.Chmod(newName, mode)
}

// Name return the name of this FileSystem
func (b *ChangePathFs) Name() string {
	return "ChangePathFs"
}

// Stat returns a FileInfo describing the named file, or an error, if any
// happens.
func (b *ChangePathFs) Stat(name string) (fi os.FileInfo, err error) {
	var newName string
	if newName, err = b.fn(name); err != nil {
		return nil, &os.PathError{Op: "stat", Path: name, Err: err}
	}
	return b.source.Stat(newName)
}

// Rename renames a file.
func (b *ChangePathFs) Rename(oldName, newName string) (err error) {
	var newOldName, newNewName string
	if newOldName, err = b.fn(oldName); err != nil {
		return &os.PathError{Op: "rename", Path: oldName, Err: err}
	}
	if newNewName, err = b.fn(newName); err != nil {
		return &os.PathError{Op: "rename", Path: newName, Err: err}
	}
	return b.source.Rename(newOldName, newNewName)
}

// RemoveAll removes a directory path and any children it contains. It
// does not fail if the path does not exist (return nil).
func (b *ChangePathFs) RemoveAll(name string) (err error) {
	var newName string
	if newName, err = b.fn(name); err != nil {
		return &os.PathError{Op: "remove_all", Path: name, Err: err}
	}
	return b.source.RemoveAll(newName)
}

// Remove removes a file identified by name, returning an error, if any
// happens.
func (b *ChangePathFs) Remove(name string) (err error) {
	var newName string
	if newName, err = b.fn(name); err != nil {
		return &os.PathError{Op: "remove", Path: name, Err: err}
	}
	return b.source.Remove(newName)
}

// OpenFile opens a file using the given flags and the given mode.
func (b *ChangePathFs) OpenFile(name string, flag int, mode os.FileMode) (f afero.File, err error) {
	var newName string
	if newName, err = b.fn(name); err != nil {
		return nil, &os.PathError{Op: "openfile", Path: name, Err: err}
	}
	sourcef, err := b.source.OpenFile(newName, flag, mode)
	if err != nil {
		return nil, err
	}
	return &ChangePathFile{File: sourcef, originalName: name}, nil
}

// Open opens a file, returning it or an error, if any happens.
func (b *ChangePathFs) Open(name string) (f afero.File, err error) {
	var newName string
	if newName, err = b.fn(name); err != nil {
		return nil, &os.PathError{Op: "open", Path: name, Err: err}
	}
	sourcef, err := b.source.Open(newName)
	if err != nil {
		return nil, err
	}
	return &ChangePathFile{File: sourcef, originalName: name}, nil
}

// Mkdir creates a directory in the filesystem, return an error if any
// happens.
func (b *ChangePathFs) Mkdir(name string, mode os.FileMode) (err error) {
	var newName string
	if newName, err = b.fn(name); err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}
	return b.source.Mkdir(newName, mode)
}

// MkdirAll creates a directory path and all parents that does not exist
// yet.
func (b *ChangePathFs) MkdirAll(name string, mode os.FileMode) (err error) {
	var newName string
	if newName, err = b.fn(name); err != nil {
		return &os.PathError{Op: "mkdir", Path: name, Err: err}
	}
	return b.source.MkdirAll(newName, mode)
}

// Create creates a file in the filesystem, returning the file and an
// error, if any happens
func (b *ChangePathFs) Create(name string) (f afero.File, err error) {
	var newName string
	if newName, err = b.fn(name); err != nil {
		return nil, &os.PathError{Op: "create", Path: name, Err: err}
	}
	sourcef, err := b.source.Create(newName)
	if err != nil {
		return nil, err
	}
	return &ChangePathFile{File: sourcef, originalName: name}, nil
}

// LstatIfPossible implements the afero.Lstater interface
func (b *ChangePathFs) LstatIfPossible(name string) (os.FileInfo, bool, error) {
	var newName string
	newName, err := b.fn(name)
	if err != nil {
		return nil, false, &os.PathError{Op: "lstat", Path: name, Err: err}
	}
	if lstater, ok := b.source.(afero.Lstater); ok {
		return lstater.LstatIfPossible(newName)
	}
	fi, err := b.source.Stat(newName)
	return fi, false, err
}
