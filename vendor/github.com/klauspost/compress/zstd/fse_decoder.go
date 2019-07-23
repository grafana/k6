// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

import (
	"errors"
	"fmt"
)

const (
	tablelogAbsoluteMax = 9
)

const (
	/*!MEMORY_USAGE :
	 *  Memory usage formula : N->2^N Bytes (examples : 10 -> 1KB; 12 -> 4KB ; 16 -> 64KB; 20 -> 1MB; etc.)
	 *  Increasing memory usage improves compression ratio
	 *  Reduced memory usage can improve speed, due to cache effect
	 *  Recommended max value is 14, for 16KB, which nicely fits into Intel x86 L1 cache */
	maxMemoryUsage = 11

	maxTableLog    = maxMemoryUsage - 2
	maxTablesize   = 1 << maxTableLog
	maxTableMask   = (1 << maxTableLog) - 1
	minTablelog    = 5
	maxSymbolValue = 255
)

// fseDecoder provides temporary storage for compression and decompression.
type fseDecoder struct {
	dt             [maxTablesize]decSymbol // Decompression table.
	symbolLen      uint16                  // Length of active part of the symbol table.
	actualTableLog uint8                   // Selected tablelog.
	maxBits        uint8                   // Maximum number of additional bits

	// used for table creation to avoid allocations.
	stateTable [256]uint16
	norm       [maxSymbolValue + 1]int16
	preDefined bool
}

// tableStep returns the next table index.
func tableStep(tableSize uint32) uint32 {
	return (tableSize >> 1) + (tableSize >> 3) + 3
}

// readNCount will read the symbol distribution so decoding tables can be constructed.
func (s *fseDecoder) readNCount(b *byteReader, maxSymbol uint16) error {
	var (
		charnum   uint16
		previous0 bool
	)
	if b.remain() < 4 {
		return errors.New("input too small")
	}
	bitStream := b.Uint32()
	nbBits := uint((bitStream & 0xF) + minTablelog) // extract tableLog
	if nbBits > tablelogAbsoluteMax {
		println("Invalid tablelog:", nbBits)
		return errors.New("tableLog too large")
	}
	bitStream >>= 4
	bitCount := uint(4)

	s.actualTableLog = uint8(nbBits)
	remaining := int32((1 << nbBits) + 1)
	threshold := int32(1 << nbBits)
	gotTotal := int32(0)
	nbBits++

	for remaining > 1 && charnum <= maxSymbol {
		if previous0 {
			//println("prev0")
			n0 := charnum
			for (bitStream & 0xFFFF) == 0xFFFF {
				//println("24 x 0")
				n0 += 24
				if r := b.remain(); r > 5 {
					b.advance(2)
					bitStream = b.Uint32() >> bitCount
				} else {
					// end of bit stream
					bitStream >>= 16
					bitCount += 16
				}
			}
			//printf("bitstream: %d, 0b%b", bitStream&3, bitStream)
			for (bitStream & 3) == 3 {
				n0 += 3
				bitStream >>= 2
				bitCount += 2
			}
			n0 += uint16(bitStream & 3)
			bitCount += 2

			if n0 > maxSymbolValue {
				return errors.New("maxSymbolValue too small")
			}
			//println("inserting ", n0-charnum, "zeroes from idx", charnum, "ending before", n0)
			for charnum < n0 {
				s.norm[uint8(charnum)] = 0
				charnum++
			}

			if r := b.remain(); r >= 7 || r+int(bitCount>>3) >= 4 {
				b.advance(bitCount >> 3)
				bitCount &= 7
				bitStream = b.Uint32() >> bitCount
			} else {
				bitStream >>= 2
			}
		}

		max := (2*threshold - 1) - remaining
		var count int32

		if int32(bitStream)&(threshold-1) < max {
			count = int32(bitStream) & (threshold - 1)
			if debug && nbBits < 1 {
				panic("nbBits underflow")
			}
			bitCount += nbBits - 1
		} else {
			count = int32(bitStream) & (2*threshold - 1)
			if count >= threshold {
				count -= max
			}
			bitCount += nbBits
		}

		// extra accuracy
		count--
		if count < 0 {
			// -1 means +1
			remaining += count
			gotTotal -= count
		} else {
			remaining -= count
			gotTotal += count
		}
		s.norm[charnum&0xff] = int16(count)
		charnum++
		previous0 = count == 0
		for remaining < threshold {
			nbBits--
			threshold >>= 1
		}

		//println("b.off:", b.off, "len:", len(b.b), "bc:", bitCount, "remain:", b.remain())
		if r := b.remain(); r >= 7 || r+int(bitCount>>3) >= 4 {
			b.advance(bitCount >> 3)
			bitCount &= 7
		} else {
			bitCount -= (uint)(8 * (len(b.b) - 4 - b.off))
			b.off = len(b.b) - 4
			//println("b.off:", b.off, "len:", len(b.b), "bc:", bitCount, "iend", iend)
		}
		bitStream = b.Uint32() >> (bitCount & 31)
		//printf("bitstream is now: 0b%b", bitStream)
	}
	s.symbolLen = charnum
	if s.symbolLen <= 1 {
		return fmt.Errorf("symbolLen (%d) too small", s.symbolLen)
	}
	if s.symbolLen > maxSymbolValue+1 {
		return fmt.Errorf("symbolLen (%d) too big", s.symbolLen)
	}
	if remaining != 1 {
		return fmt.Errorf("corruption detected (remaining %d != 1)", remaining)
	}
	if bitCount > 32 {
		return fmt.Errorf("corruption detected (bitCount %d > 32)", bitCount)
	}
	if gotTotal != 1<<s.actualTableLog {
		return fmt.Errorf("corruption detected (total %d != %d)", gotTotal, 1<<s.actualTableLog)
	}
	b.advance((bitCount + 7) >> 3)
	// println(s.norm[:s.symbolLen], s.symbolLen)
	return s.buildDtable()
}

