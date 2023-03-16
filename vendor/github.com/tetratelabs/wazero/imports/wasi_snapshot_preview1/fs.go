package wasi_snapshot_preview1

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"math"
	pathutil "path"
	"reflect"
	"syscall"
	"unsafe"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/internal/platform"
	"github.com/tetratelabs/wazero/internal/sys"
	"github.com/tetratelabs/wazero/internal/sysfs"
	. "github.com/tetratelabs/wazero/internal/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/internal/wasm"
)

// The following interfaces are used until we finalize our own FD-scoped file.
type (
	// syncFile is implemented by os.File in file_posix.go
	syncFile interface{ Sync() error }
	// truncateFile is implemented by os.File in file_posix.go
	truncateFile interface{ Truncate(size int64) error }
)

// fdAdvise is the WASI function named FdAdviseName which provides file
// advisory information on a file descriptor.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_advisefd-fd-offset-filesize-len-filesize-advice-advice---errno
var fdAdvise = newHostFunc(
	FdAdviseName, fdAdviseFn,
	[]wasm.ValueType{i32, i64, i64, i32},
	"fd", "offset", "len", "advice",
)

func fdAdviseFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fd := uint32(params[0])
	_ = params[1]
	_ = params[2]
	advice := byte(params[3])
	fsc := mod.(*wasm.CallContext).Sys.FS()

	_, ok := fsc.LookupFile(fd)
	if !ok {
		return ErrnoBadf
	}

	switch advice {
	case FdAdviceNormal,
		FdAdviceSequential,
		FdAdviceRandom,
		FdAdviceWillNeed,
		FdAdviceDontNeed,
		FdAdviceNoReuse:
	default:
		return ErrnoInval
	}

	// FdAdvice corresponds to posix_fadvise, but it can only be supported on linux.
	// However, the purpose of the call is just to do best-effort optimization on OS kernels,
	// so just making this noop rather than returning NoSup error makes sense and doesn't affect
	// the semantics of Wasm applications.
	// TODO: invoke posix_fadvise on linux, and partially on darwin.
	// - https://gitlab.com/cznic/fileutil/-/blob/v1.1.2/fileutil_linux.go#L87-95
	// - https://github.com/bytecodealliance/system-interface/blob/62b97f9776b86235f318c3a6e308395a1187439b/src/fs/file_io_ext.rs#L430-L442
	return ErrnoSuccess
}

// fdAllocate is the WASI function named FdAllocateName which forces the
// allocation of space in a file.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_allocatefd-fd-offset-filesize-len-filesize---errno
var fdAllocate = newHostFunc(
	FdAllocateName, fdAllocateFn,
	[]wasm.ValueType{i32, i64, i64},
	"fd", "offset", "len",
)

func fdAllocateFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fd := uint32(params[0])
	offset := params[1]
	length := params[2]

	fsc := mod.(*wasm.CallContext).Sys.FS()
	f, ok := fsc.LookupFile(fd)
	if !ok {
		return ErrnoBadf
	}

	tail := int64(offset + length)
	if tail < 0 {
		return ErrnoInval
	}

	var st platform.Stat_t
	if err := f.Stat(&st); err != nil {
		return ToErrno(err)
	}

	if st.Size >= tail {
		// We already have enough space.
		return ErrnoSuccess
	}

	osf, ok := f.File.(truncateFile)
	if !ok {
		return ErrnoBadf
	}

	if err := osf.Truncate(tail); err != nil {
		return ToErrno(err)
	}
	return ErrnoSuccess
}

// fdClose is the WASI function named FdCloseName which closes a file
// descriptor.
//
// # Parameters
//
//   - fd: file descriptor to close
//
// Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: the fd was not open.
//   - ErrnoNotsup: the fs was a pre-open
//
// Note: This is similar to `close` in POSIX.
// See https://github.com/WebAssembly/WASI/blob/main/phases/snapshot/docs.md#fd_close
// and https://linux.die.net/man/3/close
var fdClose = newHostFunc(FdCloseName, fdCloseFn, []api.ValueType{i32}, "fd")

func fdCloseFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()
	fd := uint32(params[0])

	if err := fsc.CloseFile(fd); err != nil {
		return ToErrno(err)
	}
	return ErrnoSuccess
}

// fdDatasync is the WASI function named FdDatasyncName which synchronizes
// the data of a file to disk.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_datasyncfd-fd---errno
var fdDatasync = newHostFunc(FdDatasyncName, fdDatasyncFn, []api.ValueType{i32}, "fd")

func fdDatasyncFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()
	fd := uint32(params[0])

	// Check to see if the file descriptor is available
	if f, ok := fsc.LookupFile(fd); !ok {
		return ErrnoBadf
	} else if err := sysfs.FileDatasync(f.File); err != nil {
		return ToErrno(err)
	}
	return ErrnoSuccess
}

// fdFdstatGet is the WASI function named FdFdstatGetName which returns the
// attributes of a file descriptor.
//
// # Parameters
//
//   - fd: file descriptor to get the fdstat attributes data
//   - resultFdstat: offset to write the result fdstat data
//
// Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: `fd` is invalid
//   - ErrnoFault: `resultFdstat` points to an offset out of memory
//
// fdstat byte layout is 24-byte size, with the following fields:
//   - fs_filetype 1 byte: the file type
//   - fs_flags 2 bytes: the file descriptor flag
//   - 5 pad bytes
//   - fs_right_base 8 bytes: ignored as rights were removed from WASI.
//   - fs_right_inheriting 8 bytes: ignored as rights were removed from WASI.
//
// For example, with a file corresponding with `fd` was a directory (=3) opened
// with `fd_read` right (=1) and no fs_flags (=0), parameter resultFdstat=1,
// this function writes the below to api.Memory:
//
//	                uint16le   padding            uint64le                uint64le
//	       uint8 --+  +--+  +-----------+  +--------------------+  +--------------------+
//	               |  |  |  |           |  |                    |  |                    |
//	     []byte{?, 3, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0}
//	resultFdstat --^  ^-- fs_flags         ^-- fs_right_base       ^-- fs_right_inheriting
//	               |
//	               +-- fs_filetype
//
// Note: fdFdstatGet returns similar flags to `fsync(fd, F_GETFL)` in POSIX, as
// well as additional fields.
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fdstat
// and https://linux.die.net/man/3/fsync
var fdFdstatGet = newHostFunc(FdFdstatGetName, fdFdstatGetFn, []api.ValueType{i32, i32}, "fd", "result.stat")

// fdFdstatGetFn cannot currently use proxyResultParams because fdstat is larger
// than api.ValueTypeI64 (i64 == 8 bytes, but fdstat is 24).
func fdFdstatGetFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()

	fd, resultFdstat := uint32(params[0]), uint32(params[1])

	// Ensure we can write the fdstat
	buf, ok := mod.Memory().Read(resultFdstat, 24)
	if !ok {
		return ErrnoFault
	}

	var fdflags uint16
	var stat fs.FileInfo
	var err error
	if f, ok := fsc.LookupFile(fd); !ok {
		return ErrnoBadf
	} else if stat, err = f.File.Stat(); err != nil {
		return ToErrno(err)
	} else if _, ok := f.File.(io.Writer); ok {
		// TODO: maybe cache flags to open instead
		fdflags = FD_APPEND
	}

	filetype := getWasiFiletype(stat.Mode())
	writeFdstat(buf, filetype, fdflags)

	return ErrnoSuccess
}

var blockFdstat = []byte{
	FILETYPE_BLOCK_DEVICE, 0, // filetype
	0, 0, 0, 0, 0, 0, // fdflags
	0, 0, 0, 0, 0, 0, 0, 0, // fs_rights_base
	0, 0, 0, 0, 0, 0, 0, 0, // fs_rights_inheriting
}

func writeFdstat(buf []byte, filetype uint8, fdflags uint16) {
	// memory is re-used, so ensure the result is defaulted.
	copy(buf, blockFdstat)
	buf[0] = filetype
	buf[2] = byte(fdflags)
}

// fdFdstatSetFlags is the WASI function named FdFdstatSetFlagsName which
// adjusts the flags associated with a file descriptor.
var fdFdstatSetFlags = newHostFunc(FdFdstatSetFlagsName, fdFdstatSetFlagsFn, []wasm.ValueType{i32, i32}, "fd", "flags")

func fdFdstatSetFlagsFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fd, wasiFlag := uint32(params[0]), uint16(params[1])
	fsc := mod.(*wasm.CallContext).Sys.FS()

	// We can only support APPEND flag.
	if FD_DSYNC&wasiFlag != 0 || FD_NONBLOCK&wasiFlag != 0 || FD_RSYNC&wasiFlag != 0 || FD_SYNC&wasiFlag != 0 {
		return ErrnoInval
	}

	var flag int
	if FD_APPEND&wasiFlag != 0 {
		flag = syscall.O_APPEND
	}

	if err := fsc.ChangeOpenFlag(fd, flag); err != nil {
		return ToErrno(err)
	}
	return ErrnoSuccess
}

// fdFdstatSetRights will not be implemented as rights were removed from WASI.
//
// See https://github.com/bytecodealliance/wasmtime/pull/4666
var fdFdstatSetRights = stubFunction(
	FdFdstatSetRightsName,
	[]wasm.ValueType{i32, i64, i64},
	"fd", "fs_rights_base", "fs_rights_inheriting",
)

// fdFilestatGet is the WASI function named FdFilestatGetName which returns
// the stat attributes of an open file.
//
// # Parameters
//
//   - fd: file descriptor to get the filestat attributes data for
//   - resultFilestat: offset to write the result filestat data
//
// Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: `fd` is invalid
//   - ErrnoIo: could not stat `fd` on filesystem
//   - ErrnoFault: `resultFilestat` points to an offset out of memory
//
// filestat byte layout is 64-byte size, with the following fields:
//   - dev 8 bytes: the device ID of device containing the file
//   - ino 8 bytes: the file serial number
//   - filetype 1 byte: the type of the file
//   - 7 pad bytes
//   - nlink 8 bytes: number of hard links to the file
//   - size 8 bytes: for regular files, the file size in bytes. For symbolic links, the length in bytes of the pathname contained in the symbolic link
//   - atim 8 bytes: ast data access timestamp
//   - mtim 8 bytes: last data modification timestamp
//   - ctim 8 bytes: ast file status change timestamp
//
// For example, with a regular file this function writes the below to api.Memory:
//
//	                                                             uint8 --+
//		                         uint64le                uint64le        |        padding               uint64le                uint64le                         uint64le                               uint64le                             uint64le
//		                 +--------------------+  +--------------------+  |  +-----------------+  +--------------------+  +-----------------------+  +----------------------------------+  +----------------------------------+  +----------------------------------+
//		                 |                    |  |                    |  |  |                 |  |                    |  |                       |  |                                  |  |                                  |  |                                  |
//		          []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 117, 80, 0, 0, 0, 0, 0, 0, 160, 153, 212, 128, 110, 221, 35, 23, 160, 153, 212, 128, 110, 221, 35, 23, 160, 153, 212, 128, 110, 221, 35, 23}
//		resultFilestat   ^-- dev                 ^-- ino                 ^                       ^-- nlink               ^-- size                   ^-- atim                              ^-- mtim                              ^-- ctim
//		                                                                 |
//		                                                                 +-- filetype
//
// The following properties of filestat are not implemented:
//   - dev: not supported by Golang FS
//   - ino: not supported by Golang FS
//   - nlink: not supported by Golang FS, we use 1
//   - atime: not supported by Golang FS, we use mtim for this
//   - ctim: not supported by Golang FS, we use mtim for this
//
// Note: This is similar to `fstat` in POSIX.
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_filestat_getfd-fd---errno-filestat
// and https://linux.die.net/man/3/fstat
var fdFilestatGet = newHostFunc(FdFilestatGetName, fdFilestatGetFn, []api.ValueType{i32, i32}, "fd", "result.filestat")

// fdFilestatGetFn cannot currently use proxyResultParams because filestat is
// larger than api.ValueTypeI64 (i64 == 8 bytes, but filestat is 64).
func fdFilestatGetFn(_ context.Context, mod api.Module, params []uint64) Errno {
	return fdFilestatGetFunc(mod, uint32(params[0]), uint32(params[1]))
}

func fdFilestatGetFunc(mod api.Module, fd, resultBuf uint32) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()

	// Ensure we can write the filestat
	buf, ok := mod.Memory().Read(resultBuf, 64)
	if !ok {
		return ErrnoFault
	}

	f, ok := fsc.LookupFile(fd)
	if !ok {
		return ErrnoBadf
	}

	var st platform.Stat_t
	if err := f.Stat(&st); err != nil {
		return ToErrno(err)
	}

	if err := writeFilestat(buf, &st); err != nil {
		return ToErrno(err)
	}

	return ErrnoSuccess
}

func getWasiFiletype(fileMode fs.FileMode) uint8 {
	wasiFileType := FILETYPE_UNKNOWN
	if fileMode&fs.ModeDevice != 0 {
		wasiFileType = FILETYPE_BLOCK_DEVICE
	} else if fileMode&fs.ModeCharDevice != 0 {
		wasiFileType = FILETYPE_CHARACTER_DEVICE
	} else if fileMode&fs.ModeDir != 0 {
		wasiFileType = FILETYPE_DIRECTORY
	} else if fileMode&fs.ModeType == 0 {
		wasiFileType = FILETYPE_REGULAR_FILE
	} else if fileMode&fs.ModeSymlink != 0 {
		wasiFileType = FILETYPE_SYMBOLIC_LINK
	}
	return wasiFileType
}

func writeFilestat(buf []byte, st *platform.Stat_t) (err error) {
	le.PutUint64(buf, st.Dev)
	le.PutUint64(buf[8:], st.Ino)
	le.PutUint64(buf[16:], uint64(getWasiFiletype(st.Mode)))
	le.PutUint64(buf[24:], st.Nlink)
	le.PutUint64(buf[32:], uint64(st.Size))
	le.PutUint64(buf[40:], uint64(st.Atim))
	le.PutUint64(buf[48:], uint64(st.Mtim))
	le.PutUint64(buf[56:], uint64(st.Ctim))
	return
}

// fdFilestatSetSize is the WASI function named FdFilestatSetSizeName which
// adjusts the size of an open file.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_filestat_set_sizefd-fd-size-filesize---errno
var fdFilestatSetSize = newHostFunc(FdFilestatSetSizeName, fdFilestatSetSizeFn, []wasm.ValueType{i32, i64}, "fd", "size")

func fdFilestatSetSizeFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fd := uint32(params[0])
	size := uint32(params[1])

	fsc := mod.(*wasm.CallContext).Sys.FS()

	// Check to see if the file descriptor is available
	if f, ok := fsc.LookupFile(fd); !ok {
		return ErrnoBadf
	} else if truncateFile, ok := f.File.(truncateFile); !ok {
		return ErrnoBadf // possibly a fake file
	} else if err := truncateFile.Truncate(int64(size)); err != nil {
		return ToErrno(err)
	}
	return ErrnoSuccess
}

// fdFilestatSetTimes is the WASI function named functionFdFilestatSetTimes
// which adjusts the times of an open file.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_filestat_set_timesfd-fd-atim-timestamp-mtim-timestamp-fst_flags-fstflags---errno
var fdFilestatSetTimes = newHostFunc(
	FdFilestatSetTimesName, fdFilestatSetTimesFn,
	[]wasm.ValueType{i32, i64, i64, i32},
	"fd", "atim", "mtim", "fst_flags",
)

func fdFilestatSetTimesFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fd := uint32(params[0])
	fstFlags := uint16(params[3])

	sys := mod.(*wasm.CallContext).Sys
	fsc := sys.FS()

	f, ok := fsc.LookupFile(fd)
	if !ok {
		return ErrnoBadf
	}

	// Unchanging a part of time spec while executing utimes is extremely complex to add support for all platforms,
	// and actually there's an outstanding issue on Go
	// - https://github.com/golang/go/issues/32558.
	// - https://go-review.googlesource.com/c/go/+/219638 (unmerged)
	//
	// Here, we emulate the behavior for empty flag (meaning "do not change") by get the current time stamp
	// by explicitly executing File.Stat() prior to Utimes.
	var atime, mtime int64
	var nowAtime, statAtime, nowMtime, statMtime bool
	if set, now := fstFlags&FileStatAdjustFlagsAtim != 0, fstFlags&FileStatAdjustFlagsAtimNow != 0; set && now {
		return ErrnoInval
	} else if set {
		atime = int64(params[1])
	} else if now {
		nowAtime = true
	} else {
		statAtime = true
	}
	if set, now := fstFlags&FileStatAdjustFlagsMtim != 0, fstFlags&FileStatAdjustFlagsMtimNow != 0; set && now {
		return ErrnoInval
	} else if set {
		mtime = int64(params[2])
	} else if now {
		nowMtime = true
	} else {
		statMtime = true
	}

	// Handle if either parameter should be now.
	if nowAtime || nowMtime {
		now := sys.WalltimeNanos()
		if nowAtime {
			atime = now
		}
		if nowMtime {
			mtime = now
		}
	}

	// Handle if either parameter should be taken from stat.
	if statAtime || statMtime {
		// Get the current timestamp via Stat in order to un-change after calling FS.Utimes().
		var st platform.Stat_t
		if err := f.Stat(&st); err != nil {
			return ToErrno(err)
		}
		if statAtime {
			atime = st.Atim
		}
		if statMtime {
			mtime = st.Mtim
		}
	}

	// TODO: this should work against the file descriptor not its last name!
	if err := f.FS.Utimes(f.Name, atime, mtime); err != nil {
		return ToErrno(err)
	}
	return ErrnoSuccess
}

