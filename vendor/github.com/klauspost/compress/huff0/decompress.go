package huff0

import (
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/fse"
)

type dTable struct {
	single []dEntrySingle
	double []dEntryDouble
}

// single-symbols decoding
type dEntrySingle struct {
	byte  uint8
	nBits uint8
}

// double-symbols decoding
type dEntryDouble struct {
	seq   uint16
	nBits uint8
	len   uint8
}

// ReadTable will read a table from the input.
// The size of the input may be larger than the table definition.
// Any content remaining after the table definition will be returned.
// If no Scratch is provided a new one is allocated.
// The returned Scratch can be used for decoding input using this table.
func ReadTable(in []byte, s *Scratch) (s2 *Scratch, remain []byte, err error) {
	s, err = s.prepare(in)
	if err != nil {
		return s, nil, err
	}
	if len(in) <= 1 {
		return s, nil, errors.New("input too small for table")
	}
	iSize := in[0]
	in = in[1:]
	if iSize >= 128 {
		// Uncompressed
		oSize := iSize - 127
		iSize = (oSize + 1) / 2
		if int(iSize) > len(in) {
			return s, nil, errors.New("input too small for table")
		}
		for n := uint8(0); n < oSize; n += 2 {
			v := in[n/2]
			s.huffWeight[n] = v >> 4
			s.huffWeight[n+1] = v & 15
		}
		s.symbolLen = uint16(oSize)
		in = in[iSize:]
	} else {
		if len(in) <= int(iSize) {
			return s, nil, errors.New("input too small for table")
		}
		// FSE compressed weights
		s.fse.DecompressLimit = 255
		hw := s.huffWeight[:]
		s.fse.Out = hw
		b, err := fse.Decompress(in[:iSize], s.fse)
		s.fse.Out = nil
		if err != nil {
			return s, nil, err
		}
		if len(b) > 255 {
			return s, nil, errors.New("corrupt input: output table too large")
		}
		s.symbolLen = uint16(len(b))
		in = in[iSize:]
	}

	// collect weight stats
	var rankStats [tableLogMax + 1]uint32
	weightTotal := uint32(0)
	for _, v := range s.huffWeight[:s.symbolLen] {
		if v > tableLogMax {
			return s, nil, errors.New("corrupt input: weight too large")
		}
		rankStats[v]++
		weightTotal += (1 << (v & 15)) >> 1
	}
	if weightTotal == 0 {
		return s, nil, errors.New("corrupt input: weights zero")
	}

	// get last non-null symbol weight (implied, total must be 2^n)
	{
		tableLog := highBit32(weightTotal) + 1
		if tableLog > tableLogMax {
			return s, nil, errors.New("corrupt input: tableLog too big")
		}
		s.actualTableLog = uint8(tableLog)
		// determine last weight
		{
			total := uint32(1) << tableLog
			rest := total - weightTotal
			verif := uint32(1) << highBit32(rest)
			lastWeight := highBit32(rest) + 1
			if verif != rest {
				// last value must be a clean power of 2
				return s, nil, errors.New("corrupt input: last value not power of two")
			}
			s.huffWeight[s.symbolLen] = uint8(lastWeight)
			s.symbolLen++
			rankStats[lastWeight]++
		}
	}

	if (rankStats[1] < 2) || (rankStats[1]&1 != 0) {
		// by construction : at least 2 elts of rank 1, must be even
		return s, nil, errors.New("corrupt input: min elt size, even check failed ")
	}

	// TODO: Choose between single/double symbol decoding

	// Calculate starting value for each rank
	{
		var nextRankStart uint32
		for n := uint8(1); n < s.actualTableLog+1; n++ {
			current := nextRankStart
			nextRankStart += rankStats[n] << (n - 1)
			rankStats[n] = current
		}
	}

	// fill DTable (always full size)
	tSize := 1 << tableLogMax
	if len(s.dt.single) != tSize {
		s.dt.single = make([]dEntrySingle, tSize)
	}

	for n, w := range s.huffWeight[:s.symbolLen] {
		length := (uint32(1) << w) >> 1
		d := dEntrySingle{
			byte:  uint8(n),
			nBits: s.actualTableLog + 1 - w,
		}
		for u := rankStats[w]; u < rankStats[w]+length; u++ {
			s.dt.single[u] = d
		}
		rankStats[w] += length
	}
	return s, in, nil
}

