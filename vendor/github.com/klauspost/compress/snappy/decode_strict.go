// Copyright 2011 The Snappy-Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package snappy

import "encoding/binary"

// DecodeStrict returns the decoded form of src, like Decode.
//
// Unlike Decode, which delegates to the s2 decoder and accepts the s2
// "repeat last offset" extension (a copy whose encoded offset is 0),
// DecodeStrict uses a strict standard-Snappy block decoder. Offset 0 is not
// valid in the standard Snappy block format, so such a copy is rejected with
// ErrCorrupt. This makes DecodeStrict match github.com/golang/snappy and the
// C++ reference decoder, at the cost of being slower than Decode.
//
// The returned slice may be a sub-slice of dst if dst was large enough to
// hold the entire decoded block. Otherwise, a newly allocated slice will be
// returned.
//
// The dst and src must not overlap. It is valid to pass a nil dst.
//
// DecodeStrict handles the Snappy block format, not the Snappy stream format.
func DecodeStrict(dst, src []byte) ([]byte, error) {
	v, n := binary.Uvarint(src)
	if n <= 0 || n > 5 || v > 0xffffffff {
		return nil, ErrCorrupt
	}
	const wordSize = 32 << (^uint(0) >> 32 & 1)
	if wordSize == 32 && v > 0x7fffffff {
		return nil, ErrTooLarge
	}
	dLen := int(v)
	if dLen <= cap(dst) {
		dst = dst[:dLen]
	} else {
		dst = make([]byte, dLen)
	}
	if decodeStrict(dst, src[n:]) != 0 {
		return nil, ErrCorrupt
	}
	return dst, nil
}

func decodeStrict(dst, src []byte) int {
	var d, s, offset, length int
	for s < len(src) {
		switch src[s] & 0x03 {
		case 0x00:
			x := uint32(src[s] >> 2)
			switch {
			case x < 60:
				s++
			case x == 60:
				s += 2
				if uint(s) > uint(len(src)) {
					return 1
				}
				x = uint32(src[s-1])
			case x == 61:
				s += 3
				if uint(s) > uint(len(src)) {
					return 1
				}
				x = uint32(src[s-2]) | uint32(src[s-1])<<8
			case x == 62:
				s += 4
				if uint(s) > uint(len(src)) {
					return 1
				}
				x = uint32(src[s-3]) | uint32(src[s-2])<<8 | uint32(src[s-1])<<16
			case x == 63:
				s += 5
				if uint(s) > uint(len(src)) {
					return 1
				}
				x = uint32(src[s-4]) | uint32(src[s-3])<<8 | uint32(src[s-2])<<16 | uint32(src[s-1])<<24
			}
			length = int(x) + 1
			if length <= 0 || length > len(dst)-d || length > len(src)-s {
				return 1
			}
			copy(dst[d:], src[s:s+length])
			d += length
			s += length
			continue
		case 0x01:
			s += 2
			if uint(s) > uint(len(src)) {
				return 1
			}
			length = 4 + int(src[s-2])>>2&0x7
			offset = int(uint32(src[s-2])&0xe0<<3 | uint32(src[s-1]))
		case 0x02:
			s += 3
			if uint(s) > uint(len(src)) {
				return 1
			}
			length = 1 + int(src[s-3])>>2
			offset = int(uint32(src[s-2]) | uint32(src[s-1])<<8)
		case 0x03:
			s += 5
			if uint(s) > uint(len(src)) {
				return 1
			}
			length = 1 + int(src[s-5])>>2
			offset = int(uint32(src[s-4]) | uint32(src[s-3])<<8 | uint32(src[s-2])<<16 | uint32(src[s-1])<<24)
		}
		if offset <= 0 || d < offset || length > len(dst)-d {
			return 1
		}
		if offset >= length {
			copy(dst[d:d+length], dst[d-offset:])
			d += length
			continue
		}
		a := dst[d : d+length]
		b := dst[d-offset:]
		b = b[:len(a)]
		for i := range a {
			a[i] = b[i]
		}
		d += length
	}
	if d != len(dst) {
		return 1
	}
	return 0
}
