package fs

import (
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"go.k6.io/k6/lib/fsext"
)

// cache is a cache of opened files.
type cache struct {
	// openedFiles holds a safe for concurrent use map, holding the content
	// of the files that were opened by the user.
	//
	// Keys are expected to be strings holding the openedFiles' path.
	// Values are expected to be byte slices holding the content of the opened file.
	//
	// That way, we can cache the file's content and avoid opening too many
	// file descriptor, and re-reading its content every time the file is opened.
	//
	// Importantly, this also means that if the
	// file is modified from outside of k6, the changes will not be reflected in the file's data.
	openedFiles sync.Map
}

// open retrieves the content of a given file from the specified filesystem (fromFs) and
// stores it in the registry's internal `openedFiles` map.
//
// The function cleans the provided filename using filepath.Clean before using it.
//
// If the file was previously "opened" (and thus cached) by the registry, it
// returns the cached content. Otherwise, it reads the file from the
// filesystem, caches its content, and then returns it.
//
// The function is designed to minimize redundant file reads by leveraging an internal cache (openedFiles).
// In case the cached value is not a byte slice (which should never occur in regular use), it
// panics with a descriptive error.
//
// Parameters:
//   - filename: The name of the file to be retrieved. This should be a relative or absolute path.
//   - fromFs: The filesystem (from the fsext package) from which the file should be read if not already cached.
//
// Returns:
//   - A byte slice containing the content of the specified file.
//   - An error if there's any issue opening or reading the file. If the file content is
//     successfully cached and returned once, subsequent calls will not produce
//     file-related errors for the same file, as the cached value will be used.
func (fr *cache) open(filename string, fromFs fsext.Fs) (data []byte, err error) {
	filename = filepath.Clean(filename)

	if f, ok := fr.openedFiles.Load(filename); ok {
		data, ok = f.([]byte)
		if !ok {
			panic(fmt.Errorf("registry's file %s is not stored as a byte slice", filename))
		}

		return data, nil
	}

	// The underlying fsext.Fs.Open method will cache the file content during this
	// operation. Which will lead to effectively holding the content of the file in memory twice.
	// However, as per #1079, we plan to eventually reduce our dependency on fsext, and
	// expect this issue to be resolved at that point.
	// TODO: re-evaluate opening from the FS this once #1079 is resolved.
	f, err := fromFs.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		cerr := f.Close()
		if cerr != nil {
			err = fmt.Errorf("failed to close file %s: %w", filename, cerr)
		}
	}()

	data, err = io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read the content of file %s: %w", filename, err)
	}

	fr.openedFiles.Store(filename, data)

	return data, nil
}
