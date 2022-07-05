package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

const k6BrowserDataDirPattern = "xk6-browser-data-*"

// Dir manages data storage for the extension and user specific data
// on the local filesystem.
type Dir struct {
	Dir    string // path to the data storage directory
	remove bool   // whether to remove the temporary directory in cleanup

	// FS abstractions
	fsMkdirTemp   func(dir, pattern string) (string, error)
	fsRemoveAll   func(path string) error
	fsRemoveAllMu sync.Mutex
}

// Make creates a new temporary directory in tmpDir, and stores the path to
// the directory in the Dir field. When the dir argument is not empty, no
// directory will be created and it will not be deleted if Cleanup is called.
func (d *Dir) Make(tmpDir string, dir interface{}) error {
	// use the provided dir.
	if ud, ok := dir.(string); ok && ud != "" {
		d.Dir = ud
		return nil
	}

	// create a temporary dir because the provided dir is empty.
	if d.fsMkdirTemp == nil {
		d.fsMkdirTemp = os.MkdirTemp
	}
	var err error
	if d.Dir, err = d.fsMkdirTemp(tmpDir, k6BrowserDataDirPattern); err != nil {
		var (
			pe   *fs.PathError
			path = filepath.Join(tmpDir, k6BrowserDataDirPattern)
		)
		if errors.As(err, &pe) {
			path = pe.Path
			err = pe.Err
		}

		return fmt.Errorf("making browser data directory %q: %w", path, err)
	}
	d.remove = true

	return nil
}

// Cleanup removes the temporary directory if Make was called with a non
// empty dir argument.
// It is named as Cleanup because it can be used for other features in the
// future.
func (d *Dir) Cleanup() error {
	if !d.remove {
		return nil
	}

	d.fsRemoveAllMu.Lock()
	defer d.fsRemoveAllMu.Unlock()

	if d.fsRemoveAll == nil {
		d.fsRemoveAll = os.RemoveAll
	}

	return d.fsRemoveAll(d.Dir)
}
