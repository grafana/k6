//go:build (amd64 || arm64) && windows

package platform

import (
	"io/fs"
	"path"
	"syscall"
)

func lstat(path string, st *Stat_t) error {
	attrs := uint32(syscall.FILE_FLAG_BACKUP_SEMANTICS)
	// Use FILE_FLAG_OPEN_REPARSE_POINT, otherwise CreateFile will follow symlink.
	// See https://docs.microsoft.com/en-us/windows/desktop/FileIO/symbolic-link-effects-on-file-systems-functions#createfile-and-createfiletransacted
	attrs |= syscall.FILE_FLAG_OPEN_REPARSE_POINT
	return statPath(attrs, path, st)
}

func stat(path string, st *Stat_t) error {
	attrs := uint32(syscall.FILE_FLAG_BACKUP_SEMANTICS)
	return statPath(attrs, path, st)
}

func statPath(createFileAttrs uint32, path string, st *Stat_t) (err error) {
	if len(path) == 0 {
		return syscall.ENOENT
	}
	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return syscall.EINVAL
	}

	// open the file handle
	h, err := syscall.CreateFile(pathp, 0, 0, nil,
		syscall.OPEN_EXISTING, createFileAttrs, 0)
	if err != nil {
		// To match expectations of WASI, e.g. TinyGo TestStatBadDir, return
		// ENOENT, not ENOTDIR.
		if err == syscall.ENOTDIR {
			err = syscall.ENOENT
		}
		return err
	}
	defer syscall.CloseHandle(h)

	return statHandle(h, st)
}

func statFile(f fs.File, st *Stat_t) (err error) {
	if of, ok := f.(fdFile); ok {
		// Attempt to get the stat by handle, which works for normal files
		err = statHandle(syscall.Handle(of.Fd()), st)

		// ERROR_INVALID_HANDLE happens before Go 1.20. Don't fail as we only
		// use that approach to fill in inode data, which is not critical.
		if err != ERROR_INVALID_HANDLE {
			return
		}
	}
	return defaultStatFile(f, st)
}

// inoFromFileInfo uses stat to get the inode information of the file.
func inoFromFileInfo(f readdirFile, t fs.FileInfo) (ino uint64, err error) {
	if pf, ok := f.(PathFile); ok {
		var st Stat_t
		inoPath := path.Clean(path.Join(pf.Path(), t.Name()))
		if err = lstat(inoPath, &st); err == nil {
			ino = st.Ino
		}
	}
	return // not in Win32FileAttributeData
}

func fillStatFromFileInfo(st *Stat_t, t fs.FileInfo) {
	if d, ok := t.Sys().(*syscall.Win32FileAttributeData); ok {
		st.Ino = 0 // not in Win32FileAttributeData
		st.Dev = 0 // not in Win32FileAttributeData
		st.Mode = t.Mode()
		st.Nlink = 1 // not in Win32FileAttributeData
		st.Size = t.Size()
		st.Atim = d.LastAccessTime.Nanoseconds()
		st.Mtim = d.LastWriteTime.Nanoseconds()
		st.Ctim = d.CreationTime.Nanoseconds()
	} else {
		fillStatFromDefaultFileInfo(st, t)
	}
}

func statHandle(h syscall.Handle, st *Stat_t) (err error) {
	winFt, err := syscall.GetFileType(h)
	if err != nil {
		return err
	}

	var fi syscall.ByHandleFileInformation
	if err = syscall.GetFileInformationByHandle(h, &fi); err != nil {
		return err
	}

	var m fs.FileMode
	if fi.FileAttributes&syscall.FILE_ATTRIBUTE_READONLY != 0 {
		m |= 0o444
	} else {
		m |= 0o666
	}

	switch { // check whether this is a symlink first
	case fi.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0:
		m |= fs.ModeSymlink
	case winFt == syscall.FILE_TYPE_PIPE:
		m |= fs.ModeNamedPipe
	case winFt == syscall.FILE_TYPE_CHAR:
		m |= fs.ModeDevice | fs.ModeCharDevice
	case fi.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0:
		m |= fs.ModeDir | 0o111 // e.g. 0o444 -> 0o555
	}

	// FileIndex{High,Low} can be combined and used as a unique identifier like inode.
	// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/ns-fileapi-by_handle_file_information
	st.Dev = uint64(fi.VolumeSerialNumber)
	st.Ino = (uint64(fi.FileIndexHigh) << 32) | uint64(fi.FileIndexLow)
	st.Mode = m
	st.Nlink = uint64(fi.NumberOfLinks)
	st.Size = int64(fi.FileSizeHigh)<<32 + int64(fi.FileSizeLow)
	st.Atim = fi.LastAccessTime.Nanoseconds()
	st.Mtim = fi.LastWriteTime.Nanoseconds()
	st.Ctim = fi.CreationTime.Nanoseconds()
	return
}
