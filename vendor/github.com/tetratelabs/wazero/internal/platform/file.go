package platform

import (
	"io"
	"io/fs"
)

// ReadFile declares all read interfaces defined on os.File used by wazero.
type ReadFile interface {
	fdFile // for the number of links.
	readdirnamesFile
	readdirFile
	fs.ReadDirFile
	io.ReaderAt // for pread
	io.Seeker   // fallback for ReaderAt for embed:fs
}

// File declares all interfaces defined on os.File used by wazero.
type File interface {
	ReadFile
	io.Writer
	io.WriterAt // for pwrite
	chmodFile
	syncFile
	truncateFile
}

// The following interfaces are used until we finalize our own FD-scoped file.
type (
	// PathFile is implemented on files that retain the path to their pre-open.
	PathFile interface {
		Path() string
	}
	// fdFile is implemented by os.File in file_unix.go and file_windows.go
	fdFile interface{ Fd() (fd uintptr) }
	// readdirnamesFile is implemented by os.File in dir.go
	readdirnamesFile interface {
		Readdirnames(n int) (names []string, err error)
	}
	// readdirFile is implemented by os.File in dir.go
	readdirFile interface {
		Readdir(n int) ([]fs.FileInfo, error)
	}
	// chmodFile is implemented by os.File in file_posix.go
	chmodFile interface{ Chmod(fs.FileMode) error }
	// syncFile is implemented by os.File in file_posix.go
	syncFile interface{ Sync() error }
	// truncateFile is implemented by os.File in file_posix.go
	truncateFile interface{ Truncate(size int64) error }
)