// decSymbol contains information about a state entry,
// Including the state offset base, the output symbol and
// the number of bits to read for the low part of the destination state.
type decSymbol struct {
	newState uint16
	addBits  uint8 // Used for symbols until transformed.
	nbBits   uint8
	baseline uint32
}

// decSymbolValue returns the transformed decSymbol for the given symbol.
func decSymbolValue(symb uint8, t []baseOffset) (decSymbol, error) {
	if int(symb) >= len(t) {
		return decSymbol{}, fmt.Errorf("rle symbol %d >= max %d", symb, len(t))
	}
	lu := t[symb]
	return decSymbol{
		addBits:  lu.addBits,
		baseline: lu.baseLine,
	}, nil
}

// setRLE will set the decoder til RLE mode.
func (s *fseDecoder) setRLE(symbol decSymbol) {
	s.actualTableLog = 0
	s.maxBits = symbol.addBits
	s.dt[0] = symbol
}

// buildDtable will build the decoding table.
func (s *fseDecoder) buildDtable() error {
	tableSize := uint32(1 << s.actualTableLog)
	highThreshold := tableSize - 1
	symbolNext := s.stateTable[:256]

	// Init, lay down lowprob symbols
	{
		for i, v := range s.norm[:s.symbolLen] {
			if v == -1 {
				s.dt[highThreshold].addBits = uint8(i)
				highThreshold--
				symbolNext[i] = 1
			} else {
				symbolNext[i] = uint16(v)
			}
		}
	}
	// Spread symbols
	{
		tableMask := tableSize - 1
		step := tableStep(tableSize)
		position := uint32(0)
		for ss, v := range s.norm[:s.symbolLen] {
			for i := 0; i < int(v); i++ {
				s.dt[position].addBits = uint8(ss)
				position = (position + step) & tableMask
				for position > highThreshold {
					// lowprob area
					position = (position + step) & tableMask
				}
			}
		}
		if position != 0 {
			// position must reach all cells once, otherwise normalizedCounter is incorrect
			return errors.New("corrupted input (position != 0)")
		}
	}

	// Build Decoding table
	{
		tableSize := uint16(1 << s.actualTableLog)
		for u, v := range s.dt[:tableSize] {
			symbol := v.addBits
			nextState := symbolNext[symbol]
			symbolNext[symbol] = nextState + 1
			nBits := s.actualTableLog - byte(highBits(uint32(nextState)))
			s.dt[u&maxTableMask].nbBits = nBits
			newState := (nextState << nBits) - tableSize
			if newState > tableSize {
				return fmt.Errorf("newState (%d) outside table size (%d)", newState, tableSize)
			}
			if newState == uint16(u) && nBits == 0 {
				// Seems weird that this is possible with nbits > 0.
				return fmt.Errorf("newState (%d) == oldState (%d) and no bits", newState, u)
			}
			s.dt[u&maxTableMask].newState = newState
		}
	}
	return nil
}

// transform will transform the decoder table into a table usable for
// decoding without having to apply the transformation while decoding.
// The state will contain the base value and the number of bits to read.
func (s *fseDecoder) transform(t []baseOffset) error {
	tableSize := uint16(1 << s.actualTableLog)
	s.maxBits = 0
	for i, v := range s.dt[:tableSize] {
		if int(v.addBits) >= len(t) {
			return fmt.Errorf("invalid decoding table entry %d, symbol %d >= max (%d)", i, v.addBits, len(t))
		}
		lu := t[v.addBits]
		if lu.addBits > s.maxBits {
			s.maxBits = lu.addBits
		}
		s.dt[i&maxTableMask] = decSymbol{
			newState: v.newState,
			nbBits:   v.nbBits,
			addBits:  lu.addBits,
			baseline: lu.baseLine,
		}
	}
	return nil
}

type fseState struct {
	// TODO: Check if *[1 << maxTablelog]decSymbol is faster.
	dt    []decSymbol
	state decSymbol
}

// Initialize and decodeAsync first state and symbol.
func (s *fseState) init(br *bitReader, tableLog uint8, dt []decSymbol) {
	s.dt = dt
	br.fill()
	s.state = dt[br.getBits(tableLog)]
}

// next returns the current symbol and sets the next state.
// At least tablelog bits must be available in the bit reader.
func (s *fseState) next(br *bitReader) {
	lowBits := uint16(br.getBits(s.state.nbBits))
	s.state = s.dt[s.state.newState+lowBits]
}

// finished returns true if all bits have been read from the bitstream
// and the next state would require reading bits from the input.
func (s *fseState) finished(br *bitReader) bool {
	return br.finished() && s.state.nbBits > 0
}

// final returns the current state symbol without decoding the next.
func (s *fseState) final() (int, uint8) {
	return int(s.state.baseline), s.state.addBits
}

// nextFast returns the next symbol and sets the next state.
// This can only be used if no symbols are 0 bits.
// At least tablelog bits must be available in the bit reader.
func (s *fseState) nextFast(br *bitReader) (uint32, uint8) {
	lowBits := uint16(br.getBitsFast(s.state.nbBits))
	s.state = s.dt[s.state.newState+lowBits]
	return s.state.baseline, s.state.addBits
}
