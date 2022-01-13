// Package unistring contains an implementation of a hybrid ASCII/UTF-16 string.
// For ASCII strings the underlying representation is equivalent to a normal Go string.
// For unicode strings the underlying representation is UTF-16 as []uint16 with 0th element set to 0xFEFF.
// unicode.String allows representing malformed UTF-16 values (e.g. stand-alone parts of surrogate pairs)
// which cannot be represented in UTF-8.
// At the same time it is possible to use unicode.String as property keys just as efficiently as simple strings,
// (the leading 0xFEFF ensures there is no clash with ASCII string), and it is possible to convert it
// to valueString without extra allocations.
package unistring

import (
	"reflect"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

const (
	BOM = 0xFEFF
)

type String string

func NewFromString(s string) String {
	ascii := true
	size := 0
	for _, c := range s {
		if c >= utf8.RuneSelf {
			ascii = false
			if c > 0xFFFF {
				size++
			}
		}
		size++
	}
	if ascii {
		return String(s)
	}
	b := make([]uint16, size+1)
	b[0] = BOM
	i := 1
	for _, c := range s {
		if c <= 0xFFFF {
			b[i] = uint16(c)
		} else {
			first, second := utf16.EncodeRune(c)
			b[i] = uint16(first)
			i++
			b[i] = uint16(second)
		}
		i++
	}
	return FromUtf16(b)
}

func NewFromRunes(s []rune) String {
	ascii := true
	size := 0
	for _, c := range s {
		if c >= utf8.RuneSelf {
			ascii = false
			if c > 0xFFFF {
				size++
			}
		}
		size++
	}
	if ascii {
		return String(s)
	}
	b := make([]uint16, size+1)
	b[0] = BOM
	i := 1
	for _, c := range s {
		if c <= 0xFFFF {
			b[i] = uint16(c)
		} else {
			first, second := utf16.EncodeRune(c)
			b[i] = uint16(first)
			i++
			b[i] = uint16(second)
		}
		i++
	}
	return FromUtf16(b)
}

func FromUtf16(b []uint16) String {
	var str string
	hdr := (*reflect.StringHeader)(unsafe.Pointer(&str))
	hdr.Data = uintptr(unsafe.Pointer(&b[0]))
	hdr.Len = len(b) * 2

	return String(str)
}

func (s String) String() string {
	if b := s.AsUtf16(); b != nil {
		return string(utf16.Decode(b[1:]))
	}

	return string(s)
}

func (s String) AsUtf16() []uint16 {
	if len(s) < 4 || len(s)&1 != 0 {
		return nil
	}

	var a []uint16
	raw := string(s)

	sliceHeader := (*reflect.SliceHeader)(unsafe.Pointer(&a))
	sliceHeader.Data = (*reflect.StringHeader)(unsafe.Pointer(&raw)).Data

	l := len(raw) / 2

	sliceHeader.Len = l
	sliceHeader.Cap = l

	if a[0] == BOM {
		return a
	}

	return nil
}
