package brotli

import (
	"math"

	"github.com/andybalholm/brotli/matchfinder"
)

func gaussianProbability(x, mean, stdDev float64) float64 {
	return math.Exp(-(x-mean)*(x-mean)/(2*stdDev*stdDev)) / math.Sqrt(2*math.Pi*stdDev*stdDev)
}

// A FastEncoder implements the matchfinder.Encoder interface, writing in Brotli
// format. It uses a simplified encoding (like level 0 in the reference
// implementation) to save time.
type FastEncoder struct {
	wroteHeader   bool
	bw            bitWriter
	commandHisto  [704]uint32
	distanceHisto [64]uint32
}

func (e *FastEncoder) Reset() {
	e.wroteHeader = false
	e.bw = bitWriter{}
}

func (e *FastEncoder) Encode(dst []byte, src []byte, matches []matchfinder.Match, lastBlock bool) []byte {
	e.bw.dst = dst
	if !e.wroteHeader {
		e.bw.writeBits(4, 15)
		e.wroteHeader = true

		// Fill the histograms with default statistics.

		// For the command codes we're using for insert lengths (insert + 2-byte copy),
		// fill the histogram with a Zipf-squared distribution.
		for i := range 24 {
			e.commandHisto[combineLengthCodes(uint16(i), 0, false)] = uint32(2000 / (i + 1) / (i + 1))
		}

		// For the command codes we're using for copy lengths (0 insert + copy
		// (length - 2), with repeat distance),
		// fill the histogram with Zipf distribution starting at code 1 (match length 5),
		// but a smaller frequency for code 0.
		e.commandHisto[combineLengthCodes(0, 0, true)] = 50
		for i := 1; i < 24; i++ {
			e.commandHisto[combineLengthCodes(0, uint16(i), i < 16)] = uint32(800 / i)
		}

		// Fill in the combined codes for short insert and copy lengths.
		for insertCode := range 6 {
			copyCode := 2
			e.commandHisto[128+insertCode<<3+copyCode] = uint32(100 / (insertCode + 1) / (insertCode + 1) / copyCode)
			for copyCode := 3; copyCode < 8; copyCode++ {
				e.commandHisto[128+insertCode<<3+copyCode] = uint32(343 / (insertCode + 1) / (insertCode + 1) / copyCode)
			}
		}

		// Fill e.distanceHisto with a normal distribution.
		e.distanceHisto[0] = 100
		for i := 16; i < 64; i++ {
			e.distanceHisto[i] = max(uint32(gaussianProbability(float64(i), 32, 8)*10000), 1)
		}
	}

	if len(src) == 0 {
		if lastBlock {
			e.bw.writeBits(2, 3) // islast + isempty
			e.bw.jumpToByteBoundary()
			return e.bw.dst
		}
		return dst
	}

	var literalHisto [256]uint32
	for _, c := range src {
		literalHisto[c]++
	}

	storeMetaBlockHeaderBW(uint(len(src)), false, &e.bw)
	e.bw.writeBits(13, 0)

	var literalDepths [256]byte
	var literalBits [256]uint16
	buildAndStoreHuffmanTreeFastBW(literalHisto[:], uint(len(src)), 8, literalDepths[:], literalBits[:], &e.bw)

	var commandDepths [704]byte
	var commandBits [704]uint16
	commandCount := 0
	for _, n := range e.commandHisto {
		commandCount += int(n)
	}
	buildAndStoreHuffmanTreeFastBW(e.commandHisto[:], uint(commandCount), 10, commandDepths[:], commandBits[:], &e.bw)

	var distanceDepths [64]byte
	var distanceBits [64]uint16
	distanceCount := 0
	for _, n := range e.distanceHisto {
		distanceCount += int(n)
	}
	buildAndStoreHuffmanTreeFastBW(e.distanceHisto[:], uint(distanceCount), 6, distanceDepths[:], distanceBits[:], &e.bw)

	// Reset the statistics, starting with a count of 1 for each symbol we might use.
	for i := range 24 {
		e.commandHisto[combineLengthCodes(uint16(i), 0, false)] = 1
	}
	for i := range 24 {
		e.commandHisto[combineLengthCodes(0, uint16(i), i < 16)] = 1
	}
	for insertCode := range 6 {
		for copyCode := 2; copyCode < 8; copyCode++ {
			e.commandHisto[128+insertCode<<3+copyCode] = 1
		}
	}
	e.distanceHisto[0] = 1
	for i := 16; i < 64; i++ {
		e.distanceHisto[i] = 1
	}

	pos := 0
	for i, m := range matches {
		lengthFinished := false
		// Write a command with the appropriate insert length, and a copy length of 2.
		if m.Unmatched < 6 {
			var command int
			if m.Length < 10 && m.Length != 0 {
				// We can use a combined insert/copy code with no extra bits.
				command = m.Unmatched<<3 + m.Length - 2 + 128
				lengthFinished = true
			} else {
				command = m.Unmatched<<3 + 128
			}
			e.bw.writeBits(uint(commandDepths[command]), uint64(commandBits[command]))
			e.commandHisto[command]++
		} else {
			insertCode := getInsertLengthCode(uint(m.Unmatched))
			command := combineLengthCodes(insertCode, 0, false)
			e.bw.writeBits(uint(commandDepths[command]), uint64(commandBits[command]))
			e.bw.writeBits(uint(kInsExtra[insertCode]), uint64(m.Unmatched)-uint64(kInsBase[insertCode]))
			e.commandHisto[command]++
		}

		// Write the literals, if any.
		if m.Unmatched > 0 {
			for _, c := range src[pos : pos+m.Unmatched] {
				e.bw.writeBits(uint(literalDepths[c]), uint64(literalBits[c]))
			}
		}

		if m.Length != 0 {
			// Write the distance code.
			var distCode distanceCode
			if i == 0 || m.Distance != matches[i-1].Distance {
				distCode = getDistanceCode(m.Distance)
			}
			e.bw.writeBits(uint(distanceDepths[distCode.code]), uint64(distanceBits[distCode.code]))
			if distCode.nExtra > 0 {
				e.bw.writeBits(distCode.nExtra, distCode.extraBits)
			}
			e.distanceHisto[distCode.code]++

			// Write a command for the remainder of the match (after the first two bytes
			// from before), using the previous distance.
			switch {
			case lengthFinished:
				// We don't need to finish the length.
			case m.Length < 12:
				command := m.Length - 4
				e.bw.writeBits(uint(commandDepths[command]), uint64(commandBits[command]))
				e.commandHisto[command]++
			case m.Length < 72:
				copyCode := getCopyLengthCode(uint(m.Length - 2))
				command := combineLengthCodes(0, copyCode, true)
				e.bw.writeBits(uint(commandDepths[command]), uint64(commandBits[command]))
				e.bw.writeBits(uint(kCopyExtra[copyCode]), uint64(m.Length-2)-uint64(kCopyBase[copyCode]))
				e.commandHisto[command]++
			default:
				copyCode := getCopyLengthCode(uint(m.Length - 2))
				command := combineLengthCodes(0, copyCode, false)
				e.bw.writeBits(uint(commandDepths[command]), uint64(commandBits[command]))
				e.bw.writeBits(uint(kCopyExtra[copyCode]), uint64(m.Length-2)-uint64(kCopyBase[copyCode]))
				e.bw.writeBits(uint(distanceDepths[0]), uint64(distanceBits[0]))
				e.commandHisto[command]++
				e.distanceHisto[0]++
			}
		}

		pos += m.Unmatched + m.Length
	}

	if lastBlock {
		e.bw.writeBits(2, 3) // islast + isempty
		e.bw.jumpToByteBoundary()
	}
	return e.bw.dst
}
