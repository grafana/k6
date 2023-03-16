package sysfs

import (
	"fmt"
	"io"
	"io/fs"
	"strings"
	"syscall"
	"time"

	"github.com/tetratelabs/wazero/internal/platform"
)

func NewRootFS(fs []FS, guestPaths []string) (FS, error) {
	switch len(fs) {
	case 0:
		return UnimplementedFS{}, nil
	case 1:
		if StripPrefixesAndTrailingSlash(guestPaths[0]) == "" {
			return fs[0], nil
		}
	}

	ret := &CompositeFS{
		string:            stringFS(fs, guestPaths),
		fs:                make([]FS, len(fs)),
		guestPaths:        make([]string, len(fs)),
		cleanedGuestPaths: make([]string, len(fs)),
		rootGuestPaths:    map[string]int{},
		rootIndex:         -1,
	}

	copy(ret.guestPaths, guestPaths)
	copy(ret.fs, fs)

	for i, guestPath := range guestPaths {
		// Clean the prefix in the same way path matches will.
		cleaned := StripPrefixesAndTrailingSlash(guestPath)
		if cleaned == "" {
			if ret.rootIndex != -1 {
				return nil, fmt.Errorf("multiple root filesystems are invalid: %s", ret.string)
			}
			ret.rootIndex = i
		} else if strings.HasPrefix(cleaned, "..") {
			// ../ mounts are special cased and aren't returned in a directory
			// listing, so we can ignore them for now.
		} else if strings.Contains(cleaned, "/") {
			return nil, fmt.Errorf("only single-level guest paths allowed: %s", ret.string)
		} else {
			ret.rootGuestPaths[cleaned] = i
		}
		ret.cleanedGuestPaths[i] = cleaned
	}

	// Ensure there is always a root match to keep runtime logic simpler.
	if ret.rootIndex == -1 {
		ret.rootIndex = len(fs)
		ret.cleanedGuestPaths = append(ret.cleanedGuestPaths, "")
		ret.fs = append(ret.fs, &fakeRootFS{})
	}
	return ret, nil
}

type CompositeFS struct {
	UnimplementedFS
	// string is cached for convenience.
	string string
	// fs is index-correlated with cleanedGuestPaths
	fs []FS
	// guestPaths are the original paths supplied by the end user, cleaned as
	// cleanedGuestPaths.
	guestPaths []string
	// cleanedGuestPaths to match in precedence order, ascending.
	cleanedGuestPaths []string
	// rootGuestPaths are cleanedGuestPaths that exist directly under root, such as
	// "tmp".
	rootGuestPaths map[string]int
	// rootIndex is the index in fs that is the root filesystem
	rootIndex int
}

// String implements fmt.Stringer
func (c *CompositeFS) String() string {
	return c.string
}

func stringFS(fs []FS, guestPaths []string) string {
	var ret strings.Builder
	ret.WriteString("[")
	writeMount(&ret, fs[0], guestPaths[0])
	for i, f := range fs[1:] {
		ret.WriteString(" ")
		writeMount(&ret, f, guestPaths[i+1])
	}
	ret.WriteString("]")
	return ret.String()
}

func writeMount(ret *strings.Builder, f FS, guestPath string) {
	ret.WriteString(f.String())
	ret.WriteString(":")
	ret.WriteString(guestPath)
	if _, ok := f.(*readFS); ok {
		ret.WriteString(":ro")
	}
}

// GuestPaths returns the underlying pre-open paths in original order.
func (c *CompositeFS) GuestPaths() (guestPaths []string) {
	return c.guestPaths
}

// FS returns the underlying filesystems in original order.
func (c *CompositeFS) FS() (fs []FS) {
	fs = make([]FS, len(c.guestPaths))
	copy(fs, c.fs)
	return
}

// Open implements the same method as documented on fs.FS
func (c *CompositeFS) Open(name string) (fs.File, error) {
	return fsOpen(c, name)
}

// OpenFile implements FS.OpenFile
func (c *CompositeFS) OpenFile(path string, flag int, perm fs.FileMode) (f fs.File, err error) {
	matchIndex, relativePath := c.chooseFS(path)

	f, err = c.fs[matchIndex].OpenFile(relativePath, flag, perm)
	if err != nil {
		return
	}

	// Ensure the root directory listing includes any prefix mounts.
	if matchIndex == c.rootIndex {
		switch path {
		case ".", "/", "":
			if len(c.rootGuestPaths) > 0 {
				f = &openRootDir{c: c, f: f.(fs.ReadDirFile)}
			}
		}
	}
	return
}

// An openRootDir is a root directory open for reading, which has mounts inside
// of it.
type openRootDir struct {
	c        *CompositeFS
	f        fs.ReadDirFile // the directory file itself
	dirents  []fs.DirEntry  // the directory contents
	direntsI int            // the read offset, an index into the files slice
}

func (d *openRootDir) Close() error { return d.f.Close() }

func (d *openRootDir) Stat() (fs.FileInfo, error) { return d.f.Stat() }

func (d *openRootDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: "/", Err: syscall.EISDIR}
}

