// +build windows

package localereader

import (
	"io"
	"syscall"
	"unicode/utf8"
	"unsafe"

	"golang.org/x/text/transform"
)

const (
	CP_ACP               = 0
	MB_ERR_INVALID_CHARS = 8
)

var (
	modkernel32             = syscall.NewLazyDLL("kernel32.dll")
	procMultiByteToWideChar = modkernel32.NewProc("MultiByteToWideChar")
	procIsDBCSLeadByte      = modkernel32.NewProc("IsDBCSLeadByte")
)

type codepageDecoder struct {
	transform.NopResetter

	cp int
}

func (codepageDecoder) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	r, size := rune(0), 0
loop:
	for ; nSrc < len(src); nSrc += size {
		switch c0 := src[nSrc]; {
		case c0 < utf8.RuneSelf:
			r, size = rune(c0), 1

		default:
			br, _, _ := procIsDBCSLeadByte.Call(uintptr(src[nSrc]))
			if br == 0 {
				r = rune(src[nSrc])
				size = 1
				break
			}
			if nSrc >= len(src)-1 {
				r = rune(src[nSrc])
				size = 1
				break
			}
			n, _, _ := procMultiByteToWideChar.Call(CP_ACP, 0, uintptr(unsafe.Pointer(&src[nSrc])), uintptr(2), uintptr(0), 0)
			if n <= 0 {
				err = syscall.GetLastError()
				break
			}
			var us [1]uint16
			rc, _, _ := procMultiByteToWideChar.Call(CP_ACP, 0, uintptr(unsafe.Pointer(&src[nSrc])), uintptr(2), uintptr(unsafe.Pointer(&us[0])), 1)
			if rc == 0 {
				size = 1
				break
			}
			r = rune(us[0])
			size = 2
		}
		if nDst+utf8.RuneLen(r) > len(dst) {
			err = transform.ErrShortDst
			break loop
		}
		nDst += utf8.EncodeRune(dst[nDst:], r)
	}
	return nDst, nSrc, err

}

func newReader(r io.Reader) io.Reader {
	return transform.NewReader(r, NewAcpDecoder())
}

func NewCodePageDecoder(cp int) transform.Transformer {
	return &codepageDecoder{cp: cp}
}

func NewAcpDecoder() transform.Transformer {
	return &codepageDecoder{cp: CP_ACP}
}
