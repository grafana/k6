package wazero

import (
	"io/fs"

	"github.com/tetratelabs/wazero/internal/sysfs"
)

// FSConfig configures filesystem paths the embedding host allows the wasm
// guest to access. Unconfigured paths are not allowed, so functions like
// `path_open` result in unsupported errors (e.g. syscall.ENOSYS).
//
// # Guest Path
//
// `guestPath` is the name of the path the guest should use a filesystem for, or
// empty for any files.
//
// All `guestPath` paths are normalized, specifically removing any leading or
// trailing slashes. This means "/", "./" or "." all coerce to empty "".
//
// Multiple `guestPath` values can be configured, but the last longest match
// wins. For example, if "tmp", then "" were added, a request to open
// "tmp/foo.txt" use the filesystem associated with "tmp" even though a wider
// path, "" (all files), was added later.
//
// A `guestPath` of "." coerces to the empty string "" because the current
// directory is handled by the guest. In other words, the guest resolves ites
// current directory prior to requesting files.
//
// More notes on `guestPath`
//   - Go compiled with runtime.GOOS=js do not pay attention to this value.
//     Hence, you need to normalize the filesystem with NewRootFS to ensure
//     paths requested resolve as expected.
//   - Working directories are typically tracked in wasm, though possible some
//     relative paths are requested. For example, TinyGo may attempt to resolve
//     a path "../.." in unit tests.
//   - Zig uses the first path name it sees as the initial working directory of
//     the process.
//
// # Scope
//
// Configuration here is module instance scoped. This means you can use the
// same configuration for multiple calls to Runtime.InstantiateModule. Each
// module will have a different file descriptor table. Any errors accessing
// resources allowed here are deferred to instantiation time of each module.
//
// Any host resources present at the time of configuration, but deleted before
// Runtime.InstantiateModule will trap/panic when the guest wasm initializes or
// calls functions like `fd_read`.
//
// # Windows
//
// While wazero supports Windows as a platform, all known compilers use POSIX
// conventions at runtime. For example, even when running on Windows, paths
// used by wasm are separated by forward slash (/), not backslash (\).
//
// # Notes
//
//   - FSConfig is immutable. Each WithXXX function returns a new instance
//     including the corresponding change.
//   - RATIONALE.md includes design background and relationship to WebAssembly
//     System Interfaces (WASI).
type FSConfig interface {
	// WithDirMount assigns a directory at `dir` to any paths beginning at
	// `guestPath`.
	//
	// If the same `guestPath` was assigned before, this overrides its value,
	// retaining the original precedence. See the documentation of FSConfig for
	// more details on `guestPath`.
	//
	// # Isolation
	//
	// The guest will have full access to this directory including escaping it
	// via relative path lookups like "../../". Full access includes operations
	// such as creating or deleting files, limited to any host level access
	// controls.
	//
	// # os.DirFS
	//
	// This configuration optimizes for WASI compatability which is sometimes
	// at odds with the behavior of os.DirFS. Hence, this will not behave
	// exactly the same as os.DirFS. See /RATIONALE.md for more.
	WithDirMount(dir, guestPath string) FSConfig

	// WithReadOnlyDirMount assigns a directory at `dir` to any paths
	// beginning at `guestPath`.
	//
	// This is the same as WithDirMount except only read operations are
	// permitted. However, escaping the directory via relative path lookups
	// like "../../" is still allowed.
	WithReadOnlyDirMount(dir, guestPath string) FSConfig

	// WithFSMount assigns a fs.FS file system for any paths beginning at
	// `guestPath`.
	//
	// If the same `guestPath` was assigned before, this overrides its value,
	// retaining the original precedence. See the documentation of FSConfig for
	// more details on `guestPath`.
	//
	// # Isolation
	//
	// fs.FS does not restrict the ability to overwrite returned files via
	// io.Writer. Moreover, os.DirFS documentation includes important notes
	// about isolation, which also applies to fs.Sub. As of Go 1.19, the
	// built-in file-systems are not jailed (chroot). See
	// https://github.com/golang/go/issues/42322
	//
	// # os.DirFS
	//
	// Due to limited control and functionality available in os.DirFS, we
	// advise using WithDirMount instead. There will be behavior differences
	// between os.DirFS and WithDirMount, as the latter biases towards what's
	// expected from WASI implementations.
	WithFSMount(fs fs.FS, guestPath string) FSConfig
}

type fsConfig struct {
	// fs are the currently configured filesystems.
	fs []sysfs.FS
	// guestPaths are the user-supplied names of the filesystems, retained for
	// error messages and fmt.Stringer.
	guestPaths []string
	// guestPathToFS are the normalized paths to the currently configured
	// filesystems, used for de-duplicating.
	guestPathToFS map[string]int
}

// NewFSConfig returns a FSConfig that can be used for configuring module instantiation.
func NewFSConfig() FSConfig {
	return &fsConfig{guestPathToFS: map[string]int{}}
}

// clone makes a deep copy of this module config.
func (c *fsConfig) clone() *fsConfig {
	ret := *c // copy except slice and maps which share a ref
	ret.fs = make([]sysfs.FS, 0, len(c.fs))
	ret.fs = append(ret.fs, c.fs...)
	ret.guestPaths = make([]string, 0, len(c.guestPaths))
	ret.guestPaths = append(ret.guestPaths, c.guestPaths...)
	ret.guestPathToFS = make(map[string]int, len(c.guestPathToFS))
	for key, value := range c.guestPathToFS {
		ret.guestPathToFS[key] = value
	}
	return &ret
}

// WithDirMount implements FSConfig.WithDirMount
func (c *fsConfig) WithDirMount(dir, guestPath string) FSConfig {
	return c.withMount(sysfs.NewDirFS(dir), guestPath)
}

// WithReadOnlyDirMount implements FSConfig.WithReadOnlyDirMount
func (c *fsConfig) WithReadOnlyDirMount(dir, guestPath string) FSConfig {
	return c.withMount(sysfs.NewReadFS(sysfs.NewDirFS(dir)), guestPath)
}

// WithFSMount implements FSConfig.WithFSMount
func (c *fsConfig) WithFSMount(fs fs.FS, guestPath string) FSConfig {
	return c.withMount(sysfs.Adapt(fs), guestPath)
}

func (c *fsConfig) withMount(fs sysfs.FS, guestPath string) FSConfig {
	cleaned := sysfs.StripPrefixesAndTrailingSlash(guestPath)
	ret := c.clone()
	if i, ok := ret.guestPathToFS[cleaned]; ok {
		ret.fs[i] = fs
		ret.guestPaths[i] = guestPath
	} else {
		ret.guestPathToFS[cleaned] = len(ret.fs)
		ret.fs = append(ret.fs, fs)
		ret.guestPaths = append(ret.guestPaths, guestPath)
	}
	return ret
}

func (c *fsConfig) toFS() (sysfs.FS, error) {
	return sysfs.NewRootFS(c.fs, c.guestPaths)
}
