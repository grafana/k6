// Package zstd provides decompression of zstandard files.
//
// For advanced usage and examples, go to the README: https://github.com/klauspost/compress/tree/master/zstd#zstd
package zstd

import (
	"errors"
	"log"
	"math/bits"
)

const debug = false
const debugSequences = false

// force encoder to use predefined tables.
const forcePreDef = false

// zstdMinMatch is the minimum zstd match length.
const zstdMinMatch = 3

var (
	// ErrReservedBlockType is returned when a reserved block type is found.
	// Typically this indicates wrong or corrupted input.
	ErrReservedBlockType = errors.New("invalid input: reserved block type encountered")

	// ErrCompressedSizeTooBig is returned when a block is bigger than allowed.
	// Typically this indicates wrong or corrupted input.
	ErrCompressedSizeTooBig = errors.New("invalid input: compressed size too big")

	// ErrBlockTooSmall is returned when a block is too small to be decoded.
	// Typically returned on invalid input.
	ErrBlockTooSmall = errors.New("block too small")

	// ErrMagicMismatch is returned when a "magic" number isn't what is expected.
	// Typically this indicates wrong or corrupted input.
	ErrMagicMismatch = errors.New("invalid input: magic number mismatch")

	// ErrWindowSizeExceeded is returned when a reference exceeds the valid window size.
	// Typically this indicates wrong or corrupted input.
	ErrWindowSizeExceeded = errors.New("window size exceeded")

	// ErrWindowSizeTooSmall is returned when no window size is specified.
	// Typically this indicates wrong or corrupted input.
	ErrWindowSizeTooSmall = errors.New("invalid input: window size was too small")

	// ErrDecoderSizeExceeded is returned if decompressed size exceeds the configured limit.
	ErrDecoderSizeExceeded = errors.New("decompressed size exceeds configured limit")

	// ErrUnknownDictionary is returned if the dictionary ID is unknown.
	// For the time being dictionaries are not supported.
	ErrUnknownDictionary = errors.New("unknown dictionary")

	// ErrFrameSizeExceeded is returned if the stated frame size is exceeded.
	// This is only returned if SingleSegment is specified on the frame.
	ErrFrameSizeExceeded = errors.New("frame size exceeded")

	// ErrCRCMismatch is returned if CRC mismatches.
	ErrCRCMismatch = errors.New("CRC check failed")

	// ErrDecoderClosed will be returned if the Decoder was used after
	// Close has been called.
	ErrDecoderClosed = errors.New("decoder used after Close")
)

func println(a ...interface{}) {
	if debug {
		log.Println(a...)
	}
}

func printf(format string, a ...interface{}) {
	if debug {
		log.Printf(format, a...)
	}
}

// matchLen returns the maximum length.
// a must be the shortest of the two.
// The function also returns whether all bytes matched.
func matchLen(a, b []byte) int {
	b = b[:len(a)]
	for i := 0; i < len(a)-7; i += 8 {
		if diff := load64(a, i) ^ load64(b, i); diff != 0 {
			return i + (bits.TrailingZeros64(diff) >> 3)
		}
	}
	checked := (len(a) >> 3) << 3
	a = a[checked:]
	b = b[checked:]
	// TODO: We could do a 4 check.
	for i := range a {
		if a[i] != b[i] {
			return int(i) + checked
		}
	}
	return len(a) + checked
}

// matchLen returns a match length in src between index s and t
func matchLenIn(src []byte, s, t int32) int32 {
	s1 := len(src)
	b := src[t:]
	a := src[s:s1]
	b = b[:len(a)]
	// Extend the match to be as long as possible.
	for i := range a {
		if a[i] != b[i] {
			return int32(i)
		}
	}
	return int32(len(a))
}

func load3232(b []byte, i int32) uint32 {
	// Help the compiler eliminate bounds checks on the read so it can be done in a single read.
	b = b[i:]
	b = b[:4]
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func load6432(b []byte, i int32) uint64 {
	// Help the compiler eliminate bounds checks on the read so it can be done in a single read.
	b = b[i:]
	b = b[:8]
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
}

func load64(b []byte, i int) uint64 {
	// Help the compiler eliminate bounds checks on the read so it can be done in a single read.
	b = b[i:]
	b = b[:8]
	return uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
}
