// Package fsext provides extended file system functions
package fsext

import (
	"path/filepath"
)

// JoinFilePath is a wrapper around filepath.Join
// starting go 1.20 on Windows, Clean (that is using inside the
// filepath.Join) does not modify the volume name
// other than to replace occurrences of "/" with `\`.
// that's why we need to add a leading slash to the path
// go.1.19: filepath.Join("\\c:", "test")  // \c:\test
// go.1.20: filepath.Join("\\c:", "test")  // \c:test
func JoinFilePath(b, p string) string {
	return filepath.Join(b, filepath.Clean("/"+p))
}

// Abs returns an absolute representation of path.
//
// this implementation allows k6 to handle absolute paths starting from
// the current drive on windows like `\users\noname\...`. It makes it easier
// to test and is needed for archive execution under windows (it always consider '/...' as an
// absolute path).
//
// If the path is not absolute it will be joined with root
// to turn it into an absolute path. The root path is assumed
// to be a directory.
//
// Because k6 does not interact with the OS itself, but with
// its own virtual file system, it needs to be able to resolve
// the root path of the file system. In most cases this would be
// the bundle or init environment's working directory.
func Abs(root, path string) string {
	if path[0] != '/' && path[0] != '\\' && !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path = filepath.Clean(path)

	if path[0:1] != FilePathSeparator {
		path = FilePathSeparator + path
	}

	return path
}
