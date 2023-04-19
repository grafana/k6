package fsext

import (
	"io/fs"

	"github.com/spf13/afero"
)

// TODO: reimplement this while tackling https://github.com/grafana/k6/issues/1079

// Fs represents a file system
type Fs = afero.Fs

// FilePathSeparator is the FilePathSeparator to be used within a file system
const FilePathSeparator = afero.FilePathSeparator

// NewMemMapFs returns a Fs that is in memory
func NewMemMapFs() Fs {
	return afero.NewMemMapFs()
}

// NewReadOnlyFs returns a Fs wrapping the provided one and returning error on any not read operation.
func NewReadOnlyFs(fs Fs) Fs {
	return afero.NewReadOnlyFs(fs)
}

// WriteFile writes the provided data to the provided fs in the provided filename
func WriteFile(fs Fs, filename string, data []byte, perm fs.FileMode) error {
	return afero.WriteFile(fs, filename, data, perm)
}

// ReadFile reads the whole file from the filesystem
func ReadFile(fs Fs, filename string) ([]byte, error) {
	return afero.ReadFile(fs, filename)
}

// ReadDir reads the info for each file in the provided dirname
func ReadDir(fs Fs, dirname string) ([]fs.FileInfo, error) {
	return afero.ReadDir(fs, dirname)
}

// NewOsFs returns a new wrapps os.Fs
func NewOsFs() Fs {
	return afero.NewOsFs()
}

// Exists checks if the provided path exists on the filesystem
func Exists(fs Fs, path string) (bool, error) {
	return afero.Exists(fs, path)
}

// IsDir checks if the provided path is a directory
func IsDir(fs Fs, path string) (bool, error) {
	// TODO move fix here
	return afero.IsDir(fs, path)
}
