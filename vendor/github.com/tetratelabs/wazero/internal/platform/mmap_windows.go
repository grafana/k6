package platform

import (
	"fmt"
	"io"
	"reflect"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procVirtualAlloc   = kernel32.NewProc("VirtualAlloc")
	procVirtualProtect = kernel32.NewProc("VirtualProtect")
	procVirtualFree    = kernel32.NewProc("VirtualFree")
)

const (
	windows_MEM_COMMIT             uintptr = 0x00001000
	windows_MEM_RELEASE            uintptr = 0x00008000
	windows_PAGE_READWRITE         uintptr = 0x00000004
	windows_PAGE_EXECUTE_READ      uintptr = 0x00000020
	windows_PAGE_EXECUTE_READWRITE uintptr = 0x00000040
)

func munmapCodeSegment(code []byte) error {
	return freeMemory(code)
}

// allocateMemory commits the memory region via the "VirtualAlloc" function.
// See https://docs.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-virtualalloc
func allocateMemory(size uintptr, protect uintptr) (uintptr, error) {
	address := uintptr(0) // TODO: document why zero
	alloctype := windows_MEM_COMMIT
	if r, _, err := procVirtualAlloc.Call(address, size, alloctype, protect); r == 0 {
		return 0, fmt.Errorf("compiler: VirtualAlloc error: %w", ensureErr(err))
	} else {
		return r, nil
	}
}

// freeMemory releases the memory region via the "VirtualFree" function.
// See https://docs.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-virtualfree
func freeMemory(code []byte) error {
	address := uintptr(unsafe.Pointer(&code[0]))
	size := uintptr(0) // size must be 0 because we're using MEM_RELEASE.
	freetype := windows_MEM_RELEASE
	if r, _, err := procVirtualFree.Call(address, size, freetype); r == 0 {
		return fmt.Errorf("compiler: VirtualFree error: %w", ensureErr(err))
	}
	return nil
}

func virtualProtect(address, size, newprotect uintptr, oldprotect *uint32) error {
	if r, _, err := procVirtualProtect.Call(address, size, newprotect, uintptr(unsafe.Pointer(oldprotect))); r == 0 {
		return fmt.Errorf("compiler: VirtualProtect error: %w", ensureErr(err))
	}
	return nil
}

func mmapCodeSegmentAMD64(code io.Reader, size int) ([]byte, error) {
	p, err := allocateMemory(uintptr(size), windows_PAGE_EXECUTE_READWRITE)
	if err != nil {
		return nil, err
	}

	var mem []byte
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&mem))
	sh.Data = p
	sh.Len = size
	sh.Cap = size

	w := &bufWriter{underlying: mem}
	_, err = io.CopyN(w, code, int64(size))
	return mem, err
}

func mmapCodeSegmentARM64(code io.Reader, size int) ([]byte, error) {
	p, err := allocateMemory(uintptr(size), windows_PAGE_READWRITE)
	if err != nil {
		return nil, err
	}

	var mem []byte
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&mem))
	sh.Data = p
	sh.Len = size
	sh.Cap = size
	w := &bufWriter{underlying: mem}
	_, err = io.CopyN(w, code, int64(size))
	if err != nil {
		return nil, err
	}

	old := uint32(windows_PAGE_READWRITE)
	err = virtualProtect(p, uintptr(size), windows_PAGE_EXECUTE_READ, &old)
	if err != nil {
		return nil, err
	}
	return mem, nil
}

// ensureErr returns syscall.EINVAL when the input error is nil.
//
// We are supposed to use "GetLastError" which is more precise, but it is not safe to execute in goroutines. While
// "GetLastError" is thread-local, goroutines are not pinned to threads.
//
// See https://docs.microsoft.com/en-us/windows/win32/api/errhandlingapi/nf-errhandlingapi-getlasterror
func ensureErr(err error) error {
	if err != nil {
		return err
	}
	return syscall.EINVAL
}
