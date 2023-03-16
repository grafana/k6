//go:build !(darwin || linux || freebsd || windows)

package platform

import (
	"fmt"
	"io"
	"runtime"
)

var errUnsupported = fmt.Errorf("mmap unsupported on GOOS=%s. Use interpreter instead.", runtime.GOOS)

func munmapCodeSegment(code []byte) error {
	panic(errUnsupported)
}

func mmapCodeSegmentAMD64(code io.Reader, size int) ([]byte, error) {
	panic(errUnsupported)
}

func mmapCodeSegmentARM64(code io.Reader, size int) ([]byte, error) {
	panic(errUnsupported)
}
