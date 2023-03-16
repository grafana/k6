package sysfs

import (
	"fmt"
	"io/fs"
	"os"
	pathutil "path"
	"runtime"
	"strings"

	"github.com/tetratelabs/wazero/internal/platform"
)

// Adapt adapts the input to FS unless it is already one. Use NewDirFS instead
// of os.DirFS as it handles interop issues such as windows support.
//
// Note: This performs no flag verification on FS.OpenFile. fs.FS cannot read
// flags as there is no parameter to pass them through with. Moreover, fs.FS
// documentation does not require the file to be present. In summary, we can't
// enforce flag behavior.
func Adapt(fs fs.FS) FS {
	if sys, ok := fs.(FS); ok {
		return sys
	}
	return &adapter{fs: fs}
}

type adapter struct {
	UnimplementedFS
	fs fs.FS
}

// String implements fmt.Stringer
func (a *adapter) String() string {
	return fmt.Sprintf("%v", a.fs)
}

// Open implements the same method as documented on fs.FS
func (a *adapter) Open(name string) (fs.File, error) {
	return a.fs.Open(name)
}

// OpenFile implements FS.OpenFile
func (a *adapter) OpenFile(path string, flag int, perm fs.FileMode) (fs.File, error) {
	path = cleanPath(path)
	f, err := a.fs.Open(path)
	return f, platform.UnwrapOSError(err)
}

// Stat implements FS.Stat
func (a *adapter) Stat(path string, stat *platform.Stat_t) error {
	name := cleanPath(path)
	f, err := a.fs.Open(name)
	if err != nil {
		return platform.UnwrapOSError(err)
	}
	defer f.Close()
	return platform.StatFile(f, stat)
}

func cleanPath(name string) string {
	if len(name) == 0 {
		return name
	}
	// fs.ValidFile cannot be rooted (start with '/')
	cleaned := name
	if name[0] == '/' {
		cleaned = name[1:]
	}
	cleaned = pathutil.Clean(cleaned) // e.g. "sub/." -> "sub"
	return cleaned
}

// fsOpen implements the Open method as documented on fs.FS
func fsOpen(f FS, name string) (fs.File, error) {
	if !fs.ValidPath(name) { // FS.OpenFile has fewer constraints than fs.FS
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	// This isn't a production-grade fs.FS implementation. The only special
	// cases we address here are to pass testfs.TestFS.

	if runtime.GOOS == "windows" {
		switch {
		case strings.Contains(name, "\\"):
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
		}
	}

	if f, err := f.OpenFile(name, os.O_RDONLY, 0); err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: err}
	} else {
		return f, nil
	}
}
