package storage

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LocalFilePersister will persist files to the local disk.
type LocalFilePersister struct{}

// Persist will write the contents of data to the local disk on the specified path.
// TODO: we should not write to disk here but put it on some queue for async disk writes.
func (l *LocalFilePersister) Persist(path string, data io.Reader) (err error) {
	cp := filepath.Clean(path)

	dir := filepath.Dir(cp)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating a local directory %q: %w", dir, err)
	}

	f, err := os.OpenFile(cp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("creating a local file %q: %w", cp, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing the local file %q: %w", cp, cerr)
		}
	}()

	bf := bufio.NewWriter(f)

	if _, err := io.Copy(bf, data); err != nil {
		return fmt.Errorf("copying data to file: %w", err)
	}

	if err := bf.Flush(); err != nil {
		return fmt.Errorf("flushing data to disk: %w", err)
	}

	return nil
}
