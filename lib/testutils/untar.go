// Package testutils contains the test utilities and helpers inside the k6 project.
package testutils

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"syscall"
	"testing"

	"go.k6.io/k6/lib/fsext"
)

// Untar a simple test helper that untars a `fileName` file to a `destination` path
func Untar(t *testing.T, fileSystem fsext.Fs, fileName string, destination string) error {
	t.Helper()

	archiveFile, err := fsext.ReadFile(fileSystem, fileName)
	if err != nil {
		return err
	}

	reader := bytes.NewBuffer(archiveFile)

	tr := tar.NewReader(reader)

	for {
		header, err := tr.Next()
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		}

		fileName := header.Name
		if !filepath.IsLocal(fileName) {
			return errors.New("tar file contains non-local file names")
		}

		target := filepath.Join(destination, filepath.Clean(fileName))

		switch header.Typeflag {
		case tar.TypeDir:
			if _, err := fileSystem.Stat(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}

			if err := fileSystem.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := fileSystem.OpenFile(target, syscall.O_CREAT|syscall.O_RDWR, fs.FileMode(header.Mode)) //nolint:gosec
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()

			// as long as this code in a test helper, we can safely
			// omit G110: Potential DoS vulnerability via decompression bomb
			if _, err := io.Copy(f, tr); err != nil { //nolint:gosec
				return err
			}
		}
	}
}
