// Package rootfs implements extensions to go's fs.FS to work around its limitations
package rootfs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrInvalidPath = errors.New("invalid path") //nolint:revive

// FS defines an interface that extends go's fs.FS with a mechanism for working with absolute paths
type FS interface {
	// Opens a File given its path. Path can be absolute or relative.
	// If path is relative, it is joined to the root to get the effective path.
	// The path must be  within the FS's root directory
	Open(path string) (fs.File, error)
	// returns FS's root dir
	Root() string
}

type rootFS struct {
	fs   fs.FS
	root string
}

// NewFromDir create a rootFS from a root directory. The root must be an absolute path
func NewFromDir(root string) (FS, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("%w: %q is not absolute", ErrInvalidPath, root)
	}

	return &rootFS{
		fs:   os.DirFS(root), //nolint:forbidigo
		root: root,
	}, nil
}

func (f *rootFS) Root() string {
	return f.root
}

func (f *rootFS) Open(filePath string) (fs.File, error) {
	return f.fs.Open(filePath)
}

// NewFromFS return a FS from a FS
func NewFromFS(root string, fs fs.FS) FS {
	return &rootFS{root: root, fs: fs}
}
