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

	// offset holds the current offset in the file
	offset int64
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

func (f *file) Read(into []byte) (int, error) {
	start := f.offset
	if start == f.size() {
		return 0, newFsError(EOFError, "EOF")
	}

	end := f.offset + int64(len(into))
	if end > f.size() {
		end = f.size()
	}

	n := copy(into, f.data[start:end])

	f.offset += int64(n)

	return n, nil
}

// Seek sets the offset for the next operation on the file, under the mode given by `whence`.
//
// `offset` indicates the number of bytes to move the offset. Based on
// the `whence` parameter, the offset is set relative to the start,
// current offset or end of the file.
//
// When using SeekModeStart, the offset must be positive.
// Negative offsets are allowed when using `SeekModeCurrent` or `SeekModeEnd`.
func (f *file) Seek(offset int64, whence SeekMode) (int64, error) {
	newOffset := f.offset

	switch whence {
	case SeekModeStart:
		if offset < 0 {
			return 0, newFsError(TypeError, "offset cannot be negative when using SeekModeStart")
		}

		newOffset = offset
	case SeekModeCurrent:
		newOffset += offset
	case SeekModeEnd:
		if offset > 0 {
			return 0, newFsError(TypeError, "offset cannot be positive when using SeekModeEnd")
		}

		newOffset = f.size() + offset
	default:
		return 0, newFsError(TypeError, "invalid seek mode")
	}

	if newOffset < 0 {
		return 0, newFsError(TypeError, "seeking before start of file")
	}

	if newOffset > f.size() {
		return 0, newFsError(TypeError, "seeking beyond end of file")
	}

	// Note that the implementation assumes one `file` instance per file/vu.
	// If that assumption was invalidated, we would need to atomically update
	// the offset instead.
	f.offset = newOffset
	return newOffset, nil
}

// SeekMode is used to specify the seek mode when seeking in a file.
type SeekMode = int

const (
	// SeekModeStart sets the offset relative to the start of the file.
	SeekModeStart SeekMode = iota + 1

	// SeekModeCurrent seeks relative to the current offset.
	SeekModeCurrent

	// SeekModeEnd seeks relative to the end of the file.
	//
	// When using this mode the seek operation will move backwards from
	// the end of the file.
	SeekModeEnd
)

func (f *file) size() int64 {
	return int64(len(f.data))
}