// fdPread is the WASI function named FdPreadName which reads from a file
// descriptor, without using and updating the file descriptor's offset.
//
// Except for handling offset, this implementation is identical to fdRead.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_preadfd-fd-iovs-iovec_array-offset-filesize---errno-size
var fdPread = newHostFunc(
	FdPreadName, fdPreadFn,
	[]api.ValueType{i32, i32, i32, i64, i32},
	"fd", "iovs", "iovs_len", "offset", "result.nread",
)

func fdPreadFn(_ context.Context, mod api.Module, params []uint64) Errno {
	return fdReadOrPread(mod, params, true)
}

// fdPrestatGet is the WASI function named FdPrestatGetName which returns
// the prestat data of a file descriptor.
//
// # Parameters
//
//   - fd: file descriptor to get the prestat
//   - resultPrestat: offset to write the result prestat data
//
// Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: `fd` is invalid or the `fd` is not a pre-opened directory
//   - ErrnoFault: `resultPrestat` points to an offset out of memory
//
// prestat byte layout is 8 bytes, beginning with an 8-bit tag and 3 pad bytes.
// The only valid tag is `prestat_dir`, which is tag zero. This simplifies the
// byte layout to 4 empty bytes followed by the uint32le encoded path length.
//
// For example, the directory name corresponding with `fd` was "/tmp" and
// parameter resultPrestat=1, this function writes the below to api.Memory:
//
//	                   padding   uint32le
//	        uint8 --+  +-----+  +--------+
//	                |  |     |  |        |
//	      []byte{?, 0, 0, 0, 0, 4, 0, 0, 0, ?}
//	resultPrestat --^           ^
//	          tag --+           |
//	                            +-- size in bytes of the string "/tmp"
//
// See fdPrestatDirName and
// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#prestat
var fdPrestatGet = newHostFunc(FdPrestatGetName, fdPrestatGetFn, []api.ValueType{i32, i32}, "fd", "result.prestat")

func fdPrestatGetFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()
	fd, resultPrestat := uint32(params[0]), uint32(params[1])

	name, errno := preopenPath(fsc, fd)
	if errno != ErrnoSuccess {
		return errno
	}

	// Upper 32-bits are zero because...
	// * Zero-value 8-bit tag, and 3-byte zero-value padding
	prestat := uint64(len(name) << 32)
	if !mod.Memory().WriteUint64Le(resultPrestat, prestat) {
		return ErrnoFault
	}
	return ErrnoSuccess
}

// fdPrestatDirName is the WASI function named FdPrestatDirNameName which
// returns the path of the pre-opened directory of a file descriptor.
//
// # Parameters
//
//   - fd: file descriptor to get the path of the pre-opened directory
//   - path: offset in api.Memory to write the result path
//   - pathLen: count of bytes to write to `path`
//   - This should match the uint32le fdPrestatGet writes to offset
//     `resultPrestat`+4
//
// Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: `fd` is invalid
//   - ErrnoFault: `path` points to an offset out of memory
//   - ErrnoNametoolong: `pathLen` is longer than the actual length of the result
//
// For example, the directory name corresponding with `fd` was "/tmp" and
// # Parameters path=1 pathLen=4 (correct), this function will write the below to
// api.Memory:
//
//	               pathLen
//	           +--------------+
//	           |              |
//	[]byte{?, '/', 't', 'm', 'p', ?}
//	    path --^
//
// See fdPrestatGet
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fd_prestat_dir_name
var fdPrestatDirName = newHostFunc(
	FdPrestatDirNameName, fdPrestatDirNameFn,
	[]api.ValueType{i32, i32, i32},
	"fd", "result.path", "result.path_len",
)

func fdPrestatDirNameFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()
	fd, path, pathLen := uint32(params[0]), uint32(params[1]), uint32(params[2])

	name, errno := preopenPath(fsc, fd)
	if errno != ErrnoSuccess {
		return errno
	}

	// Some runtimes may have another semantics. See /RATIONALE.md
	if uint32(len(name)) < pathLen {
		return ErrnoNametoolong
	}

	if !mod.Memory().Write(path, []byte(name)[:pathLen]) {
		return ErrnoFault
	}
	return ErrnoSuccess
}

// fdPwrite is the WASI function named FdPwriteName which writes to a file
// descriptor, without using and updating the file descriptor's offset.
//
// Except for handling offset, this implementation is identical to fdWrite.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_pwritefd-fd-iovs-ciovec_array-offset-filesize---errno-size
var fdPwrite = newHostFunc(
	FdPwriteName, fdPwriteFn,
	[]api.ValueType{i32, i32, i32, i64, i32},
	"fd", "iovs", "iovs_len", "offset", "result.nwritten",
)

func fdPwriteFn(_ context.Context, mod api.Module, params []uint64) Errno {
	return fdWriteOrPwrite(mod, params, true)
}

// fdRead is the WASI function named FdReadName which reads from a file
// descriptor.
//
// # Parameters
//
//   - fd: an opened file descriptor to read data from
//   - iovs: offset in api.Memory to read offset, size pairs representing where
//     to write file data
//   - Both offset and length are encoded as uint32le
//   - iovsCount: count of memory offset, size pairs to read sequentially
//     starting at iovs
//   - resultNread: offset in api.Memory to write the number of bytes read
//
// Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: `fd` is invalid
//   - ErrnoFault: `iovs` or `resultNread` point to an offset out of memory
//   - ErrnoIo: a file system error
//
// For example, this function needs to first read `iovs` to determine where
// to write contents. If parameters iovs=1 iovsCount=2, this function reads two
// offset/length pairs from api.Memory:
//
//	                  iovs[0]                  iovs[1]
//	          +---------------------+   +--------------------+
//	          | uint32le    uint32le|   |uint32le    uint32le|
//	          +---------+  +--------+   +--------+  +--------+
//	          |         |  |        |   |        |  |        |
//	[]byte{?, 18, 0, 0, 0, 4, 0, 0, 0, 23, 0, 0, 0, 2, 0, 0, 0, ?... }
//	   iovs --^            ^            ^           ^
//	          |            |            |           |
//	 offset --+   length --+   offset --+  length --+
//
// If the contents of the `fd` parameter was "wazero" (6 bytes) and parameter
// resultNread=26, this function writes the below to api.Memory:
//
//	                    iovs[0].length        iovs[1].length
//	                   +--------------+       +----+       uint32le
//	                   |              |       |    |      +--------+
//	[]byte{ 0..16, ?, 'w', 'a', 'z', 'e', ?, 'r', 'o', ?, 6, 0, 0, 0 }
//	  iovs[0].offset --^                      ^           ^
//	                         iovs[1].offset --+           |
//	                                        resultNread --+
//
// Note: This is similar to `readv` in POSIX. https://linux.die.net/man/3/readv
//
// See fdWrite
// and https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fd_read
var fdRead = newHostFunc(
	FdReadName, fdReadFn,
	[]api.ValueType{i32, i32, i32, i32},
	"fd", "iovs", "iovs_len", "result.nread",
)

func fdReadFn(_ context.Context, mod api.Module, params []uint64) Errno {
	return fdReadOrPread(mod, params, false)
}

