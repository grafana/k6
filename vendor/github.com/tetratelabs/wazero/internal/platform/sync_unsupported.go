//go:build !linux

package platform

import (
	"io/fs"
)

func fdatasync(f fs.File) error {
	// Attempt to sync everything, even if we only need to sync the data.
	if s, ok := f.(syncFile); ok {
		return s.Sync()
	}
	return nil
}
