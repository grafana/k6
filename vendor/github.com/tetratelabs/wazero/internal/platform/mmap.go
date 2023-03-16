// This uses syscall.Mprotect. Go's SDK only supports this on darwin and linux.
//go:build darwin || linux || freebsd

package platform

import (
	"io"
	"syscall"
	"unsafe"
)

func munmapCodeSegment(code []byte) error {
	return syscall.Munmap(code)
}

// mmapCodeSegmentAMD64 gives all read-write-exec permission to the mmap region
// to enter the function. Otherwise, segmentation fault exception is raised.
func mmapCodeSegmentAMD64(code io.Reader, size int) ([]byte, error) {
	mmapFunc, err := syscall.Mmap(
		-1,
		0,
		size,
		// The region must be RWX: RW for writing native codes, X for executing the region.
		syscall.PROT_READ|syscall.PROT_WRITE|syscall.PROT_EXEC,
		// Anonymous as this is not an actual file, but a memory,
		// Private as this is in-process memory region.
		syscall.MAP_ANON|syscall.MAP_PRIVATE,
	)
	if err != nil {
		return nil, err
	}

	w := &bufWriter{underlying: mmapFunc}
	_, err = io.CopyN(w, code, int64(size))
	return mmapFunc, err
}

// mmapCodeSegmentARM64 cannot give all read-write-exec permission to the mmap region.
// Otherwise, the mmap systemcall would raise an error. Here we give read-write
// to the region at first, write the native code and then change the perm to
// read-exec, so we can execute the native code.
func mmapCodeSegmentARM64(code io.Reader, size int) ([]byte, error) {
	mmapFunc, err := syscall.Mmap(
		-1,
		0,
		size,
		// The region must be RW: RW for writing native codes.
		syscall.PROT_READ|syscall.PROT_WRITE,
		// Anonymous as this is not an actual file, but a memory,
		// Private as this is in-process memory region.
		syscall.MAP_ANON|syscall.MAP_PRIVATE,
	)
	if err != nil {
		return nil, err
	}

	w := &bufWriter{underlying: mmapFunc}
	_, err = io.CopyN(w, code, int64(size))
	if err != nil {
		return nil, err
	}

	// Then we're done with writing code, change the permission to RX.
	err = mprotect(mmapFunc, syscall.PROT_READ|syscall.PROT_EXEC)
	return mmapFunc, err
}

var _zero uintptr

// mprotect is like syscall.Mprotect, defined locally so that freebsd compiles.
func mprotect(b []byte, prot int) (err error) {
	var _p0 unsafe.Pointer
	if len(b) > 0 {
		_p0 = unsafe.Pointer(&b[0])
	} else {
		_p0 = unsafe.Pointer(&_zero)
	}
	_, _, e1 := syscall.Syscall(syscall.SYS_MPROTECT, uintptr(_p0), uintptr(len(b)), uintptr(prot))
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return
}
