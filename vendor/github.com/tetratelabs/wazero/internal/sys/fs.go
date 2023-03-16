package sys

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"syscall"
	"time"

	"github.com/tetratelabs/wazero/internal/descriptor"
	"github.com/tetratelabs/wazero/internal/platform"
	"github.com/tetratelabs/wazero/internal/sysfs"
)

const (
	FdStdin uint32 = iota
	FdStdout
	FdStderr
	// FdPreopen is the file descriptor of the first pre-opened directory.
	//
	// # Why file descriptor 3?
	//
	// While not specified, the most common WASI implementation, wasi-libc,
	// expects POSIX style file descriptor allocation, where the lowest
	// available number is used to open the next file. Since 1 and 2 are taken
	// by stdout and stderr, the next is 3.
	//   - https://github.com/WebAssembly/WASI/issues/122
	//   - https://pubs.opengroup.org/onlinepubs/9699919799/functions/V2_chap02.html#tag_15_14
	//   - https://github.com/WebAssembly/wasi-libc/blob/wasi-sdk-16/libc-bottom-half/sources/preopens.c#L215
	FdPreopen
)

const (
	modeDevice     = uint32(fs.ModeDevice | 0o640)
	modeCharDevice = uint32(fs.ModeCharDevice | 0o640)
)

type stdioFileWriter struct {
	w io.Writer
	s fs.FileInfo
}

// Stat implements fs.File
func (w *stdioFileWriter) Stat() (fs.FileInfo, error) { return w.s, nil }

// Read implements fs.File
func (w *stdioFileWriter) Read([]byte) (n int, err error) {
	return // emulate os.Stdout which returns zero
}

// Write implements io.Writer
func (w *stdioFileWriter) Write(p []byte) (n int, err error) {
	return w.w.Write(p)
}

// Close implements fs.File
func (w *stdioFileWriter) Close() error {
	// Don't actually close the underlying file, as we didn't open it!
	return nil
}

type stdioFileReader struct {
	r io.Reader
	s fs.FileInfo
}

// Stat implements fs.File
func (r *stdioFileReader) Stat() (fs.FileInfo, error) { return r.s, nil }

// Read implements fs.File
func (r *stdioFileReader) Read(p []byte) (n int, err error) {
	return r.r.Read(p)
}

// Close implements fs.File
func (r *stdioFileReader) Close() error {
	// Don't actually close the underlying file, as we didn't open it!
	return nil
}

var (
	noopStdinStat  = stdioFileInfo{FdStdin, modeDevice}
	noopStdoutStat = stdioFileInfo{FdStdout, modeDevice}
	noopStderrStat = stdioFileInfo{FdStderr, modeDevice}
)

// stdioFileInfo implements fs.FileInfo where index zero is the FD and one is the mode.
type stdioFileInfo [2]uint32

func (s stdioFileInfo) Name() string {
	switch s[0] {
	case FdStdin:
		return "stdin"
	case FdStdout:
		return "stdout"
	case FdStderr:
		return "stderr"
	default:
		panic(fmt.Errorf("BUG: incorrect FD %d", s[0]))
	}
}

func (stdioFileInfo) Size() int64         { return 0 }
func (s stdioFileInfo) Mode() fs.FileMode { return fs.FileMode(s[1]) }
func (stdioFileInfo) ModTime() time.Time  { return time.Unix(0, 0) }
func (stdioFileInfo) IsDir() bool         { return false }
func (stdioFileInfo) Sys() interface{}    { return nil }

type lazyDir struct {
	fs sysfs.FS
	f  fs.File
}

// Stat implements fs.File
func (r *lazyDir) Stat() (fs.FileInfo, error) {
	if f, err := r.file(); err != nil {
		return nil, err
	} else {
		return f.Stat()
	}
}

func (r *lazyDir) file() (f fs.File, err error) {
	if f = r.f; r.f != nil {
		return
	}
	r.f, err = r.fs.OpenFile(".", os.O_RDONLY, 0)
	f = r.f
	return
}