// readDir reads the directory fully into d.dirents, replacing any entries that
// correspond to prefix matches or appending them to the end.
func (d *openRootDir) readDir() (err error) {
	if d.dirents, err = d.f.ReadDir(-1); err != nil {
		return
	}

	remaining := make(map[string]int, len(d.c.rootGuestPaths))
	for k, v := range d.c.rootGuestPaths {
		remaining[k] = v
	}

	for i, e := range d.dirents {
		if fsI, ok := remaining[e.Name()]; ok {
			if d.dirents[i], err = d.rootEntry(e.Name(), fsI); err != nil {
				return
			}
			delete(remaining, e.Name())
		}
	}

	var di fs.DirEntry
	for n, fsI := range remaining {
		if di, err = d.rootEntry(n, fsI); err != nil {
			return
		}
		d.dirents = append(d.dirents, di)
	}
	return
}

func (d *openRootDir) rootEntry(name string, fsI int) (fs.DirEntry, error) {
	var stat platform.Stat_t
	if err := d.c.fs[fsI].Stat(".", &stat); err != nil {
		return nil, err
	} else {
		return &dirInfo{name, &stat}, nil
	}
}

// dirInfo is a DirEntry based on a FileInfo.
type dirInfo struct {
	// name is needed to retain the stat info for a mount, knowing the
	// directory is masked. For example, we don't want to leak the underlying
	// host directory name.
	name string
	stat *platform.Stat_t
}

func (i *dirInfo) Name() string               { return i.name }
func (i *dirInfo) Type() fs.FileMode          { return i.stat.Mode.Type() }
func (i *dirInfo) Info() (fs.FileInfo, error) { return i, nil }
func (i *dirInfo) Size() int64                { return i.stat.Size }
func (i *dirInfo) Mode() fs.FileMode          { return i.stat.Mode }
func (i *dirInfo) ModTime() time.Time {
	return time.Unix(i.stat.Mtim/1e9, i.stat.Mtim%1e9)
}
func (i *dirInfo) IsDir() bool      { return i.stat.Mode.IsDir() }
func (i *dirInfo) Sys() interface{} { return nil }

func (d *openRootDir) ReadDir(count int) ([]fs.DirEntry, error) {
	if d.dirents == nil {
		if err := d.readDir(); err != nil {
			return nil, err
		}
	}

	// logic similar to go:embed
	n := len(d.dirents) - d.direntsI
	if n == 0 {
		if count <= 0 {
			return nil, nil
		}
		return nil, io.EOF
	}
	if count > 0 && n > count {
		n = count
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		list[i] = d.dirents[d.direntsI+i]
	}
	d.direntsI += n
	return list, nil
}

// Stat implements FS.Stat
func (c *CompositeFS) Stat(path string, stat *platform.Stat_t) error {
	matchIndex, relativePath := c.chooseFS(path)
	return c.fs[matchIndex].Stat(relativePath, stat)
}

// Mkdir implements FS.Mkdir
func (c *CompositeFS) Mkdir(path string, perm fs.FileMode) error {
	matchIndex, relativePath := c.chooseFS(path)
	return c.fs[matchIndex].Mkdir(relativePath, perm)
}

// Rename implements FS.Rename
func (c *CompositeFS) Rename(from, to string) error {
	fromFS, fromPath := c.chooseFS(from)
	toFS, toPath := c.chooseFS(to)
	if fromFS != toFS {
		return syscall.ENOSYS // not yet anyway
	}
	return c.fs[fromFS].Rename(fromPath, toPath)
}

// Rmdir implements FS.Rmdir
func (c *CompositeFS) Rmdir(path string) error {
	matchIndex, relativePath := c.chooseFS(path)
	return c.fs[matchIndex].Rmdir(relativePath)
}

// Unlink implements FS.Unlink
func (c *CompositeFS) Unlink(path string) error {
	matchIndex, relativePath := c.chooseFS(path)
	return c.fs[matchIndex].Unlink(relativePath)
}

// Utimes implements FS.Utimes
func (c *CompositeFS) Utimes(path string, atimeNsec, mtimeNsec int64) error {
	matchIndex, relativePath := c.chooseFS(path)
	return c.fs[matchIndex].Utimes(relativePath, atimeNsec, mtimeNsec)
}

