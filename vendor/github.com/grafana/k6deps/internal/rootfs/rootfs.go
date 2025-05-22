// Package rootfs implements extensions to go's fs.FS to work around its limitations
package rootfs

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/spf13/afero"
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
	afero.Fs
	root string
}

// NewFromDir create a rootFS from a root directory. The root must be an absolute path
func NewFromDir(root string) (FS, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("%w: %q is not absolute", ErrInvalidPath, root)
	}

	return &rootFS{
		Fs:   afero.NewOsFs(),
		root: root,
	}, nil
}

func (f *rootFS) Root() string {
	return f.root
}

func (f *rootFS) Open(path string) (fs.File, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(f.root, path)
	}
	// check if the path is outside the root
	if !strings.HasPrefix(path, f.root) {
		return nil, &fs.PathError{Path: path, Err: fs.ErrNotExist}
	}

	return f.Fs.Open(filepath.Clean(path))
}

// NewFromFS return a FS from a FS
func NewFromFS(root string, fs afero.Fs) FS {
	return &rootFS{root: root, Fs: fs}
}
