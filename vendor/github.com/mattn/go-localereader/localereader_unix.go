// +build !windows

package localereader

import (
	"io"
)

func newReader(r io.Reader) io.Reader {
	return r
}
