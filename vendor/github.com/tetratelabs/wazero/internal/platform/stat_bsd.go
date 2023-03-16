//go:build (amd64 || arm64) && (darwin || freebsd)

package platform

import (
	"io/fs"
	"os"
	"syscall"
)

func lstat(path string, st *Stat_t) (err error) {
	var t fs.FileInfo
	if t, err = os.Lstat(path); err == nil {
		fillStatFromFileInfo(st, t)
	}
	return
}

func stat(path string, st *Stat_t) (err error) {
	var t fs.FileInfo
	if t, err = os.Stat(path); err == nil {
		fillStatFromFileInfo(st, t)
	}
	return
}

func statFile(f fs.File, st *Stat_t) error {
	return defaultStatFile(f, st)
}

func inoFromFileInfo(_ readdirFile, t fs.FileInfo) (ino uint64, err error) {
	if d, ok := t.Sys().(*syscall.Stat_t); ok {
		ino = d.Ino
	}
	return
}

func fillStatFromFileInfo(st *Stat_t, t fs.FileInfo) {
	if d, ok := t.Sys().(*syscall.Stat_t); ok {
		st.Ino = d.Ino
		st.Dev = uint64(d.Dev)
		st.Mode = t.Mode()
		st.Nlink = uint64(d.Nlink)
		st.Size = d.Size
		atime := d.Atimespec
		st.Atim = atime.Sec*1e9 + atime.Nsec
		mtime := d.Mtimespec
		st.Mtim = mtime.Sec*1e9 + mtime.Nsec
		ctime := d.Ctimespec
		st.Ctim = ctime.Sec*1e9 + ctime.Nsec
	} else {
		fillStatFromDefaultFileInfo(st, t)
	}
}
