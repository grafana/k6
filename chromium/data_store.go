package chromium

import (
	"fmt"
	"os"
)

const k6BrowserDataDirPattern = "xk6-browser-data-*"

// DataStore manages data storage for the extension and user specific data.
type DataStore struct {
	Dir    string // path to the data storage directory
	remove bool   // whether to remove the temporary directory in cleanup

	// FS abstractions
	fsMkdirTemp func(dir, pattern string) (string, error)
	fsRemoveAll func(path string) error
}

// Make creates a new temporary directory in tmpDir, and stores the path to
// the directory in the Dir field.
// When the Dir argument is not empty, no directory will be created.
func (d *DataStore) Make(tmpDir string, dir interface{}) error {
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
		return fmt.Errorf("mkdirTemp: %w", err)
	}
	d.remove = true

	return nil
}

// Cleanup removes the temporary directory.
// it is named as Cleanup because it can be used for other features
// in the future.
func (d *DataStore) Cleanup() {
	if !d.remove {
		return
	}
	if d.fsRemoveAll == nil {
		d.fsRemoveAll = os.RemoveAll
	}
	_ = d.fsRemoveAll(d.Dir)
}
