package matchfinder

import (
	"encoding/binary"
	"math"
)

type tableEntry struct {
	val    uint32
	offset int32
}

const (
	zfastTableBits = 15
	zfastTableSize = 1 << zfastTableBits
	zfastHashLen   = 6

	prime6Bytes = 227718039650203
)

// ZFast is a MatchFinder based on the "Fastest" setting in
// github.com/klauspost/compress/zstd.
type ZFast struct {
	MaxDistance int

	history []byte
	// current is the offset at the start of history
	current int32
	table   [zfastTableSize]tableEntry
}

func (z *ZFast) Reset() {
	z.current = 0
	z.table = [zfastTableSize]tableEntry{}
	z.history = z.history[:0]
}

func (z *ZFast) FindMatches(dst []Match, src []byte) []Match {
	if z.MaxDistance == 0 {
		z.MaxDistance = 1 << 16
	}

	// Protect against overflow of current.
	if int(z.current) >= int(math.MaxInt32)-2*z.MaxDistance-len(z.history) {
		minOffset := z.current + int32(len(z.history)) - int32(z.MaxDistance)
		for i := range z.table {
			v := z.table[i].offset
			if v < minOffset {
				v = 0
			} else {
				v = v - z.current + int32(z.MaxDistance)
			}
			z.table[i].offset = v
		}
		z.current = int32(z.MaxDistance)
	}

	if len(z.history)+len(src) > cap(z.history) {
		// history doesn't have enough capacity to hold the new block.
		if cap(z.history) == 0 {
			historySize := max(2*z.MaxDistance, 1<<20, len(src))
			z.history = make([]byte, 0, historySize)
		} else {
			// Move down
			offset := len(z.history) - z.MaxDistance
			copy(z.history[:z.MaxDistance], z.history[offset:])
			z.current += int32(offset)
			z.history = z.history[:z.MaxDistance]
		}
	}
	s := int32(len(z.history))
	z.history = append(z.history, src...)

	if len(src) < 10 {
		return append(dst, Match{
			Unmatched: len(src),
		})
	}

	src = z.history
	sLimit := int32(len(src)) - 8

	const stepSize = 2

	nextEmit := s
	cv := binary.LittleEndian.Uint64(src[s:])
	var offset1, offset2 int32

mainLoop:
	for {
		// t will contain the match offset when we find one.
		// When exiting the search loop, we have already checked 4 bytes.
		var t int32

		for {
			nextHash := z.hash(cv)
			nextHash2 := z.hash(cv >> 8)
			candidate := z.table[nextHash]
			candidate2 := z.table[nextHash2]
			repIndex := s - offset1 + 2

			z.table[nextHash] = tableEntry{offset: s + z.current, val: uint32(cv)}
			z.table[nextHash2] = tableEntry{offset: s + z.current + 1, val: uint32(cv >> 8)}

			if offset1 != 0 && repIndex >= 0 && binary.LittleEndian.Uint32(src[repIndex:]) == uint32(cv>>16) {
				// There is a repeated match at s+2.
				end := extendMatch(src, int(repIndex+4), int(s+6))
				start := s + 2
				for repIndex > 0 && start > nextEmit && src[repIndex-1] == src[start-1] {
					repIndex--
					start--
				}
				dst = append(dst, Match{
					Unmatched: int(start - nextEmit),
					Length:    end - int(start),
					Distance:  int(start - repIndex),
				})
				s = int32(end)
				nextEmit = s
				if s >= sLimit {
					break mainLoop
				}
				cv = binary.LittleEndian.Uint64(src[s:])
				continue
			}

			coffset0 := s - (candidate.offset - z.current)
			coffset1 := s - (candidate2.offset - z.current) + 1
			if coffset0 < int32(z.MaxDistance) && uint32(cv) == candidate.val {
				t = candidate.offset - z.current
				if binary.LittleEndian.Uint32(src[t:]) == uint32(cv) {
					// found a regular match
					break
				}
			}
			if coffset1 < int32(z.MaxDistance) && uint32(cv>>8) == candidate2.val {
				t = candidate2.offset - z.current
				if binary.LittleEndian.Uint32(src[t:]) == uint32(cv>>8) {
					s++
					break
				}
			}

			s += stepSize + ((s - nextEmit) >> 5)
			if s > sLimit {
				break mainLoop
			}
			cv = binary.LittleEndian.Uint64(src[s:])
		}

		// A 4-byte match has been found. We'll later see if more than
		// 4 bytes.
		offset2 = offset1
		offset1 = s - t

		end := extendMatch(src, int(t+4), int(s+4))
		for t > 0 && s > nextEmit && src[t-1] == src[s-1] {
			s--
			t--
		}

		dst = append(dst, Match{
			Unmatched: int(s - nextEmit),
			Length:    end - int(s),
			Distance:  int(s - t),
		})
		s = int32(end)
		nextEmit = s
		if s >= sLimit {
			break mainLoop
		}
		cv = binary.LittleEndian.Uint64(src[s:])

		// Check offset 2
		if o2 := s - offset2; offset2 != 0 && binary.LittleEndian.Uint32(src[o2:]) == uint32(cv) {
			end := extendMatch(src, int(o2+4), int(s+4))

			// Store the hash, since we have it.
			nextHash := z.hash(cv)
			z.table[nextHash] = tableEntry{offset: s + z.current, val: uint32(cv)}
			dst = append(dst, Match{
				Length:   end - int(s),
				Distance: int(offset2),
			})
			s = int32(end)
			nextEmit = s
			offset1, offset2 = offset2, offset1
			if s >= sLimit {
				break mainLoop
			}
			cv = binary.LittleEndian.Uint64(src[s:])
		}
	}

	if int(nextEmit) < len(src) {
		dst = append(dst, Match{
			Unmatched: len(src) - int(nextEmit),
		})
	}

	return dst
}

func (z *ZFast) hash(u uint64) uint32 {
	return uint32(((u << 16) * prime6Bytes) >> (64 - zfastTableBits))
}
