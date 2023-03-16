package platform

import (
	"io/fs"
	"syscall"
)

// windowsWrappedFile deals with errno portability issues in Windows. This code
// is likely to change as we complete WASI and GOOS=js.
//
// If we don't map to syscall.Errno, wasm will crash in odd way attempting the
// same. This approach is an alternative to making our own fs.File public type.
// We aren't doing that yet, as mapping problems are generally contained to
// Windows. Hence, file is intentionally not exported.
//
// Note: Don't test for this type as it is wrapped when using sysfs.NewReadFS.
type windowsWrappedFile struct {
	File
	path           string
	flag           int
	perm           fs.FileMode
	dirInitialized bool

	fileType *fs.FileMode

	// closed is true when closed was called. This ensures proper syscall.EBADF
	// TODO: extract a base wrapper type to cover all cases.
	closed bool
}

// Path implements PathFile
func (w *windowsWrappedFile) Path() string {
	return w.path
}

// Readdir implements readdirFile.
func (w *windowsWrappedFile) Readdir(n int) (fis []fs.FileInfo, err error) {
	if err = w.requireFile("Readdir", false, true); err != nil {
		return
	} else if err = w.maybeInitDir(); err != nil {
		return
	}

	return w.File.Readdir(n)
}

// ReadDir implements fs.ReadDirFile.
func (w *windowsWrappedFile) ReadDir(n int) (dirents []fs.DirEntry, err error) {
	if err = w.requireFile("ReadDir", false, true); err != nil {
		return
	} else if err = w.maybeInitDir(); err != nil {
		return
	}

	return w.File.ReadDir(n)
}

// Write implements io.Writer
func (w *windowsWrappedFile) Write(p []byte) (n int, err error) {
	if err = w.requireFile("Write", false, false); err != nil {
		return
	}

	return w.File.Write(p)
}

// Close implements io.Closer
func (w *windowsWrappedFile) Close() (err error) {
	if w.closed {
		return
	}

	if err = w.File.Close(); err != nil {
		w.closed = true
	}
	return
}

func (w *windowsWrappedFile) maybeInitDir() error {
	if w.dirInitialized {
		return nil
	}

	// On Windows, once the directory is opened, changes to the directory are
	// not visible on ReadDir on that already-opened file handle.
	//
	// To provide consistent behavior with other platforms, we re-open it.
	if err := w.File.Close(); err != nil {
		return err
	}
	newW, err := openFile(w.path, w.flag, w.perm)
	if err != nil {
		return &fs.PathError{Op: "OpenFile", Path: w.path, Err: err}
	}
	w.File = newW
	w.dirInitialized = true
	return nil
}

// requireFile is used to making syscalls which will fail.
func (w *windowsWrappedFile) requireFile(op string, readOnly, isDir bool) error {
	var ft fs.FileMode
	var err error
	if w.closed {
		err = syscall.EBADF
	} else if readOnly && w.flag&syscall.O_RDONLY == 0 {
		err = syscall.EBADF
	} else if ft, err = w.getFileType(); err == nil && isDir != ft.IsDir() {
		if isDir {
			err = syscall.ENOTDIR
		} else {
			err = syscall.EBADF
		}
	} else if err == nil {
		return nil
	}
	return &fs.PathError{Op: op, Path: w.path, Err: err}
}

// getFileType caches the file type as this cannot change on an open file.
func (w *windowsWrappedFile) getFileType() (fs.FileMode, error) {
	if w.fileType == nil {
		var st Stat_t
		if err := StatFile(w.File, &st); err != nil {
			return 0, nil
		}
		ft := st.Mode & fs.ModeType
		w.fileType = &ft
		return ft, nil
	}
	return *w.fileType, nil
}
