package matchfinder

import "encoding/binary"

const (
	zmTableBits     = 15
	zmTableSize     = 1 << zmTableBits
	zmLongTableBits = 17
	zmLongTableSize = 1 << zmLongTableBits
)

// ZM is a MatchFinder that combines the cache tables of ZDFast with the
// overlap-based parsing of M4.
type ZM struct {
	MaxDistance int
	history     []byte
	table       [zmTableSize]tableEntry
	longTable   [zmLongTableSize]tableEntry
}

func (z *ZM) Reset() {
	z.table = [zmTableSize]tableEntry{}
	z.longTable = [zmLongTableSize]tableEntry{}
	z.history = z.history[:0]
}

func (z *ZM) FindMatches(dst []Match, src []byte) []Match {
	if z.MaxDistance == 0 {
		z.MaxDistance = 1 << 16
	}

	if len(z.history) > z.MaxDistance*2 {
		delta := len(z.history) - z.MaxDistance
		copy(z.history, z.history[delta:])
		z.history = z.history[:z.MaxDistance]

		for i := range z.table {
			v := z.table[i].offset
			v -= int32(delta)
			if v < 0 {
				z.table[i] = tableEntry{}
			} else {
				z.table[i].offset = v
			}
		}
		for i := range z.longTable {
			v := z.longTable[i].offset
			v -= int32(delta)
			if v < 0 {
				z.longTable[i] = tableEntry{}
			} else {
				z.longTable[i].offset = v
			}
		}
	}

	if len(src) < 16 {
		return append(dst, Match{
			Unmatched: len(src),
		})
	}

	e := matchEmitter{
		Dst:      dst,
		NextEmit: len(z.history),
	}
	z.history = append(z.history, src...)
	src = z.history

	// matches stores the matches that have been found but not emitted,
	// in reverse order. (matches[0] is the most recent one.)
	var matches [3]absoluteMatch

	sLimit := int32(len(src)) - 10

mainLoop:
	for {
		// Search for a match, starting after the last match emitted.
		s := int32(e.NextEmit)
		if s > sLimit {
			break mainLoop
		}

		// t will contain the match offset when we find one.
		var t int32

		cv := binary.LittleEndian.Uint64(src[s:])

		for {
			nextHashL := z.hashLong(cv)
			nextHashS := z.hashShort(cv)
			candidateL := z.longTable[nextHashL]
			candidateS := z.table[nextHashS]

			entry := tableEntry{offset: s, val: uint32(cv)}
			z.longTable[nextHashL] = entry
			z.table[nextHashS] = entry

			// Look for a repeat match one byte after the current position.
			if len(e.Dst) > 0 {
				prevDistance := int32(e.Dst[len(e.Dst)-1].Distance)
				if prevDistance != 0 {
					repIndex := s - prevDistance + 1
					if repIndex >= 0 && binary.LittleEndian.Uint32(src[repIndex:]) == uint32(cv>>8) {
						// There is a repeated match at s+2.
						s++
						t = repIndex
						break
					}
				}
			}

			if candidateL.offset < s && s-candidateL.offset < int32(z.MaxDistance) && uint32(cv) == candidateL.val &&
				binary.LittleEndian.Uint32(src[candidateL.offset:]) == uint32(cv) {
				// There is a long match at s.
				t = candidateL.offset
				break
			}
			if candidateS.offset < s && s-candidateS.offset < int32(z.MaxDistance) && uint32(cv) == candidateS.val &&
				binary.LittleEndian.Uint32(src[candidateS.offset:]) == uint32(cv) {
				// There is a regular match at s.
				// See if we can find a long match at s+1.
				cv := binary.LittleEndian.Uint64(src[s+1:])
				nextHashL = z.hashLong(cv)
				candidateL = z.longTable[nextHashL]
				coffsetL := s - candidateL.offset + 1
				z.longTable[nextHashL] = tableEntry{offset: s + 1, val: uint32(cv)}
				if candidateL.offset < s+1 && coffsetL < int32(z.MaxDistance) && uint32(cv) == candidateL.val &&
					binary.LittleEndian.Uint32(src[candidateL.offset:]) == uint32(cv) {
					// We found a long match at s+1, so we'll use that instead
					// of the regular match at s.
					t = candidateL.offset
					s++
					break
				}

				t = candidateS.offset
				break
			}

			s += 1 + ((s - int32(e.NextEmit)) >> 7)
			if s > sLimit {
				break mainLoop
			}
			cv = binary.LittleEndian.Uint64(src[s:])
		}

		currentMatch := extendMatch2(src, int(s), int(t), e.NextEmit)
		matches[0] = currentMatch

		// Store some table entries after s.
		index0 := s + 1
		cv0 := binary.LittleEndian.Uint64(src[index0:])
		te0 := tableEntry{offset: index0, val: uint32(cv0)}
		z.longTable[z.hashLong(cv0)] = te0
		cv0 >>= 8
		te0.offset++
		te0.val = uint32(cv0)
		z.table[z.hashShort(cv0)] = te0

		// We have a match in matches[0].
		// Now look for overlapping matches.

		for {
			if matches[0].End > int(sLimit) {
				break
			}
			s = int32(max(matches[0].Start+2, matches[0].End-6))
			cv = binary.LittleEndian.Uint64(src[s:])

			nextHashL := z.hashLong(cv)
			nextHashS := z.hashShort(cv)
			candidateL := z.longTable[nextHashL]
			candidateS := z.table[nextHashS]

			entry := tableEntry{offset: s, val: uint32(cv)}
			z.longTable[nextHashL] = entry
			z.table[nextHashS] = entry

			t = -1
			if candidateL.offset < s && s-candidateL.offset < int32(z.MaxDistance) && uint32(cv) == candidateL.val &&
				binary.LittleEndian.Uint32(src[candidateL.offset:]) == uint32(cv) {
				// There is a long match at s.
				t = candidateL.offset
			} else if candidateS.offset < s && s-candidateS.offset < int32(z.MaxDistance) && uint32(cv) == candidateS.val &&
				binary.LittleEndian.Uint32(src[candidateS.offset:]) == uint32(cv) {
				// There is a regular match at s.
				t = candidateS.offset
				// See if we can find a long match at s+1.
				cv := binary.LittleEndian.Uint64(src[s+1:])
				nextHashL = z.hashLong(cv)
				candidateL = z.longTable[nextHashL]
				coffsetL := s - candidateL.offset + 1
				z.longTable[nextHashL] = tableEntry{offset: s + 1, val: uint32(cv)}
				if candidateL.offset < s+1 && coffsetL < int32(z.MaxDistance) && uint32(cv) == candidateL.val &&
					binary.LittleEndian.Uint32(src[candidateL.offset:]) == uint32(cv) {
					// We found a long match at s+1, so we'll use that instead
					// of the regular match at s.
					t = candidateL.offset
					s++
				}
			}

			if t == -1 {
				// No overlapping match was found.
				break
			}

			newMatch := extendMatch2(src, int(s), int(t), e.NextEmit)

			if newMatch.End-newMatch.Start <= matches[0].End-matches[0].Start {
				// The new match isn't longer than the old one, so we break out of the loop
				// of looking for overlapping matches.
				break
			}

			matches = [3]absoluteMatch{
				newMatch,
				matches[0],
				matches[1],
			}

			if matches[2] == (absoluteMatch{}) {
				continue
			}

			// We have three matches, so it's time to emit one and/or eliminate one.
			switch {
			case matches[0].Start < matches[2].End:
				// The first and third matches overlap; discard the one in between.
				matches = [3]absoluteMatch{
					matches[0],
					matches[2],
					{},
				}

			case matches[0].Start < matches[2].End+4:
				// The first and third matches don't overlap, but there's no room for
				// another match between them. Emit the first match and discard the second.
				e.emit(matches[2])
				matches = [3]absoluteMatch{
					matches[0],
					{},
					{},
				}

			default:
				// Emit the first match, shortening it if necessary to avoid overlap with the second.
				if matches[2].End > matches[1].Start {
					matches[2].End = matches[1].Start
				}
				if matches[2].End-matches[2].Start >= 4 {
					e.emit(matches[2])
				}
				matches[2] = absoluteMatch{}
			}
		}

		// Store some table entries at the end of the last match.
		index1 := int32(matches[0].End - 2)
		if index1 < sLimit {
			cv1 := binary.LittleEndian.Uint64(src[index1:])
			te1 := tableEntry{offset: index1, val: uint32(cv1)}
			z.longTable[z.hashLong(cv1)] = te1
			cv1 >>= 8
			te1.offset++
			te1.val = uint32(cv1)
			z.table[z.hashShort(cv1)] = te1
		}

		// We're done looking for overlapping matches; emit the ones we have.

		if matches[1] != (absoluteMatch{}) {
			if matches[1].End > matches[0].Start {
				matches[1].End = matches[0].Start
			}
			if matches[1].End-matches[1].Start >= 4 {
				e.emit(matches[1])
			}
		}
		e.emit(matches[0])
		matches = [3]absoluteMatch{}
	}

	dst = e.Dst
	if e.NextEmit < len(src) {
		dst = append(dst, Match{
			Unmatched: len(src) - e.NextEmit,
		})
	}

	return dst
}

func (z *ZM) hashShort(u uint64) uint32 {
	return uint32(((u << 24) * 889523592379) >> (64 - zmTableBits))
}

func (z *ZM) hashLong(u uint64) uint32 {
	return uint32((u * 0xcf1bbcdcb7a56463) >> (64 - zmLongTableBits))
}
