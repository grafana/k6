//go:build (!((amd64 || arm64 || riscv64) && linux) && !((amd64 || arm64) && (darwin || freebsd)) && !((amd64 || arm64) && windows)) || js

package platform

import (
	"io/fs"
	"os"
)

func lstat(path string, st *Stat_t) (err error) {
	t, err := os.Lstat(path)
	if err = UnwrapOSError(err); err == nil {
		fillStatFromFileInfo(st, t)
	}
	return
}

func stat(path string, st *Stat_t) (err error) {
	t, err := os.Stat(path)
	if err = UnwrapOSError(err); err == nil {
		fillStatFromFileInfo(st, t)
	}
	return
}

func statFile(f fs.File, st *Stat_t) error {
	return defaultStatFile(f, st)
}

func inoFromFileInfo(readdirFile, fs.FileInfo) (ino uint64, err error) {
	return
}

func fillStatFromFileInfo(st *Stat_t, t fs.FileInfo) {
	fillStatFromDefaultFileInfo(st, t)
}

func fillStatFromOpenFile(st *Stat_t, fd uintptr, t os.FileInfo) (err error) {
	fillStatFromFileInfo(st, t)
	return
}
