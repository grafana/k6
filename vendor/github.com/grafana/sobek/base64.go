package sobek

import (
	"errors"
	"strconv"
)

type base64LastChunkHandling uint8

const (
	base64LastChunkHandlingInvalid base64LastChunkHandling = iota
	base64LastChunkHandlingLoose
	base64LastChunkHandlingStrict
	base64LastChunkHandlingStop
)

var base64DecodeMap = func() (t [256]byte) {
	for i := range t {
		t[i] = 0xff
	}
	for i, c := range "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/" {
		t[c] = byte(i)
	}
	return
}()

var base64DecodeMapUrl = func() (t [256]byte) {
	for i := range t {
		t[i] = 0xff
	}
	for i, c := range "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_" {
		t[c] = byte(i)
	}
	return
}()

// TC39 Abstract Operations for Uint8Array Objects - [SkipAsciiWhitespace(string, index)].
//
// [SkipAsciiWhitespace(string, index)]: https://tc39.es/ecma262/multipage/indexed-collections.html#sec-skipasciiwhitespace
func skipAsciiWhitespace(s String, index int) int {
	length := s.Length()
	for index < length {
		switch s.CharAt(index) {
		case 0x0009, 0x000A, 0x000C, 0x000D, 0x0020:
			index++
		default:
			return index
		}
	}
	return index
}

func base64Assemble32Checked(n1, n2, n3, n4 byte) uint32 {
	return uint32(n1)<<18 |
		uint32(n2)<<12 |
		uint32(n3)<<6 |
		uint32(n4)
}

func base64Assemble32(n1, n2, n3, n4 byte) (dn uint32, ok bool) {
	// Check that all the digits are valid. If any of them was 0xff, their
	// bitwise OR will be 0xff.
	if n1|n2|n3|n4 == 0xff {
		return 0, false
	}
	return base64Assemble32Checked(n1, n2, n3, n4), true
}

func base64Assemble64(n1, n2, n3, n4, n5, n6, n7, n8 byte) (dn uint64, ok bool) {
	// Check that all the digits are valid. If any of them was 0xff, their
	// bitwise OR will be 0xff.
	if n1|n2|n3|n4|n5|n6|n7|n8 == 0xff {
		return 0, false
	}
	return uint64(n1)<<42 |
			uint64(n2)<<36 |
			uint64(n3)<<30 |
			uint64(n4)<<24 |
			uint64(n5)<<18 |
			uint64(n6)<<12 |
			uint64(n7)<<6 |
			uint64(n8),
		true
}

func base64Put6Bytes(b []byte, v uint64) {
	_ = b[5] // early bounds check to guarantee safety of writes below
	b[0] = byte(v >> 40)
	b[1] = byte(v >> 32)
	b[2] = byte(v >> 24)
	b[3] = byte(v >> 16)
	b[4] = byte(v >> 8)
	b[5] = byte(v)
}

func base64Put3Bytes(b []byte, v uint32) {
	_ = b[2] // early bounds check to guarantee safety of writes below
	b[0] = byte(v >> 16)
	b[1] = byte(v >> 8)
	b[2] = byte(v)
}

// TC39 Abstract Operations for Uint8Array Objects - [DecodeFinalBase64Chunk(chunk, throwOnExtraBits)].
// chunk must contain 2 or 3 characters of the standard base64 alphabet.
//
// [DecodeFinalBase64Chunk(chunk, throwOnExtraBits)]: https://tc39.es/ecma262/multipage/indexed-collections.html#sec-decodefinalbase64chunk
func decodeFinalBase64Chunk(chunk [4]byte, chunkLength int, throwOnExtraBits bool) ([3]byte, int, error) {

	n := base64Assemble32Checked(chunk[0], chunk[1], chunk[2], chunk[3])

	var res [3]byte
	base64Put3Bytes(res[:], n)

	if chunkLength == 2 {
		if throwOnExtraBits && res[1] != 0 {
			return [3]byte{}, 0, errors.New("extra bits in the last base64 character")
		}
		return res, 1, nil
	}
	if throwOnExtraBits && res[2] != 0 {
		return [3]byte{}, 0, errors.New("extra bits in the last base64 character")
	}
	return res, 2, nil
}

