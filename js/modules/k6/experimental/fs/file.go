package fs

import (
	"io"
	"path/filepath"
	"sync/atomic"
)

// file is an abstraction for interacting with files.
type file struct {
	path string `js:"path"`

	// data holds a pointer to the file's data
	data []byte `js:"data"`

	// offset holds the current offset in the file
	//
	// TODO: using an atomic here does not guarantee ordering of reads and seeks, and leaves
	// the behavior not strictly defined. This is something we might want to address in the future, and
	// is tracked as part of #3433.
	offset atomic.Int64 `js:"offset"`
}

// Stat returns a FileInfo describing the named file.
func (f *file) Stat() *FileInfo {
	filename := filepath.Base(f.path)
	return &FileInfo{Name: filename, Size: f.size()}
}

// Ensure that `file` implements the Stater interface.
var _ Stater = (*file)(nil)

// FileInfo holds information about a file.
type FileInfo struct {
	// Name holds the base name of the file.
	Name string `json:"name"`

	// Size holds the size of the file in bytes.
	Size int64 `json:"size"`
}

// Read reads up to len(into) bytes into the provided byte slice.
//
// It returns the number of bytes read (0 <= n <= len(into)) and any error
// encountered.
//
// If the end of the file has been reached, it returns EOFError.
func (f *file) Read(into []byte) (n int, err error) {
	currentOffset := f.offset.Load()
	fileSize := f.size()

	// Check if we have reached the end of the file
	if currentOffset == fileSize {
		return 0, io.EOF
	}

	// Calculate the effective new offset
	targetOffset := currentOffset + int64(len(into))
	newOffset := min(targetOffset, fileSize)

	// Read the data into the provided slice, and update
	// the offset accordingly
	n = copy(into, f.data[currentOffset:newOffset])
	f.offset.Store(newOffset)

	// If we've reached or surpassed the end, set the error to EOF
	if targetOffset > fileSize {
		err = io.EOF
	}

	return n, err
}

// Ensure that `file` implements the io.Reader interface.
var _ io.Reader = (*file)(nil)

// Seek sets the offset for the next operation on the file, under the mode given by `whence`.
//
// `offset` indicates the number of bytes to move the offset. Based on
// the `whence` parameter, the offset is set relative to the start,
// current offset or end of the file.
//
// When using SeekModeStart, the offset must be positive.
// Negative offsets are allowed when using `SeekModeCurrent` or `SeekModeEnd`.
func (f *file) Seek(offset int64, whence SeekMode) (int64, error) {
	startingOffset := f.offset.Load()

	newOffset := startingOffset
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

		newOffset = (f.size() - 1) + offset
	default:
		return 0, newFsError(TypeError, "invalid seek mode")
	}

	if newOffset < 0 {
		return 0, newFsError(TypeError, "seeking before start of file")
	}

	if newOffset > f.size() {
		return 0, newFsError(TypeError, "seeking beyond end of file")
	}

	// Update the file instance's offset to the new selected position
	f.offset.Store(newOffset)

	return newOffset, nil
}

var _ io.Seeker = (*file)(nil)

// SeekMode is used to specify the seek mode when seeking in a file.
type SeekMode = int

const (
	// SeekModeStart sets the offset relative to the start of the file.
	SeekModeStart SeekMode = 0

	// SeekModeCurrent seeks relative to the current offset.
	SeekModeCurrent = 1

	// SeekModeEnd seeks relative to the end of the file.
	//
	// When using this mode the seek operation will move backwards from
	// the end of the file.
	SeekModeEnd = 2
)

func (f *file) size() int64 {
	return int64(len(f.data))
}