func fdReadOrPread(mod api.Module, params []uint64, isPread bool) Errno {
	mem := mod.Memory()
	fsc := mod.(*wasm.CallContext).Sys.FS()

	fd := uint32(params[0])

	r, ok := fsc.LookupFile(fd)
	if !ok {
		return ErrnoBadf
	}

	var reader io.Reader = r.File

	iovs := uint32(params[1])
	iovsCount := uint32(params[2])

	var resultNread uint32
	if isPread {
		offset := int64(params[3])
		reader = sysfs.ReaderAtOffset(r.File, offset)
		resultNread = uint32(params[4])
	} else {
		resultNread = uint32(params[3])
	}

	var nread uint32
	iovsStop := iovsCount << 3 // iovsCount * 8
	iovsBuf, ok := mem.Read(iovs, iovsStop)
	if !ok {
		return ErrnoFault
	}

	for iovsPos := uint32(0); iovsPos < iovsStop; iovsPos += 8 {
		offset := le.Uint32(iovsBuf[iovsPos:])
		l := le.Uint32(iovsBuf[iovsPos+4:])

		b, ok := mem.Read(offset, l)
		if !ok {
			return ErrnoFault
		}

		n, err := reader.Read(b)
		nread += uint32(n)

		shouldContinue, errno := fdRead_shouldContinueRead(uint32(n), l, err)
		if errno != ErrnoSuccess {
			return errno
		} else if !shouldContinue {
			break
		}
	}
	if !mem.WriteUint32Le(resultNread, nread) {
		return ErrnoFault
	} else {
		return ErrnoSuccess
	}
}

// fdRead_shouldContinueRead decides whether to continue reading the next iovec
// based on the amount read (n/l) and a possible error returned from io.Reader.
//
// Note: When there are both bytes read (n) and an error, this continues.
// See /RATIONALE.md "Why ignore the error returned by io.Reader when n > 1?"
func fdRead_shouldContinueRead(n, l uint32, err error) (bool, Errno) {
	if errors.Is(err, io.EOF) {
		return false, ErrnoSuccess // EOF isn't an error, and we shouldn't continue.
	} else if err != nil && n == 0 {
		return false, ErrnoIo
	} else if err != nil {
		return false, ErrnoSuccess // Allow the caller to process n bytes.
	}
	// Continue reading, unless there's a partial read or nothing to read.
	return n == l && n != 0, ErrnoSuccess
}

// fdReaddir is the WASI function named FdReaddirName which reads directory
// entries from a directory.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_readdirfd-fd-buf-pointeru8-buf_len-size-cookie-dircookie---errno-size
var fdReaddir = newHostFunc(
	FdReaddirName, fdReaddirFn,
	[]wasm.ValueType{i32, i32, i32, i64, i32},
	"fd", "buf", "buf_len", "cookie", "result.bufused",
)

func fdReaddirFn(_ context.Context, mod api.Module, params []uint64) Errno {
	mem := mod.Memory()
	fsc := mod.(*wasm.CallContext).Sys.FS()

	fd := uint32(params[0])
	buf := uint32(params[1])
	bufLen := uint32(params[2])
	// We control the value of the cookie, and it should never be negative.
	// However, we coerce it to signed to ensure the caller doesn't manipulate
	// it in such a way that becomes negative.
	cookie := int64(params[3])
	resultBufused := uint32(params[4])

	// The bufLen must be enough to write a dirent. Otherwise, the caller can't
	// read what the next cookie is.
	if bufLen < DirentSize {
		return ErrnoInval
	}

	// Validate the FD is a directory
	rd, dir, errno := openedDir(fsc, fd)
	if errno != ErrnoSuccess {
		return errno
	}

	if cookie == 0 && dir.CountRead > 0 {
		// This means that there was a previous call to the dir, but cookie is reset.
		// This happens when the program calls rewinddir, for example:
		// https://github.com/WebAssembly/wasi-libc/blob/659ff414560721b1660a19685110e484a081c3d4/libc-bottom-half/cloudlibc/src/libc/dirent/rewinddir.c#L10-L12
		//
		// Since we cannot unwind fs.ReadDirFile results, we re-open while keeping the same file descriptor.
		f, err := fsc.ReOpenDir(fd)
		if err != nil {
			return ToErrno(err)
		}
		rd, dir = f.File, f.ReadDir
	}

	// First, determine the maximum directory entries that can be encoded as
	// dirents. The total size is DirentSize(24) + nameSize, for each file.
	// Since a zero-length file name is invalid, the minimum size entry is
	// 25 (DirentSize + 1 character).
	maxDirEntries := int(bufLen/DirentSize + 1)

	// While unlikely maxDirEntries will fit into bufLen, add one more just in
	// case, as we need to know if we hit the end of the directory or not to
	// write the correct bufused (e.g. == bufLen unless EOF).
	//	>> If less than the size of the read buffer, the end of the
	//	>> directory has been reached.
	maxDirEntries += 1

	// The host keeps state for any unread entries from the prior call because
	// we cannot seek to a previous directory position. Collect these entries.
	dirents, errno := lastDirents(dir, cookie)
	if errno != ErrnoSuccess {
		return errno
	}

	// Add entries for dot and dot-dot as wasi-testsuite requires them.
	if cookie == 0 && dirents == nil {
		var err error
		if f, ok := fsc.LookupFile(fd); !ok {
			return ErrnoBadf
		} else if dirents, err = dotDirents(f); err != nil {
			return ToErrno(err)
		}
		dir.Dirents = dirents
		dir.CountRead = 2 // . and ..
	}

	// Check if we have maxDirEntries, and read more from the FS as needed.
	if entryCount := len(dirents); entryCount < maxDirEntries {
		// Note: platform.Readdir does not return io.EOF as it is
		// inconsistently returned (e.g. darwin does, but linux doesn't).
		l, err := platform.Readdir(rd, maxDirEntries-entryCount)
		if errno = ToErrno(err); errno != ErrnoSuccess {
			return errno
		}

		// Zero length read is possible on an empty or exhausted directory.
		if len(l) > 0 {
			dir.CountRead += uint64(len(l))
			dirents = append(dirents, l...)
			// Replace the cache with up to maxDirEntries, starting at cookie.
			dir.Dirents = dirents
		}
	}

	// Determine how many dirents we can write, excluding a potentially
	// truncated entry.
	bufused, direntCount, writeTruncatedEntry := maxDirents(dirents, bufLen)

	// Now, write entries to the underlying buffer.
	if bufused > 0 {

		// d_next is the index of the next file in the list, so it should
		// always be one higher than the requested cookie.
		d_next := uint64(cookie + 1)
		// ^^ yes this can overflow to negative, which means our implementation
		// doesn't support writing greater than max int64 entries.

		buf, ok := mem.Read(buf, bufused)
		if !ok {
			return ErrnoFault
		}

		writeDirents(dirents, direntCount, writeTruncatedEntry, buf, d_next)
	}

	if !mem.WriteUint32Le(resultBufused, bufused) {
		return ErrnoFault
	}
	return ErrnoSuccess
}

// dotDirents returns "." and "..", where "." because wasi-testsuite does inode
// validation.
func dotDirents(f *sys.FileEntry) ([]*platform.Dirent, error) {
	dotIno, ft, err := f.CachedStat()
	if err != nil {
		return nil, err
	} else if ft.Type() != fs.ModeDir {
		return nil, syscall.ENOTDIR
	}
	dotDotIno := uint64(0)
	if !f.IsPreopen && f.Name != "." {
		var st platform.Stat_t
		if err = f.FS.Stat(pathutil.Dir(f.Name), &st); err != nil {
			return nil, err
		}
		dotDotIno = st.Ino
	}
	return []*platform.Dirent{
		{Name: ".", Ino: dotIno, Type: fs.ModeDir},
		{Name: "..", Ino: dotDotIno, Type: fs.ModeDir},
	}, nil
}

const largestDirent = int64(math.MaxUint32 - DirentSize)

// lastDirents is broken out from fdReaddirFn for testability.
func lastDirents(dir *sys.ReadDir, cookie int64) (dirents []*platform.Dirent, errno Errno) {
	if cookie < 0 {
		errno = ErrnoInval // invalid as we will never send a negative cookie.
		return
	}

	entryCount := int64(len(dir.Dirents))
	if entryCount == 0 { // there was no prior call
		if cookie != 0 {
			errno = ErrnoInval // invalid as we haven't sent that cookie
		}
		return
	}

	// Get the first absolute position in our window of results
	firstPos := int64(dir.CountRead) - entryCount
	cookiePos := cookie - firstPos

	switch {
	case cookiePos < 0: // cookie is asking for results outside our window.
		errno = ErrnoNosys // we can't implement directory seeking backwards.
	case cookiePos > entryCount:
		errno = ErrnoInval // invalid as we read that far, yet.
	case cookiePos > 0: // truncate so to avoid large lists.
		dirents = dir.Dirents[cookiePos:]
	default:
		dirents = dir.Dirents
	}
	if len(dirents) == 0 {
		dirents = nil
	}
	return
}

