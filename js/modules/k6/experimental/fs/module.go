// Package fs provides a k6 module that allows users to interact with files from the
// local filesystem as per the [File API design document].
//
// [File API design document]: https://github.com/grafana/k6/blob/master/docs/design/019-file-api.md#proposed-solution
package fs

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/js/promises"
	"go.k6.io/k6/lib/fsext"
)

type (
	// RootModule is the global module instance that will create instances of our
	// module for each VU.
	RootModule struct {
		cache *cache
	}

	// ModuleInstance represents an instance of the fs module for a single VU.
	ModuleInstance struct {
		vu    modules.VU
		cache *cache
	}
)

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// New returns a pointer to a new [RootModule] instance.
func New() *RootModule {
	return &RootModule{
		cache: &cache{},
	}
}

// NewModuleInstance implements the modules.Module interface and returns a new
// instance of our module for the given VU.
func (rm *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{vu: vu, cache: rm.cache}
}

// Exports implements the modules.Module interface and returns the exports of
// our module.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]any{
			"open": mi.Open,
			"SeekMode": map[string]any{
				"Start":   SeekModeStart,
				"Current": SeekModeCurrent,
				"End":     SeekModeEnd,
			},
		},
	}
}

// Open opens a file and returns a promise that will resolve to a [File] instance
func (mi *ModuleInstance) Open(path goja.Value) *goja.Promise {
	promise, resolve, reject := promises.New(mi.vu)

	if mi.vu.State() != nil {
		reject(newFsError(ForbiddenError, "open() failed; reason: opening a file is allowed only in the Init context"))
		return promise
	}

	if common.IsNullish(path) {
		reject(newFsError(TypeError, "open() failed; reason: path cannot be null or undefined"))
		return promise
	}

	// Obtain the underlying path string from the JS value.
	pathStr := path.String()
	if pathStr == "" {
		reject(newFsError(TypeError, "open() failed; reason: path cannot be empty"))
		return promise
	}

	go func() {
		file, err := mi.openImpl(pathStr)
		if err != nil {
			reject(err)
			return
		}

		resolve(file)
	}()

	return promise
}

func (mi *ModuleInstance) openImpl(path string) (*File, error) {
	initEnv := mi.vu.InitEnv()

	// We resolve the path relative to the entrypoint script, as opposed to
	// the current working directory (the k6 command is called from).
	//
	// This is done on purpose, although it diverges in some respect with
	// how files are handled in different k6 contexts, so that we cater to
	// and intuitive user experience.
	//
	// See #2781 and #2674.
	path = fsext.Abs(initEnv.CWD.Path, path)

	fs, ok := initEnv.FileSystems["file"]
	if !ok {
		return nil, errors.New("open() failed; reason: unable to access the file system")
	}

	if exists, err := fsext.Exists(fs, path); err != nil {
		return nil, fmt.Errorf("open() failed, unable to verify if %q exists; reason: %w", path, err)
	} else if !exists {
		return nil, newFsError(NotFoundError, fmt.Sprintf("no such file or directory %q", path))
	}

	if isDir, err := fsext.IsDir(fs, path); err != nil {
		return nil, fmt.Errorf("open() failed, unable to verify if %q is a directory; reason: %w", path, err)
	} else if isDir {
		return nil, newFsError(
			InvalidResourceError,
			fmt.Sprintf("cannot open %q: opening a directory is not supported", path),
		)
	}

	data, err := mi.cache.open(path, fs)
	if err != nil {
		return nil, err
	}

	return &File{
		Path: path,
		file: file{
			path: path,
			data: data,
		},
		vu:    mi.vu,
		cache: mi.cache,
	}, nil
}

// File represents a file and exposes methods to interact with it.
//
// It is a wrapper around the [file] struct, which is meant to be directly
// exposed to the JS runtime.
type File struct {
	// Path holds the name of the file, as presented to [Open].
	Path string `json:"path"`

	// file contains the actual implementation for the file system.
	file

	// vu holds a reference to the VU this file is associated with.
	//
	// We need this to be able to access the VU's runtime, and produce
	// promises that are handled by the VU's runtime.
	vu modules.VU

	// cache holds a pointer to the file cache this file is associated
	// with. That way we are able to close the file when it's not needed
	// anymore.
	cache *cache
}

// Stat returns a promise that will resolve to a [FileInfo] instance describing
// the file.
func (f *File) Stat() *goja.Promise {
	promise, resolve, _ := promises.New(f.vu)

	go func() {
		resolve(f.file.stat())
	}()

	return promise
}