// Read implements fs.File
func (r *lazyDir) Read(p []byte) (n int, err error) {
	if f, err := r.file(); err != nil {
		return 0, err
	} else {
		return f.Read(p)
	}
}

// Close implements fs.File
func (r *lazyDir) Close() error {
	if f, err := r.file(); err != nil {
		return nil
	} else {
		return f.Close()
	}
}

// FileEntry maps a path to an open file in a file system.
type FileEntry struct {
	// Name is the name of the directory up to its pre-open, or the pre-open
	// name itself when IsPreopen.
	//
	// Note: This can drift on rename.
	Name string

	// IsPreopen is a directory that is lazily opened.
	IsPreopen bool

	// FS is the filesystem associated with the pre-open.
	FS sysfs.FS

	// cachedStat includes fields that won't change while a file is open.
	cachedStat *cachedStat

	// File is always non-nil.
	File fs.File

	// ReadDir is present when this File is a fs.ReadDirFile and `ReadDir`
	// was called.
	ReadDir *ReadDir

	openPath string
	openFlag int
	openPerm fs.FileMode
}

type cachedStat struct {
	// Ino is the file serial number, or zero if not available.
	Ino uint64

	// Type is the same as what's documented on platform.Dirent.
	Type fs.FileMode
}

// CachedStat returns the cacheable parts of platform.Stat_t or an error if
// they couldn't be retrieved.
func (f *FileEntry) CachedStat() (ino uint64, fileType fs.FileMode, err error) {
	if f.cachedStat == nil {
		var st platform.Stat_t
		if err = f.Stat(&st); err != nil {
			return
		}
		f.cachedStat = &cachedStat{Ino: st.Ino, Type: st.Mode & fs.ModeType}
	}
	return f.cachedStat.Ino, f.cachedStat.Type, nil
}

// Stat returns the underlying stat of this file.
func (f *FileEntry) Stat(st *platform.Stat_t) (err error) {
	if ld, ok := f.File.(*lazyDir); ok {
		var sf fs.File
		if sf, err = ld.file(); err == nil {
			err = platform.StatFile(sf, st)
		}
	} else {
		err = platform.StatFile(f.File, st)
	}

	if err == nil {
		f.cachedStat = &cachedStat{Ino: st.Ino, Type: st.Mode}
	}
	return
}

// ReadDir is the status of a prior fs.ReadDirFile call.
type ReadDir struct {
	// CountRead is the total count of files read including Dirents.
	CountRead uint64

	// Dirents is the contents of the last platform.Readdir call. Notably,
	// directory listing are not rewindable, so we keep entries around in case
	// the caller mis-estimated their buffer and needs a few still cached.
	//
	// Note: This is wasi-specific and needs to be refactored.
	// In wasi preview1, dot and dot-dot entries are required to exist, but the
	// reverse is true for preview2. More importantly, preview2 holds separate
	// stateful dir-entry-streams per file.
	Dirents []*platform.Dirent
}

type FSContext struct {
	// rootFS is the root ("/") mount.
	rootFS sysfs.FS

	// openedFiles is a map of file descriptor numbers (>=FdPreopen) to open files
	// (or directories) and defaults to empty.
	// TODO: This is unguarded, so not goroutine-safe!
	openedFiles FileTable
}

// FileTable is an specialization of the descriptor.Table type used to map file
// descriptors to file entries.
type FileTable = descriptor.Table[uint32, *FileEntry]

