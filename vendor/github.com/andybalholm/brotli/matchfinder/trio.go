package matchfinder

import "encoding/binary"

// Trio is a MatchFinder that uses 3 different hash lengths, and
// overlap parsing.
type Trio struct {
	MaxDistance int
	history     []byte
	table5      [1 << 16]tableEntry
	table8      [1 << 17]tableEntry
	table12     [1 << 18]tableEntry
}

func (z *Trio) Reset() {
	z.table5 = [len(z.table5)]tableEntry{}
	z.table8 = [len(z.table8)]tableEntry{}
	z.table12 = [len(z.table12)]tableEntry{}
	z.history = z.history[:0]
}

func (z *Trio) FindMatches(dst []Match, src []byte) []Match {
	if z.MaxDistance == 0 {
		z.MaxDistance = 1 << 16
	}

	if len(z.history) > z.MaxDistance*2 {
		delta := len(z.history) - z.MaxDistance
		copy(z.history, z.history[delta:])
		z.history = z.history[:z.MaxDistance]

		for i := range z.table5 {
			v := z.table5[i].offset
			v -= int32(delta)
			if v < 0 {
				z.table5[i] = tableEntry{}
			} else {
				z.table5[i].offset = v
			}
		}
		for i := range z.table8 {
			v := z.table8[i].offset
			v -= int32(delta)
			if v < 0 {
				z.table8[i] = tableEntry{}
			} else {
				z.table8[i].offset = v
			}
		}
		for i := range z.table12 {
			v := z.table12[i].offset
			v -= int32(delta)
			if v < 0 {
				z.table12[i] = tableEntry{}
			} else {
				z.table12[i].offset = v
			}
		}
	}

	if len(src) < 20 {
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

	sLimit := int32(len(src)) - 14

mainLoop:
	for {
		// Search for a match, starting after the last match emitted.
		s := int32(e.NextEmit)
		if s > sLimit {
			break mainLoop
		}

		// t will contain the match offset when we find one.
		var t int32
		var hashLengthFound int

		cv := binary.LittleEndian.Uint64(src[s:])
		extra := binary.LittleEndian.Uint32(src[s+8:])

		for {
			nextHash12 := z.hash12(cv, extra)
			nextHash8 := z.hash8(cv)
			nextHash5 := z.hash5(cv)
			candidate12 := z.table12[nextHash12]
			candidate8 := z.table8[nextHash8]
			candidate5 := z.table5[nextHash5]

			entry := tableEntry{offset: s, val: uint32(cv)}
			z.table12[nextHash12] = entry
			z.table8[nextHash8] = entry
			z.table5[nextHash5] = entry

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

			if candidate12.offset < s && s-candidate12.offset < int32(z.MaxDistance) && uint32(cv) == candidate12.val &&
				binary.LittleEndian.Uint32(src[candidate12.offset:]) == uint32(cv) {
				// There is a 12-byte match at s.
				t = candidate12.offset
				hashLengthFound = 12
				break
			}
			if candidate8.offset < s && s-candidate8.offset < int32(z.MaxDistance) && uint32(cv) == candidate8.val &&
				binary.LittleEndian.Uint32(src[candidate8.offset:]) == uint32(cv) {
				t = candidate8.offset
				hashLengthFound = 8
				break
			}
			if candidate5.offset < s && s-candidate5.offset < int32(z.MaxDistance) && uint32(cv) == candidate5.val &&
				binary.LittleEndian.Uint32(src[candidate5.offset:]) == uint32(cv) {
				t = candidate5.offset
				hashLengthFound = 5
				break
			}

			s += 1 + ((s - int32(e.NextEmit)) >> 7)
			if s > sLimit {
				break mainLoop
			}
			cv = binary.LittleEndian.Uint64(src[s:])
			extra = binary.LittleEndian.Uint32(src[s+8:])
		}

		if hashLengthFound != 0 && hashLengthFound < 12 {
			// Look for a "lazy" match with a longer hash at s+1.
			cv := binary.LittleEndian.Uint64(src[s+1:])
			extra := binary.LittleEndian.Uint32(src[s+9:])
			nextHash12 := z.hash12(cv, extra)
			nextHash8 := z.hash8(cv)
			candidate12 := z.table12[nextHash12]
			candidate8 := z.table8[nextHash8]
			coffset12 := s - candidate12.offset + 1
			coffset8 := s - candidate8.offset + 1
			entry := tableEntry{offset: s + 1, val: uint32(cv)}
			z.table12[nextHash12] = entry
			z.table8[nextHash8] = entry
			if candidate12.offset < s+1 && coffset12 < int32(z.MaxDistance) && uint32(cv) == candidate12.val &&
				binary.LittleEndian.Uint32(src[candidate12.offset:]) == uint32(cv) {
				t = candidate12.offset
				s++
			} else if hashLengthFound < 8 && candidate8.offset < s+1 && coffset8 < int32(z.MaxDistance) && uint32(cv) == candidate8.val &&
				binary.LittleEndian.Uint32(src[candidate8.offset:]) == uint32(cv) {
				t = candidate8.offset
				s++
			}
		}

		currentMatch := extendMatch2(src, int(s), int(t), e.NextEmit)
		matches[0] = currentMatch

		index0 := s + 1

		// We have a match in matches[0].
		// Now look for overlapping matches.

		for {
			if matches[0].End > int(sLimit) {
				break
			}
			s = int32(max(matches[0].Start+2, matches[0].End-10))

			// Store some entries that haven't been indexed yet.
			for index0 < s-1 {
				cv0 := binary.LittleEndian.Uint64(src[index0:])
				extra0 := binary.LittleEndian.Uint32(src[index0+8:])
				te0 := tableEntry{offset: index0, val: uint32(cv0)}
				z.table5[z.hash5(cv0)] = te0
				z.table8[z.hash8(cv0)] = te0
				z.table12[z.hash12(cv0, extra0)] = te0
				index0++
			}

			cv = binary.LittleEndian.Uint64(src[s:])
			extra = binary.LittleEndian.Uint32(src[s+8:])

			nextHash12 := z.hash12(cv, extra)
			nextHash8 := z.hash8(cv)
			nextHash5 := z.hash5(cv)
			candidate12 := z.table12[nextHash12]
			candidate8 := z.table8[nextHash8]

			entry := tableEntry{offset: s, val: uint32(cv)}
			z.table12[nextHash12] = entry
			z.table8[nextHash8] = entry
			z.table5[nextHash5] = entry

			t = -1
			if candidate12.offset < s && s-candidate12.offset < int32(z.MaxDistance) && uint32(cv) == candidate12.val &&
				binary.LittleEndian.Uint32(src[candidate12.offset:]) == uint32(cv) {
				// There is a 12-byte match at s.
				t = candidate12.offset
			} else if candidate8.offset < s && s-candidate8.offset < int32(z.MaxDistance) && uint32(cv) == candidate8.val &&
				binary.LittleEndian.Uint32(src[candidate8.offset:]) == uint32(cv) {
				// There is a long match at s.
				t = candidate8.offset
			}

			index0 = s + 1

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

		// Store some entries up to the end of the last match.
		for index0 < int32(matches[0].End) && index0 < sLimit {
			cv0 := binary.LittleEndian.Uint64(src[index0:])
			extra0 := binary.LittleEndian.Uint32(src[index0+8:])
			te0 := tableEntry{offset: index0, val: uint32(cv0)}
			z.table5[z.hash5(cv0)] = te0
			z.table8[z.hash8(cv0)] = te0
			z.table12[z.hash12(cv0, extra0)] = te0
			index0++
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

func (z *Trio) hash5(u uint64) uint32 {
	return uint32(((u << 24) * 889523592379) >> (64 - 16))
}

func (z *Trio) hash8(u uint64) uint32 {
	return uint32((u * 0xcf1bbcdcb7a56463) >> (64 - 17))
}

func (z *Trio) hash12(u uint64, e uint32) uint32 {
	return uint32((u*0xcf1bbcdcb7a56463 + uint64(e)*(2654435761<<32)) >> (64 - 18))
}
