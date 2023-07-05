package fs

import (
	"path/filepath"
)

// file is an abstraction for interacting with files.
type file struct {
	// name holds the name of the file, as presented to [Open].
	path string

	// data holds a pointer to the file's data
	data []byte
}

// Stat returns a FileInfo describing the named file.
func (f *file) Stat() *FileInfo {
	filename := filepath.Base(f.path)
	return &FileInfo{Name: filename, Size: len(f.data)}
}

// FileInfo holds information about a file.
//
// It is a wrapper around the [fileInfo] struct, which is meant to be directly
// exposed to the JS runtime.
type FileInfo struct {
	// Name holds the base name of the file.
	Name string `json:"name"`

	// Name holds the length in bytes of the file.
	Size int `json:"size"`
}