// NewFSContext creates a FSContext with stdio streams and an optional
// pre-opened filesystem.
//
// If `preopened` is not sysfs.UnimplementedFS, it is inserted into
// the file descriptor table as FdPreopen.
func NewFSContext(stdin io.Reader, stdout, stderr io.Writer, rootFS sysfs.FS) (fsc *FSContext, err error) {
	fsc = &FSContext{rootFS: rootFS}
	fsc.openedFiles.Insert(stdinReader(stdin))
	fsc.openedFiles.Insert(stdioWriter(stdout, noopStdoutStat))
	fsc.openedFiles.Insert(stdioWriter(stderr, noopStderrStat))

	if _, ok := rootFS.(sysfs.UnimplementedFS); ok {
		return fsc, nil
	}

	if comp, ok := rootFS.(*sysfs.CompositeFS); ok {
		preopens := comp.FS()
		for i, p := range comp.GuestPaths() {
			fsc.openedFiles.Insert(&FileEntry{
				FS:        preopens[i],
				Name:      p,
				IsPreopen: true,
				File:      &lazyDir{fs: rootFS},
			})
		}
	} else {
		fsc.openedFiles.Insert(&FileEntry{
			FS:        rootFS,
			Name:      "/",
			IsPreopen: true,
			File:      &lazyDir{fs: rootFS},
		})
	}

	return fsc, nil
}

func stdinReader(r io.Reader) *FileEntry {
	if r == nil {
		r = eofReader{}
	}
	s := stdioStat(r, noopStdinStat)
	return &FileEntry{Name: noopStdinStat.Name(), File: &stdioFileReader{r: r, s: s}}
}

func stdioWriter(w io.Writer, defaultStat stdioFileInfo) *FileEntry {
	if w == nil {
		w = io.Discard
	}
	s := stdioStat(w, defaultStat)
	return &FileEntry{Name: s.Name(), File: &stdioFileWriter{w: w, s: s}}
}

func stdioStat(f interface{}, defaultStat stdioFileInfo) fs.FileInfo {
	if f, ok := f.(*os.File); ok && platform.IsTerminal(f.Fd()) {
		return stdioFileInfo{defaultStat[0], modeCharDevice}
	}
	return defaultStat
}

// fileModeStat is a fake fs.FileInfo which only returns its mode.
// This is used for character devices.
type fileModeStat fs.FileMode

var _ fs.FileInfo = fileModeStat(0)

func (s fileModeStat) Size() int64        { return 0 }
func (s fileModeStat) Mode() fs.FileMode  { return fs.FileMode(s) }
func (s fileModeStat) ModTime() time.Time { return time.Unix(0, 0) }
func (s fileModeStat) Sys() interface{}   { return nil }
func (s fileModeStat) Name() string       { return "" }
func (s fileModeStat) IsDir() bool        { return false }

// RootFS returns the underlying filesystem. Any files that should be added to
// the table should be inserted via InsertFile.
func (c *FSContext) RootFS() sysfs.FS {
	return c.rootFS
}

// OpenFile opens the file into the table and returns its file descriptor.
// The result must be closed by CloseFile or Close.
func (c *FSContext) OpenFile(fs sysfs.FS, path string, flag int, perm fs.FileMode) (uint32, error) {
	if f, err := fs.OpenFile(path, flag, perm); err != nil {
		return 0, err
	} else {
		fe := &FileEntry{openPath: path, FS: fs, File: f, openFlag: flag, openPerm: perm}
		if path == "/" || path == "." {
			fe.Name = ""
		} else {
			fe.Name = path
		}
		newFD := c.openedFiles.Insert(fe)
		return newFD, nil
	}
}

// ReOpenDir re-opens the directory while keeping the same file descriptor.
// TODO: this might not be necessary once we have our own File type.
func (c *FSContext) ReOpenDir(fd uint32) (*FileEntry, error) {
	f, ok := c.openedFiles.Lookup(fd)
	if !ok {
		return nil, syscall.EBADF
	} else if _, ft, err := f.CachedStat(); err != nil {
		return nil, err
	} else if ft.Type() != fs.ModeDir {
		return nil, syscall.EISDIR
	}

	if err := c.reopen(f); err != nil {
		return f, err
	}

	f.ReadDir.CountRead, f.ReadDir.Dirents = 0, nil
	return f, nil
}

func (c *FSContext) reopen(f *FileEntry) error {
	if err := f.File.Close(); err != nil {
		return err
	}

	// Re-opens with  the same parameters as before.
	opened, err := f.FS.OpenFile(f.openPath, f.openFlag, f.openPerm)
	if err != nil {
		return err
	}

	// Reset the state.
	f.File = opened
	return nil
}