// chooseFS chooses the best fs and the relative path to use for the input.
func (c *CompositeFS) chooseFS(path string) (matchIndex int, relativePath string) {
	matchIndex = -1
	matchPrefixLen := 0
	pathI, pathLen := stripPrefixesAndTrailingSlash(path)

	// Last is the highest precedence, so we iterate backwards. The last longest
	// match wins. e.g. the pre-open "tmp" wins vs "" regardless of order.
	for i := len(c.fs) - 1; i >= 0; i-- {
		prefix := c.cleanedGuestPaths[i]
		if eq, match := hasPathPrefix(path, pathI, pathLen, prefix); eq {
			// When the input equals the prefix, there cannot be a longer match
			// later. The relative path is the FS root, so return empty string.
			matchIndex = i
			relativePath = ""
			return
		} else if match {
			// Check to see if this is a longer match
			prefixLen := len(prefix)
			if prefixLen > matchPrefixLen || matchIndex == -1 {
				matchIndex = i
				matchPrefixLen = prefixLen
			}
		} // Otherwise, keep looking for a match
	}

	// Now, we know the path != prefix, but it matched an existing fs, because
	// setup ensures there's always a root filesystem.

	// If this was a root path match the cleaned path is the relative one to
	// pass to the underlying filesystem.
	if matchPrefixLen == 0 {
		// Avoid re-slicing when the input is already clean
		if pathI == 0 && len(path) == pathLen {
			relativePath = path
		} else {
			relativePath = path[pathI:pathLen]
		}
		return
	}

	// Otherwise, it is non-root match: the relative path is past "$prefix/"
	pathI += matchPrefixLen + 1 // e.g. prefix=foo, path=foo/bar -> bar
	relativePath = path[pathI:pathLen]
	return
}

// hasPathPrefix compares an input path against a prefix, both cleaned by
// stripPrefixesAndTrailingSlash. This returns a pair of eq, match to allow an
// early short circuit on match.
//
// Note: This is case-sensitive because POSIX paths are compared case
// sensitively.
func hasPathPrefix(path string, pathI, pathLen int, prefix string) (eq, match bool) {
	matchLen := pathLen - pathI
	if prefix == "" {
		return matchLen == 0, true // e.g. prefix=, path=foo
	}

	prefixLen := len(prefix)
	// reset pathLen temporarily to represent the length to match as opposed to
	// the length of the string (that may contain leading slashes).
	if matchLen == prefixLen {
		if pathContainsPrefix(path, pathI, prefixLen, prefix) {
			return true, true // e.g. prefix=bar, path=bar
		}
		return false, false
	} else if matchLen < prefixLen {
		return false, false // e.g. prefix=fooo, path=foo
	}

	if path[pathI+prefixLen] != '/' {
		return false, false // e.g. prefix=foo, path=fooo
	}

	// Not equal, but maybe a match. e.g. prefix=foo, path=foo/bar
	return false, pathContainsPrefix(path, pathI, prefixLen, prefix)
}

// pathContainsPrefix is faster than strings.HasPrefix even if we didn't cache
// the index,len. See benchmarks.
func pathContainsPrefix(path string, pathI, prefixLen int, prefix string) bool {
	for i := 0; i < prefixLen; i++ {
		if path[pathI] != prefix[i] {
			return false // e.g. prefix=bar, path=foo or foo/bar
		}
		pathI++
	}
	return true // e.g. prefix=foo, path=foo or foo/bar
}

func StripPrefixesAndTrailingSlash(path string) string {
	pathI, pathLen := stripPrefixesAndTrailingSlash(path)
	return path[pathI:pathLen]
}

// stripPrefixesAndTrailingSlash skips any leading "./" or "/" such that the
// result index begins with another string. A result of "." coerces to the
// empty string "" because the current directory is handled by the guest.
//
// Results are the offset/len pair which is an optimization to avoid re-slicing
// overhead, as this function is called for every path operation.
//
// Note: Relative paths should be handled by the guest, as that's what knows
// what the current directory is. However, paths that escape the current
// directory e.g. "../.." have been found in `tinygo test` and this
// implementation takes care to avoid it.
func stripPrefixesAndTrailingSlash(path string) (pathI, pathLen int) {
	// strip trailing slashes
	pathLen = len(path)
	for ; pathLen > 0 && path[pathLen-1] == '/'; pathLen-- {
	}

	pathI = 0
loop:
	for pathI < pathLen {
		switch path[pathI] {
		case '/':
			pathI++
		case '.':
			nextI := pathI + 1
			if nextI < pathLen && path[nextI] == '/' {
				pathI = nextI + 1
			} else if nextI == pathLen {
				pathI = nextI
			} else {
				break loop
			}
		default:
			break loop
		}
	}
	return
}

type fakeRootFS struct{ UnimplementedFS }

// OpenFile implements FS.OpenFile
func (*fakeRootFS) OpenFile(path string, flag int, perm fs.FileMode) (fs.File, error) {
	switch path {
	case ".", "/", "":
		return fakeRootDir{}, nil
	}
	return nil, syscall.ENOENT
}

type fakeRootDir struct{}

func (fakeRootDir) Close() (err error) { return }

func (fakeRootDir) Stat() (fs.FileInfo, error) { return fakeRootDirInfo{}, nil }

func (fakeRootDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: "/", Err: syscall.EISDIR}
}

type fakeRootDirInfo struct{}

func (fakeRootDirInfo) Name() string                               { return "/" }
func (fakeRootDirInfo) Size() int64                                { return 0 }
func (fakeRootDirInfo) Mode() fs.FileMode                          { return fs.ModeDir | 0o500 }
func (fakeRootDirInfo) ModTime() time.Time                         { return time.Unix(0, 0) }
func (fakeRootDirInfo) IsDir() bool                                { return true }
func (fakeRootDirInfo) Sys() interface{}                           { return nil }
func (fakeRootDir) ReadDir(int) (dirents []fs.DirEntry, err error) { return }