// maxDirents returns the maximum count and total entries that can fit in
// maxLen bytes.
//
// truncatedEntryLen is the amount of bytes past bufLen needed to write the
// next entry. We have to return bufused == bufLen unless the directory is
// exhausted.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fd_readdir
// See https://github.com/WebAssembly/wasi-libc/blob/659ff414560721b1660a19685110e484a081c3d4/libc-bottom-half/cloudlibc/src/libc/dirent/readdir.c#L44
func maxDirents(entries []*platform.Dirent, bufLen uint32) (bufused, direntCount uint32, writeTruncatedEntry bool) {
	lenRemaining := bufLen
	for _, e := range entries {
		if lenRemaining < DirentSize {
			// We don't have enough space in bufLen for another struct,
			// entry. A caller who wants more will retry.

			// bufused == bufLen means more entries exist, which is the case
			// when the dirent is larger than bytes remaining.
			bufused = bufLen
			break
		}

		// use int64 to guard against huge filenames
		nameLen := int64(len(e.Name))
		var entryLen uint32

		// Check to see if DirentSize + nameLen overflows, or if it would be
		// larger than possible to encode.
		if el := int64(DirentSize) + nameLen; el < 0 || el > largestDirent {
			// panic, as testing is difficult. ex we would have to extract a
			// function to get size of a string or allocate a 2^32 size one!
			panic("invalid filename: too large")
		} else { // we know this can fit into a uint32
			entryLen = uint32(el)
		}

		if entryLen > lenRemaining {
			// We haven't room to write the entry, and docs say to write the
			// header. This helps especially when there is an entry with a very
			// long filename. Ex if bufLen is 4096 and the filename is 4096,
			// we need to write DirentSize(24) + 4096 bytes to write the entry.
			// In this case, we only write up to DirentSize(24) to allow the
			// caller to resize.

			// bufused == bufLen means more entries exist, which is the case
			// when the next entry is larger than bytes remaining.
			bufused = bufLen

			// We do have enough space to write the header, this value will be
			// passed on to writeDirents to only write the header for this entry.
			writeTruncatedEntry = true
			break
		}

		// This won't go negative because we checked entryLen <= lenRemaining.
		lenRemaining -= entryLen
		bufused += entryLen
		direntCount++
	}
	return
}

// writeDirents writes the directory entries to the buffer, which is pre-sized
// based on maxDirents.	truncatedEntryLen means write one past entryCount,
// without its name. See maxDirents for why
func writeDirents(
	dirents []*platform.Dirent,
	direntCount uint32,
	writeTruncatedEntry bool,
	buf []byte,
	d_next uint64,
) {
	pos, i := uint32(0), uint32(0)
	for ; i < direntCount; i++ {
		e := dirents[i]
		nameLen := uint32(len(e.Name))

		writeDirent(buf[pos:], d_next, e.Ino, nameLen, e.IsDir())
		pos += DirentSize

		copy(buf[pos:], e.Name)
		pos += nameLen
		d_next++
	}

	if !writeTruncatedEntry {
		return
	}

	// Write a dirent without its name
	dirent := make([]byte, DirentSize)
	e := dirents[i]
	writeDirent(dirent, d_next, e.Ino, uint32(len(e.Name)), e.IsDir())

	// Potentially truncate it
	copy(buf[pos:], dirent)
}

// writeDirent writes DirentSize bytes
func writeDirent(buf []byte, dNext uint64, ino uint64, dNamlen uint32, dType bool) {
	le.PutUint64(buf, dNext)        // d_next
	le.PutUint64(buf[8:], ino)      // d_ino
	le.PutUint32(buf[16:], dNamlen) // d_namlen

	filetype := FILETYPE_REGULAR_FILE
	if dType {
		filetype = FILETYPE_DIRECTORY
	}
	le.PutUint32(buf[20:], uint32(filetype)) //  d_type
}

// openedDir returns the directory and ErrnoSuccess if the fd points to a readable directory.
func openedDir(fsc *sys.FSContext, fd uint32) (fs.File, *sys.ReadDir, Errno) {
	if f, ok := fsc.LookupFile(fd); !ok {
		return nil, nil, ErrnoBadf
	} else if _, ft, err := f.CachedStat(); err != nil {
		return nil, nil, ToErrno(err)
	} else if ft.Type() != fs.ModeDir {
		// fd_readdir docs don't indicate whether to return ErrnoNotdir or
		// ErrnoBadf. It has been noticed that rust will crash on ErrnoNotdir,
		// and POSIX C ref seems to not return this, so we don't either.
		//
		// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fd_readdir
		// and https://en.wikibooks.org/wiki/C_Programming/POSIX_Reference/dirent.h
		return nil, nil, ErrnoBadf
	} else {
		if f.ReadDir == nil {
			f.ReadDir = &sys.ReadDir{}
		}
		return f.File, f.ReadDir, ErrnoSuccess
	}
}

// fdRenumber is the WASI function named FdRenumberName which atomically
// replaces a file descriptor by renumbering another file descriptor.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_renumberfd-fd-to-fd---errno
var fdRenumber = newHostFunc(FdRenumberName, fdRenumberFn, []wasm.ValueType{i32, i32}, "fd", "to")

func fdRenumberFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()

	from := uint32(params[0])
	to := uint32(params[1])

	if err := fsc.Renumber(from, to); err != nil {
		return ToErrno(err)
	}
	return ErrnoSuccess
}

// fdSeek is the WASI function named FdSeekName which moves the offset of a
// file descriptor.
//
// # Parameters
//
//   - fd: file descriptor to move the offset of
//   - offset: signed int64, which is encoded as uint64, input argument to
//     `whence`, which results in a new offset
//   - whence: operator that creates the new offset, given `offset` bytes
//   - If io.SeekStart, new offset == `offset`.
//   - If io.SeekCurrent, new offset == existing offset + `offset`.
//   - If io.SeekEnd, new offset == file size of `fd` + `offset`.
//   - resultNewoffset: offset in api.Memory to write the new offset to,
//     relative to start of the file
//
// Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: `fd` is invalid
//   - ErrnoFault: `resultNewoffset` points to an offset out of memory
//   - ErrnoInval: `whence` is an invalid value
//   - ErrnoIo: a file system error
//
// For example, if fd 3 is a file with offset 0, and parameters fd=3, offset=4,
// whence=0 (=io.SeekStart), resultNewOffset=1, this function writes the below
// to api.Memory:
//
//	                         uint64le
//	                  +--------------------+
//	                  |                    |
//	        []byte{?, 4, 0, 0, 0, 0, 0, 0, 0, ? }
//	resultNewoffset --^
//
// Note: This is similar to `lseek` in POSIX. https://linux.die.net/man/3/lseek
//
// See io.Seeker
// and https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fd_seek
var fdSeek = newHostFunc(
	FdSeekName, fdSeekFn,
	[]api.ValueType{i32, i64, i32, i32},
	"fd", "offset", "whence", "result.newoffset",
)

func fdSeekFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()
	fd := uint32(params[0])
	offset := params[1]
	whence := uint32(params[2])
	resultNewoffset := uint32(params[3])

	var seeker io.Seeker
	// Check to see if the file descriptor is available
	if f, ok := fsc.LookupFile(fd); !ok {
		return ErrnoBadf
		// fs.FS doesn't declare io.Seeker, but implementations such as os.File implement it.
	} else if _, ft, err := f.CachedStat(); err != nil {
		return ToErrno(err)
	} else if ft.Type() == fs.ModeDir {
		return ErrnoBadf
	} else if seeker, ok = f.File.(io.Seeker); !ok {
		return ErrnoBadf
	}

	if whence > io.SeekEnd /* exceeds the largest valid whence */ {
		return ErrnoInval
	}

	newOffset, err := seeker.Seek(int64(offset), int(whence))
	if err != nil {
		return ToErrno(err)
	}

	if !mod.Memory().WriteUint64Le(resultNewoffset, uint64(newOffset)) {
		return ErrnoFault
	}
	return ErrnoSuccess
}

// fdSync is the WASI function named FdSyncName which synchronizes the data
// and metadata of a file to disk.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_syncfd-fd---errno
var fdSync = newHostFunc(FdSyncName, fdSyncFn, []api.ValueType{i32}, "fd")

func fdSyncFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()
	fd := uint32(params[0])

	// Check to see if the file descriptor is available
	if f, ok := fsc.LookupFile(fd); !ok {
		return ErrnoBadf
	} else if syncFile, ok := f.File.(syncFile); !ok {
		return ErrnoBadf // possibly a fake file
	} else if err := syncFile.Sync(); err != nil {
		return ToErrno(err)
	}
	return ErrnoSuccess
}

// fdTell is the WASI function named FdTellName which returns the current
// offset of a file descriptor.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-fd_tellfd-fd---errno-filesize
var fdTell = newHostFunc(FdTellName, fdTellFn, []api.ValueType{i32, i32}, "fd", "result.offset")

func fdTellFn(ctx context.Context, mod api.Module, params []uint64) Errno {
	fd := params[0]
	offset := uint64(0)
	whence := uint64(io.SeekCurrent)
	resultNewoffset := params[1]

	fdSeekParams := []uint64{fd, offset, whence, resultNewoffset}
	return fdSeekFn(ctx, mod, fdSeekParams)
}

// fdWrite is the WASI function named FdWriteName which writes to a file
// descriptor.
//
// # Parameters
//
//   - fd: an opened file descriptor to write data to
//   - iovs: offset in api.Memory to read offset, size pairs representing the
//     data to write to `fd`
//   - Both offset and length are encoded as uint32le.
//   - iovsCount: count of memory offset, size pairs to read sequentially
//     starting at iovs
//   - resultNwritten: offset in api.Memory to write the number of bytes
//     written
//
// Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: `fd` is invalid
//   - ErrnoFault: `iovs` or `resultNwritten` point to an offset out of memory
//   - ErrnoIo: a file system error
//
// For example, this function needs to first read `iovs` to determine what to
// write to `fd`. If parameters iovs=1 iovsCount=2, this function reads two
// offset/length pairs from api.Memory:
//
//	                  iovs[0]                  iovs[1]
//	          +---------------------+   +--------------------+
//	          | uint32le    uint32le|   |uint32le    uint32le|
//	          +---------+  +--------+   +--------+  +--------+
//	          |         |  |        |   |        |  |        |
//	[]byte{?, 18, 0, 0, 0, 4, 0, 0, 0, 23, 0, 0, 0, 2, 0, 0, 0, ?... }
//	   iovs --^            ^            ^           ^
//	          |            |            |           |
//	 offset --+   length --+   offset --+  length --+
//
// This function reads those chunks api.Memory into the `fd` sequentially.
//
//	                    iovs[0].length        iovs[1].length
//	                   +--------------+       +----+
//	                   |              |       |    |
//	[]byte{ 0..16, ?, 'w', 'a', 'z', 'e', ?, 'r', 'o', ? }
//	  iovs[0].offset --^                      ^
//	                         iovs[1].offset --+
//
// Since "wazero" was written, if parameter resultNwritten=26, this function
// writes the below to api.Memory:
//
//	                   uint32le
//	                  +--------+
//	                  |        |
//	[]byte{ 0..24, ?, 6, 0, 0, 0', ? }
//	 resultNwritten --^
//
// Note: This is similar to `writev` in POSIX. https://linux.die.net/man/3/writev
//
// See fdRead
// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#ciovec
// and https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#fd_write
var fdWrite = newHostFunc(
	FdWriteName, fdWriteFn,
	[]api.ValueType{i32, i32, i32, i32},
	"fd", "iovs", "iovs_len", "result.nwritten",
)

func fdWriteFn(_ context.Context, mod api.Module, params []uint64) Errno {
	return fdWriteOrPwrite(mod, params, false)
}

func fdWriteOrPwrite(mod api.Module, params []uint64, isPwrite bool) Errno {
	mem := mod.Memory()
	fsc := mod.(*wasm.CallContext).Sys.FS()

	fd := uint32(params[0])
	iovs := uint32(params[1])
	iovsCount := uint32(params[2])

	var resultNwritten uint32
	var writer io.Writer
	if f, ok := fsc.LookupFile(fd); !ok {
		return ErrnoBadf
	} else if isPwrite {
		offset := int64(params[3])
		writer = sysfs.WriterAtOffset(f.File, offset)
		resultNwritten = uint32(params[4])
	} else {
		writer = f.File.(io.Writer)
		resultNwritten = uint32(params[3])
	}

	var err error
	var nwritten uint32
	iovsStop := iovsCount << 3 // iovsCount * 8
	iovsBuf, ok := mem.Read(iovs, iovsStop)
	if !ok {
		return ErrnoFault
	}

	for iovsPos := uint32(0); iovsPos < iovsStop; iovsPos += 8 {
		offset := le.Uint32(iovsBuf[iovsPos:])
		l := le.Uint32(iovsBuf[iovsPos+4:])

		var n int
		if writer == io.Discard { // special-case default
			n = int(l)
		} else {
			b, ok := mem.Read(offset, l)
			if !ok {
				return ErrnoFault
			}
			n, err = writer.Write(b)
			if err != nil {
				return ToErrno(err)
			}
		}
		nwritten += uint32(n)
	}

	if !mod.Memory().WriteUint32Le(resultNwritten, nwritten) {
		return ErrnoFault
	}
	return ErrnoSuccess
}

// pathCreateDirectory is the WASI function named PathCreateDirectoryName which
// creates a directory.
//
// # Parameters
//
//   - fd: file descriptor of a directory that `path` is relative to
//   - path: offset in api.Memory to read the path string from
//   - pathLen: length of `path`
//
// # Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: `fd` is invalid
//   - ErrnoNoent: `path` does not exist.
//   - ErrnoNotdir: `path` is a file
//
// # Notes
//   - This is similar to mkdirat in POSIX.
//     See https://linux.die.net/man/2/mkdirat
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_create_directoryfd-fd-path-string---errno
var pathCreateDirectory = newHostFunc(
	PathCreateDirectoryName, pathCreateDirectoryFn,
	[]wasm.ValueType{i32, i32, i32},
	"fd", "path", "path_len",
)

func pathCreateDirectoryFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()

	dirFD := uint32(params[0])
	path := uint32(params[1])
	pathLen := uint32(params[2])

	preopen, pathName, errno := atPath(fsc, mod.Memory(), dirFD, path, pathLen)
	if errno != ErrnoSuccess {
		return errno
	}

	if err := preopen.Mkdir(pathName, 0o700); err != nil {
		return ToErrno(err)
	}

	return ErrnoSuccess
}

// pathFilestatGet is the WASI function named PathFilestatGetName which
// returns the stat attributes of a file or directory.
//
// # Parameters
//
//   - fd: file descriptor of the folder to look in for the path
//   - flags: flags determining the method of how paths are resolved
//   - path: path under fd to get the filestat attributes data for
//   - path_len: length of the path that was given
//   - resultFilestat: offset to write the result filestat data
//
// Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: `fd` is invalid
//   - ErrnoNotdir: `fd` points to a file not a directory
//   - ErrnoIo: could not stat `fd` on filesystem
//   - ErrnoInval: the path contained "../"
//   - ErrnoNametoolong: `path` + `path_len` is out of memory
//   - ErrnoFault: `resultFilestat` points to an offset out of memory
//   - ErrnoNoent: could not find the path
//
// The rest of this implementation matches that of fdFilestatGet, so is not
// repeated here.
//
// Note: This is similar to `fstatat` in POSIX.
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_filestat_getfd-fd-flags-lookupflags-path-string---errno-filestat
// and https://linux.die.net/man/2/fstatat
var pathFilestatGet = newHostFunc(
	PathFilestatGetName, pathFilestatGetFn,
	[]api.ValueType{i32, i32, i32, i32, i32},
	"fd", "flags", "path", "path_len", "result.filestat",
)

func pathFilestatGetFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()

	dirFD := uint32(params[0])

	// TODO: flags is a lookupflags and it only has one bit: symlink_follow
	// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#lookupflags
	_ /* flags */ = uint32(params[1])

	path := uint32(params[2])
	pathLen := uint32(params[3])

	preopen, pathName, errno := atPath(fsc, mod.Memory(), dirFD, path, pathLen)
	if errno != ErrnoSuccess {
		return errno
	}

	// Stat the file without allocating a file descriptor
	var st platform.Stat_t
	if err := preopen.Stat(pathName, &st); err != nil {
		return ToErrno(err)
	}

	// Write the stat result to memory
	resultBuf := uint32(params[4])
	buf, ok := mod.Memory().Read(resultBuf, 64)
	if !ok {
		return ErrnoFault
	}

	if err := writeFilestat(buf, &st); err != nil {
		return ToErrno(err)
	}
	return ErrnoSuccess
}

