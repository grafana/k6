package fs

import (
	"fmt"
	"io"
	"path/filepath"
	"sync"

	"github.com/spf13/afero"
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

	// refCounts holds a safe for concurrent use map, holding the number of times
	// a file was opened.
	openedRefCounts map[string]uint

	// mutex is used to ensure that the openedRefCounts map is not accessed concurrently.
	// We resort to using a mutex as opposed to a sync.Map because we need to be able to
	mutex sync.Mutex
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
//   - fromFs: The filesystem (from the afero package) from which the file should be read if not already cached.
//
// Returns:
//   - A byte slice containing the content of the specified file.
//   - An error if there's any issue opening or reading the file. If the file content is
//     successfully cached and returned once, subsequent calls will not produce
//     file-related errors for the same file, as the cached value will be used.
func (fr *cache) open(filename string, fromFs afero.Fs) ([]byte, error) {
	filename = filepath.Clean(filename)

	if f, ok := fr.openedFiles.Load(filename); ok {
		data, ok := f.([]byte)
		if !ok {
			panic(fmt.Errorf("registry's file %s is not stored as a byte slice", filename))
		}

		// Increase the ref count.
		fr.mutex.Lock()
		fr.openedRefCounts[filename]++
		fmt.Println("cache.open[file already loaded]: openedRefCount=", fr.openedRefCounts[filename])
		fr.mutex.Unlock()

		return data, nil
	}

	// The underlying afero.Fs.Open method will cache the file content during this
	// operation. Which will lead to effectively holding the content of the file in memory twice.
	// However, as per #1079, we plan to eventually reduce our dependency on afero, and
	// expect this issue to be resolved at that point.
	// TODO: re-evaluate opening from the FS this once #1079 is resolved.
	f, err := fromFs.Open(filename)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	fr.openedFiles.Store(filename, data)

	// As this is the first time we open the file, initialize
	// the ref count to 1.
	fr.mutex.Lock()
	fr.openedRefCounts[filename] = 1
	fmt.Println("cache.open[initializing open]: openedRefCount=", 1)
	fr.mutex.Unlock()

	return data, nil
}

func (fr *cache) close(filename string) error {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()

	// If no ref count is found, it means the file was either never
	// opened, or already closed.
	if _, ok := fr.openedRefCounts[filename]; !ok {
		return newFsError(ClosedError, fmt.Sprintf("file %s is not opened", filename))
	}

	// If the ref count is 0, it means the file was already closed.
	if fr.openedRefCounts[filename] == 0 {
		return newFsError(ClosedError, fmt.Sprintf("file %s is already closed", filename))
	}

	// Decrease the ref count.
	fr.openedRefCounts[filename]--
	fmt.Println("cache.close: openedRefCount=", fr.openedRefCounts[filename])

	// If the ref count is 0, it means the file was closed for the last time.
	// We can safely remove it from the openedFiles map.
	if fr.openedRefCounts[filename] == 0 {
		fr.openedFiles.Delete(filename)
		delete(fr.openedRefCounts, filename)
		fmt.Println("cache.close: last openedRefCount=", fr.openedRefCounts[filename])
	}

	return nil
}