// ChangeOpenFlag changes the open flag of the given opened file pointed by `fd`.
// Currently, this only supports the change of syscall.O_APPEND flag.
func (c *FSContext) ChangeOpenFlag(fd uint32, flag int) error {
	f, ok := c.LookupFile(fd)
	if !ok {
		return syscall.EBADF
	} else if _, ft, err := f.CachedStat(); err != nil {
		return err
	} else if ft.Type() == fs.ModeDir {
		return syscall.EISDIR
	}

	if flag&syscall.O_APPEND != 0 {
		f.openFlag |= syscall.O_APPEND
	} else {
		f.openFlag &= ^syscall.O_APPEND
	}

	// Changing the flag while opening is not really supported well in Go. Even when using
	// syscall package, the feasibility of doing so really depends on the platform. For examples:
	//
	// 	* This appendMode (bool) cannot be changed later.
	// 	https://github.com/golang/go/blob/go1.20/src/os/file_unix.go#L60
	// 	* On Windows, re-opening it is the only way to emulate the behavior.
	// 	https://github.com/bytecodealliance/system-interface/blob/62b97f9776b86235f318c3a6e308395a1187439b/src/fs/fd_flags.rs#L196
	//
	// Therefore, here we re-open the file while keeping the file descriptor.
	// TODO: this might be improved once we have our own File type.
	if err := c.reopen(f); err != nil {
		return err
	}
	return nil
}

// LookupFile returns a file if it is in the table.
func (c *FSContext) LookupFile(fd uint32) (*FileEntry, bool) {
	f, ok := c.openedFiles.Lookup(fd)
	return f, ok
}

// Renumber assigns the file pointed by the descriptor `from` to `to`.
func (c *FSContext) Renumber(from, to uint32) error {
	fromFile, ok := c.openedFiles.Lookup(from)
	if !ok {
		return syscall.EBADF
	} else if fromFile.IsPreopen {
		return syscall.ENOTSUP
	}

	// If toFile is already open, we close it to prevent windows lock issues.
	//
	// The doc is unclear and other implementations do nothing for already-opened To FDs.
	// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_renumberfd-fd-to-fd---errno
	// https://github.com/bytecodealliance/wasmtime/blob/main/crates/wasi-common/src/snapshots/preview_1.rs#L531-L546
	if toFile, ok := c.openedFiles.Lookup(to); ok {
		if toFile.IsPreopen {
			return syscall.ENOTSUP
		}
		_ = toFile.File.Close()
	}

	c.openedFiles.Delete(from)
	c.openedFiles.InsertAt(fromFile, to)
	return nil
}

// CloseFile returns any error closing the existing file.
func (c *FSContext) CloseFile(fd uint32) error {
	f, ok := c.openedFiles.Lookup(fd)
	if !ok {
		return syscall.EBADF
	} else if f.IsPreopen {
		// WASI is the only user of pre-opens and wasi-testsuite disallows this
		// See https://github.com/WebAssembly/wasi-testsuite/issues/50
		return syscall.ENOTSUP
	}
	c.openedFiles.Delete(fd)
	return f.File.Close()
}

// Close implements api.Closer
func (c *FSContext) Close(context.Context) (err error) {
	// Close any files opened in this context
	c.openedFiles.Range(func(fd uint32, entry *FileEntry) bool {
		if e := entry.File.Close(); e != nil {
			err = e // This means err returned == the last non-nil error.
		}
		return true
	})
	// A closed FSContext cannot be reused so clear the state instead of
	// using Reset.
	c.openedFiles = FileTable{}
	return
}

// WriterForFile returns a writer for the given file descriptor or nil if not
// opened or not writeable (e.g. a directory or a file not opened for writes).
func WriterForFile(fsc *FSContext, fd uint32) (writer io.Writer) {
	if f, ok := fsc.LookupFile(fd); ok {
		writer = f.File.(io.Writer)
	}
	return
}