// pathFilestatSetTimes is the WASI function named PathFilestatSetTimesName
// which adjusts the timestamps of a file or directory.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_filestat_set_timesfd-fd-flags-lookupflags-path-string-atim-timestamp-mtim-timestamp-fst_flags-fstflags---errno
var pathFilestatSetTimes = stubFunction(
	PathFilestatSetTimesName,
	[]wasm.ValueType{i32, i32, i32, i32, i64, i64, i32},
	"fd", "flags", "path", "path_len", "atim", "mtim", "fst_flags",
)

// pathLink is the WASI function named PathLinkName which adjusts the
// timestamps of a file or directory.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#path_link
var pathLink = newHostFunc(
	PathLinkName, pathLinkFn,
	[]wasm.ValueType{i32, i32, i32, i32, i32, i32, i32},
	"old_fd", "old_flags", "old_path", "old_path_len", "new_fd", "new_path", "new_path_len",
)

func pathLinkFn(_ context.Context, mod api.Module, params []uint64) Errno {
	mem := mod.Memory()
	fsc := mod.(*wasm.CallContext).Sys.FS()

	oldFd := uint32(params[0])
	// TODO: use old_flags?
	_ = uint32(params[1])
	oldPath := uint32(params[2])
	oldPathLen := uint32(params[3])

	oldFS, oldName, errno := atPath(fsc, mem, oldFd, oldPath, oldPathLen)
	if errno != ErrnoSuccess {
		return errno
	}

	newFd := uint32(params[4])
	newPath := uint32(params[5])
	newPathLen := uint32(params[6])

	newFS, newName, errno := atPath(fsc, mem, newFd, newPath, newPathLen)
	if errno != ErrnoSuccess {
		return errno
	}

	if oldFS != newFS { // TODO: handle link across filesystems
		return ErrnoNosys
	}

	if err := oldFS.Link(oldName, newName); err != nil {
		return ToErrno(err)
	}
	return ErrnoSuccess
}

// pathOpen is the WASI function named PathOpenName which opens a file or
// directory. This returns ErrnoBadf if the fd is invalid.
//
// # Parameters
//
//   - fd: file descriptor of a directory that `path` is relative to
//   - dirflags: flags to indicate how to resolve `path`
//   - path: offset in api.Memory to read the path string from
//   - pathLen: length of `path`
//   - oFlags: open flags to indicate the method by which to open the file
//   - fsRightsBase: ignored as rights were removed from WASI.
//   - fsRightsInheriting: ignored as rights were removed from WASI.
//     created file descriptor for `path`
//   - fdFlags: file descriptor flags
//   - resultOpenedFd: offset in api.Memory to write the newly created file
//     descriptor to.
//   - The result FD value is guaranteed to be less than 2**31
//
// Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: `fd` is invalid
//   - ErrnoFault: `resultOpenedFd` points to an offset out of memory
//   - ErrnoNoent: `path` does not exist.
//   - ErrnoExist: `path` exists, while `oFlags` requires that it must not.
//   - ErrnoNotdir: `path` is not a directory, while `oFlags` requires it.
//   - ErrnoIo: a file system error
//
// For example, this function needs to first read `path` to determine the file
// to open. If parameters `path` = 1, `pathLen` = 6, and the path is "wazero",
// pathOpen reads the path from api.Memory:
//
//	                pathLen
//	            +------------------------+
//	            |                        |
//	[]byte{ ?, 'w', 'a', 'z', 'e', 'r', 'o', ?... }
//	     path --^
//
// Then, if parameters resultOpenedFd = 8, and this function opened a new file
// descriptor 5 with the given flags, this function writes the below to
// api.Memory:
//
//	                  uint32le
//	                 +--------+
//	                 |        |
//	[]byte{ 0..6, ?, 5, 0, 0, 0, ?}
//	resultOpenedFd --^
//
// # Notes
//   - This is similar to `openat` in POSIX. https://linux.die.net/man/3/openat
//   - The returned file descriptor is not guaranteed to be the lowest-number
//
// See https://github.com/WebAssembly/WASI/blob/main/phases/snapshot/docs.md#path_open
var pathOpen = newHostFunc(
	PathOpenName, pathOpenFn,
	[]api.ValueType{i32, i32, i32, i32, i32, i64, i64, i32, i32},
	"fd", "dirflags", "path", "path_len", "oflags", "fs_rights_base", "fs_rights_inheriting", "fdflags", "result.opened_fd",
)

func pathOpenFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()

	preopenFD := uint32(params[0])

	// TODO: dirflags is a lookupflags, and it only has one bit: symlink_follow
	// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#lookupflags
	dirflags := uint16(params[1])

	path := uint32(params[2])
	pathLen := uint32(params[3])

	oflags := uint16(params[4])

	// rights aren't used
	_, _ = params[5], params[6]

	fdflags := uint16(params[7])
	resultOpenedFd := uint32(params[8])

	preopen, pathName, errno := atPath(fsc, mod.Memory(), preopenFD, path, pathLen)
	if errno != ErrnoSuccess {
		return errno
	}

	fileOpenFlags := openFlags(dirflags, oflags, fdflags)
	isDir := fileOpenFlags&platform.O_DIRECTORY != 0

	if isDir && oflags&O_CREAT != 0 {
		return ErrnoInval // use pathCreateDirectory!
	}

	newFD, err := fsc.OpenFile(preopen, pathName, fileOpenFlags, 0o600)
	if err != nil {
		return ToErrno(err)
	}

	// Check any flags that require the file to evaluate.
	if isDir {
		if f, ok := fsc.LookupFile(newFD); !ok {
			return ErrnoBadf // unexpected
		} else if _, ft, err := f.CachedStat(); err != nil {
			_ = fsc.CloseFile(newFD)
			return ToErrno(err)
		} else if ft.Type() != fs.ModeDir {
			_ = fsc.CloseFile(newFD)
			return ErrnoNotdir
		}
	}

	if !mod.Memory().WriteUint32Le(resultOpenedFd, newFD) {
		_ = fsc.CloseFile(newFD)
		return ErrnoFault
	}
	return ErrnoSuccess
}

// atPath returns the pre-open specific path after verifying it is a directory.
//
// # Notes
//
// Languages including Zig and Rust use only pre-opens for the FD because
// wasi-libc `__wasilibc_find_relpath` will only return a preopen. That said,
// our wasi.c example shows other languages act differently and can use dirFD
// of a non-preopen.
//
// We don't handle AT_FDCWD, as that's resolved in the compiler. There's no
// working directory function in WASI, so most assume CWD is "/". Notably, Zig
// has different behavior which assumes it is whatever the first pre-open name
// is.
//
// See https://github.com/WebAssembly/wasi-libc/blob/659ff414560721b1660a19685110e484a081c3d4/libc-bottom-half/sources/at_fdcwd.c
// See https://linux.die.net/man/2/openat
func atPath(fsc *sys.FSContext, mem api.Memory, dirFD, path, pathLen uint32) (sysfs.FS, string, Errno) {
	b, ok := mem.Read(path, pathLen)
	if !ok {
		return nil, "", ErrnoFault
	}
	pathName := string(b)

	if f, ok := fsc.LookupFile(dirFD); !ok {
		return nil, "", ErrnoBadf // closed
	} else if _, ft, err := f.CachedStat(); err != nil {
		return nil, "", ToErrno(err)
	} else if ft.Type() != fs.ModeDir {
		return nil, "", ErrnoNotdir
	} else if f.IsPreopen { // don't append the pre-open name
		return f.FS, pathName, ErrnoSuccess
	} else {
		return f.FS, pathutil.Join(f.Name, pathName), ErrnoSuccess
	}
}

func preopenPath(fsc *sys.FSContext, dirFD uint32) (string, Errno) {
	if f, ok := fsc.LookupFile(dirFD); !ok {
		return "", ErrnoBadf // closed
	} else if !f.IsPreopen {
		return "", ErrnoBadf
	} else {
		return f.Name, ErrnoSuccess
	}
}

func openFlags(dirflags, oflags, fdflags uint16) (openFlags int) {
	if dirflags&LOOKUP_SYMLINK_FOLLOW == 0 {
		openFlags |= platform.O_NOFOLLOW
	}
	if oflags&O_DIRECTORY != 0 {
		openFlags |= platform.O_DIRECTORY
		return // Early return for directories as the rest of flags doesn't make sense for it.
	} else if oflags&O_EXCL != 0 {
		openFlags |= syscall.O_EXCL
	}
	if oflags&O_TRUNC != 0 {
		openFlags |= syscall.O_RDWR | syscall.O_TRUNC
	}
	if oflags&O_CREAT != 0 {
		openFlags |= syscall.O_RDWR | syscall.O_CREAT
	}
	if fdflags&FD_APPEND != 0 {
		openFlags |= syscall.O_RDWR | syscall.O_APPEND
	}
	if openFlags == 0 {
		openFlags = syscall.O_RDONLY
	}
	return
}

