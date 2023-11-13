package fs

import (
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"go.k6.io/k6/lib/fsext"
)

// cache is a cache of opened files, designed to minimize redundant file reads, and
// avoid replicating the content of the files in memory as much as possible.
//
// Unlike the underlying [fsext.Fs] which also caches file contents, this cache minimizes
// synchronization overhead. [fsext.Fs], using `afero`, employs a [sync.RWMutex] for each
// file access, involving lock/unlock operations. Our cache, however, utilizes a concurrent-safe
// map (openedFiles), bypassing the need for these locks and enhancing performance.
//
// This cache could be seen as redundant, as the underlying [fsext.Fs] implementation
// already caches the content of the files it opens. However, the current implementation of
// [fsext.Fs] relies on `afero` under the hood, which in turn relies on a [sync.RWMutex] to
// protect access to the cached file content. This means that every time a file is opened,
// the `fsext.Fs` cache is accessed, and the [sync.RWMutex] is locked and unlocked.
//
// This cache is designed to avoid this synchronization overhead, by caching the content of
// the files in a map that is safe for concurrent use, and thus avoid the need for a lock.
//
// This leads to a performance improvement, at the cost of holding the content of the files
// in memory twice, once in the cache's `openedFiles` map, and once in the `fsext.Fs` cache.
//
// Note that the current implementation of the cache diverges from the guarantees expressed in the
// [design document] defining the `fs` module, as it we effectively hold the file's content in memory
// twice as opposed to once.
//
// Future updates (see [#1079](https://github.com/grafana/k6/issues/1079)) may phase out reliance on `afero`.
// Depending on our new choice for [fsext] implementation, this cache might become obsolete, allowing us
// to solely depend on [fsext.Fs.Open].
//
// [#1079]: https://github.com/grafana/k6/issues/1079
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
// stores it in the cache's internal `openedFiles` map.
//
// The function cleans the provided filename using filepath.Clean before using it.
//
// If the file was previously "opened" (and thus cached), it
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
			panic(fmt.Errorf("cache's file %s is not stored as a byte slice", filename))
		}

		return data, nil
	}

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
