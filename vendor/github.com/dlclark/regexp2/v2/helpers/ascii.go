package helpers

import (
	"unicode/utf8"
	"unsafe"
)

const asciiHighBits = 0x8080808080808080

// ASCIIScanMin is the input length where a separate ASCII probe plus a tight
// copy beats a single UTF-8 range loop. Short matches stay on the one-pass path.
const ASCIIScanMin = 64

// ASCIISearchMin is the haystack length where stdlib/SIMD string search beats
// a scalar first-rune scan. Below this, needle setup and unaligned
// bytes.Index retries dominate.
const ASCIISearchMin = 32

// IsASCII reports whether s contains only bytes below utf8.RuneSelf.
func IsASCII(s string) bool {
	n := len(s)
	if n == 0 {
		return true
	}

	b := unsafe.Slice(unsafe.StringData(s), n)
	i := 0
	for ; i < n && uintptr(unsafe.Pointer(&b[i]))&7 != 0; i++ {
		if b[i] >= utf8.RuneSelf {
			return false
		}
	}
	for ; i+8 <= n; i += 8 {
		v := *(*uint64)(unsafe.Pointer(&b[i]))
		if v&asciiHighBits != 0 {
			return false
		}
	}
	for ; i < n; i++ {
		if b[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// IsASCIIRunes reports whether every rune in in is an ASCII code point.
func IsASCIIRunes(in []rune) bool {
	for _, ch := range in {
		if ch >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// DecodeString writes the runes of s into buf, which must have length >= len(s).
// It returns the rune count and whether s was all ASCII.
func DecodeString(s string, buf []rune) (n int, ascii bool) {
	if len(s) >= ASCIIScanMin && IsASCII(s) {
		for i := 0; i < len(s); i++ {
			buf[i] = rune(s[i])
		}
		return len(s), true
	}
	ascii = true
	for _, ch := range s {
		if ch >= utf8.RuneSelf {
			ascii = false
		}
		buf[n] = ch
		n++
	}
	return n, ascii
}