// Decompress1X will decompress a 1X encoded stream.
// The length of the supplied input must match the end of a block exactly.
// Before this is called, the table must be initialized with ReadTable unless
// the encoder re-used the table.
func (s *Scratch) Decompress1X(in []byte) (out []byte, err error) {
	if len(s.dt.single) == 0 {
		return nil, errors.New("no table loaded")
	}
	var br bitReader
	err = br.init(in)
	if err != nil {
		return nil, err
	}
	s.Out = s.Out[:0]

	decode := func() byte {
		val := br.peekBitsFast(s.actualTableLog) /* note : actualTableLog >= 1 */
		v := s.dt.single[val]
		br.bitsRead += v.nBits
		return v.byte
	}
	hasDec := func(v dEntrySingle) byte {
		br.bitsRead += v.nBits
		return v.byte
	}

	// Avoid bounds check by always having full sized table.
	const tlSize = 1 << tableLogMax
	const tlMask = tlSize - 1
	dt := s.dt.single[:tlSize]

	// Use temp table to avoid bound checks/append penalty.
	var tmp = s.huffWeight[:256]
	var off uint8

	for br.off >= 8 {
		br.fillFast()
		tmp[off+0] = hasDec(dt[br.peekBitsFast(s.actualTableLog)&tlMask])
		tmp[off+1] = hasDec(dt[br.peekBitsFast(s.actualTableLog)&tlMask])
		br.fillFast()
		tmp[off+2] = hasDec(dt[br.peekBitsFast(s.actualTableLog)&tlMask])
		tmp[off+3] = hasDec(dt[br.peekBitsFast(s.actualTableLog)&tlMask])
		off += 4
		if off == 0 {
			s.Out = append(s.Out, tmp...)
		}
	}

	s.Out = append(s.Out, tmp[:off]...)

	for !br.finished() {
		br.fill()
		s.Out = append(s.Out, decode())
	}
	return s.Out, br.close()
}

// Decompress4X will decompress a 4X encoded stream.
// Before this is called, the table must be initialized with ReadTable unless
// the encoder re-used the table.
// The length of the supplied input must match the end of a block exactly.
// The destination size of the uncompressed data must be known and provided.
func (s *Scratch) Decompress4X(in []byte, dstSize int) (out []byte, err error) {
	if len(s.dt.single) == 0 {
		return nil, errors.New("no table loaded")
	}
	if len(in) < 6+(4*1) {
		return nil, errors.New("input too small")
	}
	// TODO: We do not detect when we overrun a buffer, except if the last one does.

	var br [4]bitReader
	start := 6
	for i := 0; i < 3; i++ {
		length := int(in[i*2]) | (int(in[i*2+1]) << 8)
		if start+length >= len(in) {
			return nil, errors.New("truncated input (or invalid offset)")
		}
		err = br[i].init(in[start : start+length])
		if err != nil {
			return nil, err
		}
		start += length
	}
	err = br[3].init(in[start:])
	if err != nil {
		return nil, err
	}

	// Prepare output
	if cap(s.Out) < dstSize {
		s.Out = make([]byte, 0, dstSize)
	}
	s.Out = s.Out[:dstSize]
	// destination, offset to match first output
	dstOut := s.Out
	dstEvery := (dstSize + 3) / 4

	decode := func(br *bitReader) byte {
		val := br.peekBitsFast(s.actualTableLog) /* note : actualTableLog >= 1 */
		v := s.dt.single[val]
		br.bitsRead += v.nBits
		return v.byte
	}

	// Use temp table to avoid bound checks/append penalty.
	var tmp = s.huffWeight[:256]
	var off uint8

	// Decode 2 values from each decoder/loop.
	const bufoff = 256 / 4
bigloop:
	for {
		for i := range br {
			if br[i].off < 4 {
				break bigloop
			}
			br[i].fillFast()
		}
		tmp[off] = decode(&br[0])
		tmp[off+bufoff] = decode(&br[1])
		tmp[off+bufoff*2] = decode(&br[2])
		tmp[off+bufoff*3] = decode(&br[3])
		tmp[off+1] = decode(&br[0])
		tmp[off+1+bufoff] = decode(&br[1])
		tmp[off+1+bufoff*2] = decode(&br[2])
		tmp[off+1+bufoff*3] = decode(&br[3])
		off += 2
		if off == bufoff {
			if bufoff > dstEvery {
				return nil, errors.New("corruption detected: stream overrun")
			}
			copy(dstOut, tmp[:bufoff])
			copy(dstOut[dstEvery:], tmp[bufoff:bufoff*2])
			copy(dstOut[dstEvery*2:], tmp[bufoff*2:bufoff*3])
			copy(dstOut[dstEvery*3:], tmp[bufoff*3:bufoff*4])
			off = 0
			dstOut = dstOut[bufoff:]
			// There must at least be 3 buffers left.
			if len(dstOut) < dstEvery*3+3 {
				return nil, errors.New("corruption detected: stream overrun")
			}
		}
	}
	if off > 0 {
		ioff := int(off)
		if len(dstOut) < dstEvery*3+ioff {
			return nil, errors.New("corruption detected: stream overrun")
		}
		copy(dstOut, tmp[:off])
		copy(dstOut[dstEvery:dstEvery+ioff], tmp[bufoff:bufoff*2])
		copy(dstOut[dstEvery*2:dstEvery*2+ioff], tmp[bufoff*2:bufoff*3])
		copy(dstOut[dstEvery*3:dstEvery*3+ioff], tmp[bufoff*3:bufoff*4])
		dstOut = dstOut[off:]
	}

	for i := range br {
		offset := dstEvery * i
		br := &br[i]
		for !br.finished() {
			br.fill()
			if offset >= len(dstOut) {
				return nil, errors.New("corruption detected: stream overrun")
			}
			dstOut[offset] = decode(br)
			offset++
		}
		err = br.close()
		if err != nil {
			return nil, err
		}
	}

	return s.Out, nil
}

