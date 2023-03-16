// Package platform includes runtime-specific code needed for the compiler or otherwise.
//
// Note: This is a dependency-free alternative to depending on parts of Go's x/sys.
// See /RATIONALE.md for more context.
package platform

import (
	"errors"
	"io"
	"runtime"
	"strings"
)

// TODO: IsAtLeastGo120
var IsGo120 = strings.Contains(runtime.Version(), "go1.20")

// archRequirementsVerified is set by platform-specific init to true if the platform is supported
var archRequirementsVerified bool

// CompilerSupported is exported for tests and includes constraints here and also the assembler.
func CompilerSupported() bool {
	switch runtime.GOOS {
	case "darwin", "windows", "linux", "freebsd":
	default:
		return false
	}

	return archRequirementsVerified
}

// MmapCodeSegment copies the code into the executable region and returns the byte slice of the region.
//
// See https://man7.org/linux/man-pages/man2/mmap.2.html for mmap API and flags.
func MmapCodeSegment(code io.Reader, size int) ([]byte, error) {
	if size == 0 {
		panic(errors.New("BUG: MmapCodeSegment with zero length"))
	}
	if runtime.GOARCH == "amd64" {
		return mmapCodeSegmentAMD64(code, size)
	} else {
		return mmapCodeSegmentARM64(code, size)
	}
}

// MunmapCodeSegment unmaps the given memory region.
func MunmapCodeSegment(code []byte) error {
	if len(code) == 0 {
		panic(errors.New("BUG: MunmapCodeSegment with zero length"))
	}
	return munmapCodeSegment(code)
}

// IsTerminal returns true if the given file descriptor is a terminal.
func IsTerminal(fd uintptr) bool {
	return isTerminal(fd)
}
