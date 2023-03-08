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