// matches will compare a decoding table to a coding table.
// Errors are written to the writer.
// Nothing will be written if table is ok.
func (s *Scratch) matches(ct cTable, w io.Writer) {
	if s == nil || len(s.dt.single) == 0 {
		return
	}
	dt := s.dt.single[:1<<s.actualTableLog]
	tablelog := s.actualTableLog
	ok := 0
	broken := 0
	for sym, enc := range ct {
		errs := 0
		broken++
		if enc.nBits == 0 {
			for _, dec := range dt {
				if dec.byte == byte(sym) {
					fmt.Fprintf(w, "symbol %x has decoder, but no encoder\n", sym)
					errs++
					break
				}
			}
			if errs == 0 {
				broken--
			}
			continue
		}
		// Unused bits in input
		ub := tablelog - enc.nBits
		top := enc.val << ub
		// decoder looks at top bits.
		dec := dt[top]
		if dec.nBits != enc.nBits {
			fmt.Fprintf(w, "symbol 0x%x bit size mismatch (enc: %d, dec:%d).\n", sym, enc.nBits, dec.nBits)
			errs++
		}
		if dec.byte != uint8(sym) {
			fmt.Fprintf(w, "symbol 0x%x decoder output mismatch (enc: %d, dec:%d).\n", sym, sym, dec.byte)
			errs++
		}
		if errs > 0 {
			fmt.Fprintf(w, "%d errros in base, stopping\n", errs)
			continue
		}
		// Ensure that all combinations are covered.
		for i := uint16(0); i < (1 << ub); i++ {
			vval := top | i
			dec := dt[vval]
			if dec.nBits != enc.nBits {
				fmt.Fprintf(w, "symbol 0x%x bit size mismatch (enc: %d, dec:%d).\n", vval, enc.nBits, dec.nBits)
				errs++
			}
			if dec.byte != uint8(sym) {
				fmt.Fprintf(w, "symbol 0x%x decoder output mismatch (enc: %d, dec:%d).\n", vval, sym, dec.byte)
				errs++
			}
			if errs > 20 {
				fmt.Fprintf(w, "%d errros, stopping\n", errs)
				break
			}
		}
		if errs == 0 {
			ok++
			broken--
		}
	}
	if broken > 0 {
		fmt.Fprintf(w, "%d broken, %d ok\n", broken, ok)
	}
}
