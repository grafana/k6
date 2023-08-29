package fs

import (
	"path/filepath"
)

// file is an abstraction for interacting with files.
type file struct {
	path string

	// data holds a pointer to the file's data
	data []byte

	// offset holds the current offset in the file
	offset int
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

// Read reads up to len(into) bytes into the provided byte slice.
//
// It returns the number of bytes read (0 <= n <= len(into)) and any error
// encountered.
//
// If the end of the file has been reached, it returns EOFError.
func (f *file) Read(into []byte) (int, error) {
	start := f.offset
	if start == len(f.data) {
		return 0, newFsError(EOFError, "EOF")
	}

	end := f.offset + len(into)
	if end > len(f.data) {
		end = len(f.data)
	}

	n := copy(into, f.data[start:end])

	f.offset += n

	return n, nil
}
