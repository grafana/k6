package matchfinder

import (
	"encoding/binary"
	"math"
	"math/bits"
	"slices"
)

const (
	bargain1TableBits = 18
	bargain1TableSize = 1 << bargain1TableBits
)

// Bargain1 is a MatchFinder that attempts to find the encoding with the lowest
// "bit cost", using 1 hash length (6).
type Bargain1 struct {
	MaxDistance int

	// Skip is whether to look for matches at every other byte instead of every
	// byte (to increase speed but decrease compression).
	Skip bool

	history []byte
	table6  [bargain1TableSize]tableEntry

	// holding onto buffers to reduce allocations:

	arrivals []arrival
	matches  []Match
}

func (z *Bargain1) Reset() {
	z.table6 = [bargain1TableSize]tableEntry{}
	z.history = z.history[:0]
}

func (z *Bargain1) FindMatches(dst []Match, src []byte) []Match {
	if z.MaxDistance == 0 {
		z.MaxDistance = 1 << 16
	}

	var histogram [256]uint32
	for _, b := range src {
		histogram[b]++
	}
	var byteCost [256]float32
	for b, n := range histogram {
		cost := max(math.Log2(float64(len(src))/float64(n)), 1)
		byteCost[b] = float32(cost)
	}

	// Each element in arrivals corresponds to the position just after
	// the corresponding byte in src.
	arrivals := z.arrivals
	if len(arrivals) < len(src) {
		arrivals = make([]arrival, len(src))
		z.arrivals = arrivals
	} else {
		arrivals = arrivals[:len(src)]
		for i := range arrivals {
			arrivals[i] = arrival{}
		}
	}

	if len(z.history) > z.MaxDistance*2 {
		delta := len(z.history) - z.MaxDistance
		copy(z.history, z.history[delta:])
		z.history = z.history[:z.MaxDistance]

		for i := range z.table6 {
			v := z.table6[i].offset
			v -= int32(delta)
			if v < 0 {
				z.table6[i] = tableEntry{}
			} else {
				z.table6[i].offset = v
			}
		}
	}

	historyLen := len(z.history)
	z.history = append(z.history, src...)
	src = z.history

	addMatch := func(m absoluteMatch, unmatched int, repeat bool) {
		var startCost float32
		if m.Start > historyLen {
			startCost = arrivals[m.Start-historyLen-1].cost
		}
		insertCost := float32(bits.Len(uint(unmatched)))
		var distanceCost float32
		if !repeat {
			distanceCost = float32(bits.Len(uint(m.Start - m.Match)))
		}
		cost := startCost + baseMatchCost + insertCost + distanceCost
		for j := m.End; j >= m.Start+3; j-- {
			a := &arrivals[j-historyLen-1]
			if a.cost > 0 && a.cost <= cost {
				break
			}
			*a = arrival{
				length:   uint32(j - m.Start),
				distance: uint32(m.Start - m.Match),
				cost:     cost,
			}
		}
	}

	var nextOverlapSearch int

	for i := historyLen; i < len(src); i++ {
		var arrivedHere arrival
		if i > historyLen {
			arrivedHere = arrivals[i-historyLen-1]
		}

		unmatched := 0
		if arrivedHere.distance == 0 {
			unmatched = int(arrivedHere.length)
		}
		prevDistance := 0
		if unmatched != 0 && i-unmatched > historyLen {
			prevDistance = int(arrivals[i-historyLen-1-unmatched].distance)
		}

		literalCost := byteCost[src[i]]
		nextArrival := &arrivals[i-historyLen]
		if nextArrival.cost == 0 || arrivedHere.cost+literalCost < nextArrival.cost {
			*nextArrival = arrival{
				cost:   arrivedHere.cost + literalCost,
				length: uint32(unmatched + 1),
			}
		}

		if i > len(src)-8 {
			// There's no room to check hashes.
			continue
		}

		cv := binary.LittleEndian.Uint64(src[i:])
		nextHash6 := z.hash6(cv)
		candidate6 := z.table6[nextHash6]

		entry := tableEntry{offset: int32(i), val: uint32(cv)}
		z.table6[nextHash6] = entry

		// Look for a repeat match, unless there is no previous distance, or a match at
		// that distance has already been found.
		if prevDistance != 0 && prevDistance != int(arrivals[i-historyLen-1+4].distance) {
			repIndex := i - prevDistance
			if repIndex >= 0 && binary.LittleEndian.Uint32(src[repIndex:]) == uint32(cv) {
				// We have a repeat of the previous match distance.
				m := extendMatch2(src, i, repIndex, i)
				addMatch(m, unmatched, true)
			}
		}

		if z.Skip && i%2 != 0 {
			continue
		}

		nextByteIsUnmatched := arrivals[i-historyLen-1+1].distance == 0

		if unmatched > 0 || i >= nextOverlapSearch || nextByteIsUnmatched {
			if int(candidate6.offset) < i && i-int(candidate6.offset) < z.MaxDistance && uint32(cv) == candidate6.val &&
				binary.LittleEndian.Uint32(src[candidate6.offset:]) == uint32(cv) {
				m := extendMatch2(src, i, int(candidate6.offset), historyLen)
				delta := i - m.Start
				if delta == 0 {
					addMatch(m, unmatched, false)
				} else {
					// The match was extended backwards. Add it with and without the extra.
					addMatch(m, max(unmatched-delta, 0), false)
					m.Start += delta
					m.Match += delta
					addMatch(m, unmatched, false)
				}
				nextOverlapSearch = max(nextOverlapSearch, m.Start+1, m.End-4)
			}
		}
	}

	// We've found the shortest path; now walk it backward and store the matches.
	matches := z.matches[:0]
	i := len(arrivals) - 1
	for i >= 0 {
		a := arrivals[i]
		if a.distance > 0 {
			matches = append(matches, Match{
				Length:   int(a.length),
				Distance: int(a.distance),
			})
			i -= int(a.length)
		} else {
			if len(matches) == 0 {
				matches = append(matches, Match{})
			}
			matches[len(matches)-1].Unmatched = int(a.length)
			i -= int(a.length)
		}
	}
	z.matches = matches

	slices.Reverse(matches)

	return append(dst, matches...)
}

func (z *Bargain1) hash6(u uint64) uint32 {
	return uint32(((u << 16) * 227718039650203) >> (64 - bargain1TableBits))
}