// Read the file's content, and writes it into the provided Uint8Array.
//
// Resolves to either the number of bytes read during the operation
// or EOF (null) if there was nothing more to read.
//
// It is possible for a read to successfully return with 0 bytes.
// This does not indicate EOF.
func (f *File) Read(into goja.Value) *goja.Promise {
	promise, resolve, reject := f.vu.Runtime().NewPromise()

	if common.IsNullish(into) {
		reject(newFsError(TypeError, "read() failed; reason: into argument cannot be null or undefined"))
		return promise
	}

	intoObj := into.ToObject(f.vu.Runtime())
	if !isUint8Array(f.vu.Runtime(), intoObj) {
		reject(newFsError(TypeError, "read() failed; reason: into argument must be a Uint8Array"))
		return promise
	}

	// Obtain the underlying ArrayBuffer from the Uint8Array
	ab, ok := intoObj.Get("buffer").Export().(goja.ArrayBuffer)
	if !ok {
		reject(newFsError(TypeError, "read() failed; reason: into argument must be a Uint8Array"))
		return promise
	}

	// To avoid concurrency linked to modifying the runtime's `into` buffer from multiple
	// goroutines we make sure to work on a separate copy, and will copy the bytes back
	// into the runtime's `into` buffer once the promise is resolved.
	intoBytes := ab.Bytes()
	buffer := make([]byte, len(intoBytes))

	// We register a callback to be executed by the VU's runtime.
	// This ensures that the modification of the JS runtime's `into` buffer
	// occurs on the main thread, during the promise's resolution.
	callback := f.vu.RegisterCallback()
	go func() {
		n, readErr := f.file.Read(buffer)
		callback(func() error {
			_ = copy(intoBytes[0:n], buffer)

			// Read was successful, resolve early with the number of
			// bytes read.
			if readErr == nil {
				resolve(n)
				return nil
			}

			var fsErr *fsError
			isFSErr := errors.As(readErr, &fsErr)

			if !isFSErr {
				reject(readErr)
				return nil
			}

			if fsErr.kind == EOFError && n == 0 {
				resolve(goja.Null())
			} else {
				resolve(n)
			}

			return nil
		})
	}()

	return promise
}

// Seek seeks to the given `offset` in the file, under the given `whence` mode.
//
// The returned promise resolves to the new `offset` (position) within the file, which
// is expressed in bytes from the selected start, current, or end position depending
// the provided `whence`.
func (f *File) Seek(offset goja.Value, whence goja.Value) *goja.Promise {
	promise, resolve, reject := f.vu.Runtime().NewPromise()

	intOffset, err := exportInt(offset)
	if err != nil {
		reject(newFsError(TypeError, "seek() failed; reason: the offset argument "+err.Error()))
		return promise
	}

	intWhence, err := exportInt(whence)
	if err != nil {
		reject(newFsError(TypeError, "seek() failed; reason: the whence argument "+err.Error()))
		return promise
	}

	seekMode := SeekMode(intWhence)
	switch seekMode {
	case SeekModeStart, SeekModeCurrent, SeekModeEnd:
		// Valid modes, do nothing.
	default:
		reject(newFsError(TypeError, "seek() failed; reason: the whence argument must be a SeekMode"))
		return promise
	}

	callback := f.vu.RegisterCallback()
	go func() {
		newOffset, err := f.file.Seek(intOffset, seekMode)
		callback(func() error {
			if err != nil {
				reject(err)
				return err
			}

			resolve(newOffset)
			return nil
		})
	}()

	return promise
}

func isUint8Array(rt *goja.Runtime, o *goja.Object) bool {
	uint8ArrayConstructor := rt.Get("Uint8Array")
	if isUint8Array := o.Get("constructor").SameAs(uint8ArrayConstructor); !isUint8Array {
		return false
	}

	return true
}

func exportInt(v goja.Value) (int64, error) {
	if common.IsNullish(v) {
		return 0, errors.New("cannot be null or undefined")
	}

	// We initially tried using `ExportTo` with a int64 value argument, however
	// this led to a string passed as argument not being an error.
	// Thus, we explicitly check that the value is a number, by comparing
	// its export type to the type of an int64.
	if v.ExportType().Kind() != reflect.Int64 {
		return 0, errors.New("must be a number")
	}

	return v.ToInteger(), nil
}
