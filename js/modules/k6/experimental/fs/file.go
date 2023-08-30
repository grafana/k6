package fs

import (
	"fmt"
	"path/filepath"
)

// file is an abstraction for interacting with files.
type file struct {
	path string

	// data holds a pointer to the file's data
	data []byte

	// isClosed indicates whether the file is closed or not.
	isClosed bool

	// closerFunc holds a function that can be used to close the file.
	//
	// This is used to ensure that whatever operation necessary to cleanly
	// close the file is performed.
	//
	// In our case, the [closerFunc] is provided by the [cache] struct, and
	// it ensures the file's reference count is decreased on close, and that,
	// if relevant (the ref count is zero), the cached file content is released
	// from memory.
	closerFunc func(string) error
}

// Stat returns a FileInfo describing the named file.
func (f *file) stat() *FileInfo {
	filename := filepath.Base(f.path)
	return &FileInfo{Name: filename, Size: len(f.data)}
}

// FileInfo holds information about a file.
type FileInfo struct {
	// Name holds the base name of the file.
	Name string `json:"name"`

	// Size holds the size of the file in bytes.
	Size int `json:"size"`
}

func (f *file) close() error {
	if f.isClosed {
		return nil
	}

	if f.closerFunc == nil {
		return fmt.Errorf("file %s has no closer function", f.path)
	}

	if err := f.closerFunc(f.path); err != nil {
		return err
	}
	f.isClosed = true

	return nil
}
