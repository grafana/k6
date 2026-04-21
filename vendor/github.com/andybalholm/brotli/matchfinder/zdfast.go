package matchfinder

import (
	"encoding/binary"
	"math"
)

const (
	zdfastLongTableBits = 17
	zdfastLongTableSize = 1 << zdfastLongTableBits
)

// ZDFast is a MatchFinder based on the "Default" setting in
// github.com/klauspost/compress/zstd.
type ZDFast struct {
	MaxDistance int

	history []byte
	// current is the offset at the start of history
	current   int32
	table     [zfastTableSize]tableEntry
	longTable [zdfastLongTableSize]tableEntry
}

func (z *ZDFast) Reset() {
	z.current = 0
	z.table = [zfastTableSize]tableEntry{}
	z.longTable = [zdfastLongTableSize]tableEntry{}
	z.history = z.history[:0]
}

func (z *ZDFast) FindMatches(dst []Match, src []byte) []Match {
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
		for i := range z.longTable {
			v := z.longTable[i].offset
			if v < minOffset {
				v = 0
			} else {
				v = v - z.current + int32(z.MaxDistance)
			}
			z.longTable[i].offset = v
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

	if len(src) < 16 {
		return append(dst, Match{
			Unmatched: len(src),
		})
	}

	src = z.history
	sLimit := int32(len(src)) - 10

	const stepSize = 1

	nextEmit := s
	cv := binary.LittleEndian.Uint64(src[s:])
	var offset1, offset2 int32

mainLoop:
	for {
		// t will contain the match offset when we find one.
		// When exiting the search loop, we have already checked 4 bytes.
		var t int32

		for {
			nextHashL := z.hashLong(cv)
			nextHashS := z.hashShort(cv)
			candidateL := z.longTable[nextHashL]
			candidateS := z.table[nextHashS]

			repIndex := s - offset1 + 1

			entry := tableEntry{offset: s + z.current, val: uint32(cv)}
			z.longTable[nextHashL] = entry
			z.table[nextHashS] = entry

			if offset1 != 0 && repIndex >= 0 && binary.LittleEndian.Uint32(src[repIndex:]) == uint32(cv>>8) {
				// There is a repeated match at s+1.
				end := extendMatch(src, int(repIndex+4), int(s+5))
				start := s + 1
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

			coffsetL := s - (candidateL.offset - z.current)
			coffsetS := s - (candidateS.offset - z.current)
			if coffsetL < int32(z.MaxDistance) && uint32(cv) == candidateL.val {
				t = candidateL.offset - z.current
				if binary.LittleEndian.Uint32(src[t:]) == uint32(cv) {
					// found a long match (likely at least 8 bytes)
					break
				}
			}
			if coffsetS < int32(z.MaxDistance) && uint32(cv) == candidateS.val {
				t = candidateS.offset - z.current
				if binary.LittleEndian.Uint32(src[t:]) != uint32(cv) {
					goto noMatch
				}
				// Found a regular match.
				// See if we can find a long match at s+1
				cv := binary.LittleEndian.Uint64(src[s+1:])
				nextHashL = z.hashLong(cv)
				candidateL = z.longTable[nextHashL]
				coffsetL = s - (candidateL.offset - z.current) + 1
				z.longTable[nextHashL] = tableEntry{offset: s + 1 + z.current, val: uint32(cv)}
				if coffsetL < int32(z.MaxDistance) && uint32(cv) == candidateL.val {
					t = candidateL.offset - z.current
					if binary.LittleEndian.Uint32(src[t:]) == uint32(cv) {
						// We found a long match at s+1, so we'll use that instead
						// of the regular match at s.
						s++
						break
					}
				}

				t = candidateS.offset - z.current
				break
			}
		noMatch:

			s += stepSize + ((s - nextEmit) >> 7)
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
		prevS := s
		s = int32(end)
		nextEmit = s
		if s >= sLimit {
			break mainLoop
		}

		// Store some table entries near the start and end of the match.
		index0 := prevS + 1
		index1 := s - 2
		cv0 := binary.LittleEndian.Uint64(src[index0:])
		cv1 := binary.LittleEndian.Uint64(src[index1:])
		te0 := tableEntry{offset: index0 + z.current, val: uint32(cv0)}
		te1 := tableEntry{offset: index1 + z.current, val: uint32(cv1)}
		z.longTable[z.hashLong(cv0)] = te0
		z.longTable[z.hashLong(cv1)] = te1
		cv0 >>= 8
		cv1 >>= 8
		te0.offset++
		te1.offset++
		te0.val = uint32(cv0)
		te1.val = uint32(cv1)
		z.table[z.hashShort(cv0)] = te0
		z.table[z.hashShort(cv1)] = te1

		cv = binary.LittleEndian.Uint64(src[s:])

		// Check offset 2
		if o2 := s - offset2; offset2 != 0 && binary.LittleEndian.Uint32(src[o2:]) == uint32(cv) {
			end := extendMatch(src, int(o2+4), int(s+4))

			// Store the hashes, since we have them.
			nextHashS := z.hashShort(cv)
			nextHashL := z.hashLong(cv)
			entry := tableEntry{offset: s + z.current, val: uint32(cv)}
			z.table[nextHashS] = entry
			z.longTable[nextHashL] = entry
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

func (z *ZDFast) hashShort(u uint64) uint32 {
	return uint32(((u << 24) * 889523592379) >> (64 - zfastTableBits))
}

func (z *ZDFast) hashLong(u uint64) uint32 {
	return uint32((u * 0xcf1bbcdcb7a56463) >> (64 - zdfastLongTableBits))
}