// TC39 Abstract Operations for Uint8Array Objects - [FromBase64(string, alphabet, lastChunkHandling)],
//
// [FromBase64(string, alphabet, lastChunkHandling, maxLength)]: https://tc39.es/ecma262/multipage/indexed-collections.html#sec-frombase64
func fromBase64(s String, decodeMap *[256]byte, lastChunkHandling base64LastChunkHandling) (read int, bytes []byte, err error) {
	// 4 characters of input decode into at most 3 bytes, so a buffer of the
	// upper-bound size makes fromBase64Into behave as if maxLength were absent.
	dst := make([]byte, (s.Length()+3)/4*3)
	read, written, err := fromBase64Into(s, decodeMap, lastChunkHandling, dst)
	return read, dst[:written], err
}

// Decode a base64-encoded ASCII string.
// The fast case algorithm is borrowed from the standard Go encoding/base64.
func base64DecodeAscii(a asciiString, s String, decodeMap *[256]byte, lastChunkHandling base64LastChunkHandling, dst []byte) (read, written int, err error) {
	// Lift the nil check outside of the loop. decodeMap is directly
	// used later in this function, to let the compiler know that the
	// receiver can't be nil.
	_ = *decodeMap

	for strconv.IntSize >= 64 && len(a)-read >= 8 && len(dst)-written >= 6 {
		src2 := a[read : read+8]
		if dn, ok := base64Assemble64(
			decodeMap[src2[0]],
			decodeMap[src2[1]],
			decodeMap[src2[2]],
			decodeMap[src2[3]],
			decodeMap[src2[4]],
			decodeMap[src2[5]],
			decodeMap[src2[6]],
			decodeMap[src2[7]],
		); ok {
			base64Put6Bytes(dst[written:], dn)
			written += 6
			read += 8
		} else {
			var eof bool
			read, written, eof, err = base64DecodeChunk(s, read, len(a), decodeMap, lastChunkHandling, dst, written)
			if eof || err != nil {
				return
			}
		}
	}

	for len(a)-read >= 4 && len(dst)-written >= 3 {
		src2 := a[read : read+4]
		if dn, ok := base64Assemble32(
			decodeMap[src2[0]],
			decodeMap[src2[1]],
			decodeMap[src2[2]],
			decodeMap[src2[3]],
		); ok {
			base64Put3Bytes(dst[written:], dn)
			written += 3
			read += 4
		} else {
			var eof bool
			read, written, eof, err = base64DecodeChunk(s, read, len(a), decodeMap, lastChunkHandling, dst, written)
			if eof || err != nil {
				return
			}
		}
	}

	if written == len(dst) {
		return
	}

	for eof := false; !eof; {
		read, written, eof, err = base64DecodeChunk(s, read, len(a), decodeMap, lastChunkHandling, dst, written)
		if err != nil {
			return
		}
	}
	return
}

// Decode a base64-encoded Unicode string.
// The fast case algorithm is borrowed from the standard Go encoding/base64.
func base64DecodeUnicode(u unicodeString, s String, decodeMap *[256]byte, lastChunkHandling base64LastChunkHandling, dst []byte) (read, written int, err error) {
	u = u[1:] // Skip BOM

	// Lift the nil check outside of the loop. decodeMap is directly
	// used later in this function, to let the compiler know that the
	// receiver can't be nil.
	_ = *decodeMap

	for strconv.IntSize >= 64 && len(u)-read >= 8 && len(dst)-written >= 6 {
		src2 := u[read : read+8]
		// Make sure there are no code units above 255
		if (src2[0]|src2[1]|src2[2]|src2[3]|src2[4]|src2[5]|src2[6]|src2[7])&0xFF00 == 0 {
			if dn, ok := base64Assemble64(
				decodeMap[byte(src2[0])],
				decodeMap[byte(src2[1])],
				decodeMap[byte(src2[2])],
				decodeMap[byte(src2[3])],
				decodeMap[byte(src2[4])],
				decodeMap[byte(src2[5])],
				decodeMap[byte(src2[6])],
				decodeMap[byte(src2[7])],
			); ok {
				base64Put6Bytes(dst[written:], dn)
				written += 6
				read += 8
				continue
			}
		}
		var eof bool
		read, written, eof, err = base64DecodeChunk(s, read, len(u), decodeMap, lastChunkHandling, dst, written)
		if eof || err != nil {
			return
		}
	}

	for len(u)-read >= 4 && len(dst)-written >= 3 {
		src2 := u[read : read+4]
		// Make sure there are no code units above 255
		if (src2[0]|src2[1]|src2[2]|src2[3])&0xFF00 == 0 {
			if dn, ok := base64Assemble32(
				decodeMap[byte(src2[0])],
				decodeMap[byte(src2[1])],
				decodeMap[byte(src2[2])],
				decodeMap[byte(src2[3])],
			); ok {
				base64Put3Bytes(dst[written:], dn)
				written += 3
				read += 4
				continue
			}
		}
		var eof bool
		read, written, eof, err = base64DecodeChunk(s, read, len(u), decodeMap, lastChunkHandling, dst, written)
		if eof || err != nil {
			return
		}
	}

	if written == len(dst) {
		return
	}

	for eof := false; !eof; {
		read, written, eof, err = base64DecodeChunk(s, read, len(u), decodeMap, lastChunkHandling, dst, written)
		if err != nil {
			return
		}
	}
	return
}