// pathReadlink is the WASI function named PathReadlinkName that reads the
// contents of a symbolic link.
//
// See: https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_readlinkfd-fd-path-string-buf-pointeru8-buf_len-size---errno-size
var pathReadlink = newHostFunc(
	PathReadlinkName, pathReadlinkFn,
	[]wasm.ValueType{i32, i32, i32, i32, i32, i32},
	"fd", "path", "path_len", "buf", "buf_len", "result.bufused",
)

func pathReadlinkFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()

	fd := uint32(params[0])
	path := uint32(params[1])
	pathLen := uint32(params[2])
	bufPtr := uint32(params[3])
	bufLen := uint32(params[4])
	resultBufUsedPtr := uint32(params[5])

	if pathLen == 0 || bufLen == 0 {
		return ErrnoInval
	}

	mem := mod.Memory()
	preopen, p, en := atPath(fsc, mem, fd, path, pathLen)
	if en != ErrnoSuccess {
		return en
	}

	buf, ok := mem.Read(bufPtr, bufLen)
	if !ok {
		return ErrnoFault
	}

	n, err := preopen.Readlink(p, buf)
	if err != nil {
		return ToErrno(err)
	}

	if !mem.WriteUint32Le(resultBufUsedPtr, uint32(n)) {
		return ErrnoFault
	}
	return ErrnoSuccess
}

// pathRemoveDirectory is the WASI function named PathRemoveDirectoryName which
// removes a directory.
//
// # Parameters
//
//   - fd: file descriptor of a directory that `path` is relative to
//   - path: offset in api.Memory to read the path string from
//   - pathLen: length of `path`
//
// # Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: `fd` is invalid
//   - ErrnoNoent: `path` does not exist.
//   - ErrnoNotempty: `path` is not empty
//   - ErrnoNotdir: `path` is a file
//
// # Notes
//   - This is similar to unlinkat with AT_REMOVEDIR in POSIX.
//     See https://linux.die.net/man/2/unlinkat
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_remove_directoryfd-fd-path-string---errno
var pathRemoveDirectory = newHostFunc(
	PathRemoveDirectoryName, pathRemoveDirectoryFn,
	[]wasm.ValueType{i32, i32, i32},
	"fd", "path", "path_len",
)

func pathRemoveDirectoryFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()

	dirFD := uint32(params[0])
	path := uint32(params[1])
	pathLen := uint32(params[2])

	preopen, pathName, errno := atPath(fsc, mod.Memory(), dirFD, path, pathLen)
	if errno != ErrnoSuccess {
		return errno
	}

	if err := preopen.Rmdir(pathName); err != nil {
		return ToErrno(err)
	}

	return ErrnoSuccess
}

// pathRename is the WASI function named PathRenameName which renames a file or
// directory.
//
// # Parameters
//
//   - fd: file descriptor of a directory that `old_path` is relative to
//   - old_path: offset in api.Memory to read the old path string from
//   - old_path_len: length of `old_path`
//   - new_fd: file descriptor of a directory that `new_path` is relative to
//   - new_path: offset in api.Memory to read the new path string from
//   - new_path_len: length of `new_path`
//
// # Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: `fd` or `new_fd` are invalid
//   - ErrnoNoent: `old_path` does not exist.
//   - ErrnoNotdir: `old` is a directory and `new` exists, but is a file.
//   - ErrnoIsdir: `old` is a file and `new` exists, but is a directory.
//
// # Notes
//   - This is similar to unlinkat in POSIX.
//     See https://linux.die.net/man/2/renameat
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_renamefd-fd-old_path-string-new_fd-fd-new_path-string---errno
var pathRename = newHostFunc(
	PathRenameName, pathRenameFn,
	[]wasm.ValueType{i32, i32, i32, i32, i32, i32},
	"fd", "old_path", "old_path_len", "new_fd", "new_path", "new_path_len",
)

func pathRenameFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()

	olddirFD := uint32(params[0])
	oldPath := uint32(params[1])
	oldPathLen := uint32(params[2])

	newdirFD := uint32(params[3])
	newPath := uint32(params[4])
	newPathLen := uint32(params[5])

	oldFS, oldPathName, errno := atPath(fsc, mod.Memory(), olddirFD, oldPath, oldPathLen)
	if errno != ErrnoSuccess {
		return errno
	}

	newFS, newPathName, errno := atPath(fsc, mod.Memory(), newdirFD, newPath, newPathLen)
	if errno != ErrnoSuccess {
		return errno
	}

	if oldFS != newFS { // TODO: handle renames across filesystems
		return ErrnoNosys
	}

	if err := oldFS.Rename(oldPathName, newPathName); err != nil {
		return ToErrno(err)
	}

	return ErrnoSuccess
}

// pathSymlink is the WASI function named PathSymlinkName which creates a
// symbolic link.
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#path_symlink
var pathSymlink = newHostFunc(
	PathSymlinkName, pathSymlinkFn,
	[]wasm.ValueType{i32, i32, i32, i32, i32},
	"old_path", "old_path_len", "fd", "new_path", "new_path_len",
)

func pathSymlinkFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()

	oldPath := uint32(params[0])
	oldPathLen := uint32(params[1])
	dirFD := uint32(params[2])
	newPath := uint32(params[3])
	newPathLen := uint32(params[4])

	mem := mod.Memory()

	dir, ok := fsc.LookupFile(dirFD)
	if !ok {
		return ErrnoBadf // closed
	} else if _, ft, err := dir.CachedStat(); err != nil {
		return ToErrno(err)
	} else if ft.Type() != fs.ModeDir {
		return ErrnoNotdir
	}

	if oldPathLen == 0 || newPathLen == 0 {
		return ErrnoInval
	}

	oldPathBuf, ok := mem.Read(oldPath, oldPathLen)
	if !ok {
		return ErrnoFault
	}

	newPathBuf, ok := mem.Read(newPath, newPathLen)
	if !ok {
		return ErrnoFault
	}

	if err := dir.FS.Symlink(
		// Do not join old path since it's only resolved when dereference the link created here.
		// And the dereference result depends on the opening directory's file descriptor at that point.
		bufToStr(oldPathBuf, int(oldPathLen)),
		pathutil.Join(dir.Name, bufToStr(newPathBuf, int(newPathLen))),
	); err != nil {
		return ToErrno(err)
	}
	return ErrnoSuccess
}

// bufToStr converts the given byte slice as string unsafely.
func bufToStr(buf []byte, l int) string {
	return *(*string)(unsafe.Pointer(&reflect.SliceHeader{ //nolint
		Data: uintptr(unsafe.Pointer(&buf[0])),
		Len:  l,
		Cap:  l,
	}))
}

// pathUnlinkFile is the WASI function named PathUnlinkFileName which unlinks a
// file.
//
// # Parameters
//
//   - fd: file descriptor of a directory that `path` is relative to
//   - path: offset in api.Memory to read the path string from
//   - pathLen: length of `path`
//
// # Result (Errno)
//
// The return value is ErrnoSuccess except the following error conditions:
//   - ErrnoBadf: `fd` is invalid
//   - ErrnoNoent: `path` does not exist.
//   - ErrnoIsdir: `path` is a directory
//
// # Notes
//   - This is similar to unlinkat without AT_REMOVEDIR in POSIX.
//     See https://linux.die.net/man/2/unlinkat
//
// See https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-path_unlink_filefd-fd-path-string---errno
var pathUnlinkFile = newHostFunc(
	PathUnlinkFileName, pathUnlinkFileFn,
	[]wasm.ValueType{i32, i32, i32},
	"fd", "path", "path_len",
)

func pathUnlinkFileFn(_ context.Context, mod api.Module, params []uint64) Errno {
	fsc := mod.(*wasm.CallContext).Sys.FS()

	dirFD := uint32(params[0])
	path := uint32(params[1])
	pathLen := uint32(params[2])

	preopen, pathName, errno := atPath(fsc, mod.Memory(), dirFD, path, pathLen)
	if errno != ErrnoSuccess {
		return errno
	}

	if err := preopen.Unlink(pathName); err != nil {
		return ToErrno(err)
	}

	return ErrnoSuccess
}
