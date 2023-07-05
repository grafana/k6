package fs

import (
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/spf13/afero"
)

// registry is a registry of opened files.
type registry struct {
	// files holds a safe for concurrent use map of opened files.
	//
	// Keys are expected to be strings holding the files' path.
	// Values are expected to be byte slices holding the files' data.
	//
	// That way, we can cache the file's content and avoid opening too many
	// file descriptor, and re-reading its content every time the file is opened.
	//
	// Importantly, this also means that if the
	// file is modified from outside of k6, the changes will not be reflected in the file's data.
	// files map[string][]byte
	files sync.Map
}

// open opens the named file for reading.
//
// If the file was already opened, it returns a pointer to the cached file data. Otherwise, it
// opens the file, reads its content, caches it, and returns a pointer to it.
//
// The file is always opened in read-only mode. The provided file path is cleaned before being
// used.
func (fr *registry) open(filename string, fromFs afero.Fs) ([]byte, error) {
	filename = filepath.Clean(filename)

	if f, ok := fr.files.Load(filename); ok {
		data, ok := f.([]byte)
		if !ok {
			panic(fmt.Errorf("registry's file %s is not stored as a byte slice", filename))
		}

		return data, nil
	}

	f, err := fromFs.Open(filename)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	fr.files.Store(filename, data)

	return data, nil
}