// Decode a single base64 chunk where fast case fails (either because there are whitespaces or because it's the last chunk, short or padded).
func base64DecodeChunk(s String, index, length int, decodeMap *[256]byte, lastChunkHandling base64LastChunkHandling, dst []byte, dstIndex int) (newIndex int, newDstIndex int, eof bool, err error) {
	newIndex = index
	newDstIndex = dstIndex
	var chunk [4]byte
	chunkLength := 0
	for {
		index = skipAsciiWhitespace(s, index)
		if index == length {
			eof = true
			if chunkLength > 0 {
				if lastChunkHandling == base64LastChunkHandlingStop {
					return
				}
				if lastChunkHandling == base64LastChunkHandlingStrict {
					err = errors.New("missing padding in the last chunk")
					return
				}
				// lastChunkHandling is "loose"
				if chunkLength == 1 {
					err = errors.New("a single extra base64 character in the last chunk")
					return
				}
				dec, n, _ := decodeFinalBase64Chunk(chunk, chunkLength, false)
				newDstIndex += copy(dst[newDstIndex:], dec[:n])
			}
			newIndex = length
			return
		}
		char := s.CharAt(index)
		index++
		if char == '=' {
			if chunkLength < 2 {
				err = errors.New("unexpected padding character")
				return
			}
			index = skipAsciiWhitespace(s, index)
			if chunkLength == 2 {
				if index == length {
					if lastChunkHandling == base64LastChunkHandlingStop {
						eof = true
						return
					}
					err = errors.New("missing padding character")
					return
				}
				if s.CharAt(index) == '=' {
					index = skipAsciiWhitespace(s, index+1)
				}
			}
			if index < length {
				err = errors.New("unexpected character after padding")
				return
			}
			dec, n, decErr := decodeFinalBase64Chunk(chunk, chunkLength, lastChunkHandling == base64LastChunkHandlingStrict)
			if decErr != nil {
				err = decErr
				return
			}
			newDstIndex += copy(dst[newDstIndex:], dec[:n])
			newIndex = length
			eof = true
			return
		}
		if char >= 128 {
			err = errors.New("invalid base64 character")
			return
		}
		b := decodeMap[char]
		if b == 0xff {
			err = errors.New("invalid base64 character")
			return
		}
		remaining := len(dst) - newDstIndex
		if (remaining == 1 && chunkLength == 2) || (remaining == 2 && chunkLength == 3) {
			eof = true
			return
		}
		chunk[chunkLength] = b
		chunkLength++
		if chunkLength == 4 {
			n := base64Assemble32Checked(chunk[0], chunk[1], chunk[2], chunk[3])
			if remaining > 3 {
				base64Put3Bytes(dst[newDstIndex:], n)
				newDstIndex += 3
			} else {
				base64Put3Bytes(chunk[:], n)
				newDstIndex += copy(dst[newDstIndex:], chunk[:])
				eof = true
			}
			newIndex = index
			return
		}
	}
}
