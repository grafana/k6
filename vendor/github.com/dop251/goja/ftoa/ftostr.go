package ftoa

import (
	"math"
	"strconv"

	"github.com/dop251/goja/ftoa/internal/fast"
)

type FToStrMode int

const (
	// Either fixed or exponential format; round-trip
	ModeStandard FToStrMode = iota
	// Always exponential format; round-trip
	ModeStandardExponential
	// Round to <precision> digits after the decimal point; exponential if number is large
	ModeFixed
	// Always exponential format; <precision> significant digits
	ModeExponential
	// Either fixed or exponential format; <precision> significant digits
	ModePrecision
)

func insert(b []byte, p int, c byte) []byte {
	b = append(b, 0)
	copy(b[p+1:], b[p:])
	b[p] = c
	return b
}

func expand(b []byte, delta int) []byte {
	newLen := len(b) + delta
	if newLen <= cap(b) {
		return b[:newLen]
	}
	b1 := make([]byte, newLen)
	copy(b1, b)
	return b1
}

func FToStr(d float64, mode FToStrMode, precision int, buffer []byte) []byte {
	if math.IsNaN(d) {
		buffer = append(buffer, "NaN"...)
		return buffer
	}
	if math.IsInf(d, 0) {
		if math.Signbit(d) {
			buffer = append(buffer, '-')
		}
		buffer = append(buffer, "Infinity"...)
		return buffer
	}

	if mode == ModeFixed && (d >= 1e21 || d <= -1e21) {
		mode = ModeStandard
	}

	var decPt int
	var ok bool
	startPos := len(buffer)

	if d != 0 { // also matches -0
		if d < 0 {
			buffer = append(buffer, '-')
			d = -d
			startPos++
		}
		switch mode {
		case ModeStandard, ModeStandardExponential:
			buffer, decPt, ok = fast.Dtoa(d, fast.ModeShortest, 0, buffer)
		case ModeExponential, ModePrecision:
			buffer, decPt, ok = fast.Dtoa(d, fast.ModePrecision, precision, buffer)
		}
	} else {
		buffer = append(buffer, '0')
		decPt, ok = 1, true
	}
	if !ok {
		buffer, decPt = ftoa(d, dtoaModes[mode], mode >= ModeFixed, precision, buffer)
	}
	exponentialNotation := false
	minNDigits := 0 /* Minimum number of significand digits required by mode and precision */
	nDigits := len(buffer) - startPos

	switch mode {
	case ModeStandard:
		if decPt < -5 || decPt > 21 {
			exponentialNotation = true
		} else {
			minNDigits = decPt
		}
	case ModeFixed:
		if precision >= 0 {
			minNDigits = decPt + precision
		} else {
			minNDigits = decPt
		}
	case ModeExponential:
		//                    JS_ASSERT(precision > 0);
		minNDigits = precision
		fallthrough
	case ModeStandardExponential:
		exponentialNotation = true
	case ModePrecision:
		//                    JS_ASSERT(precision > 0);
		minNDigits = precision
		if decPt < -5 || decPt > precision {
			exponentialNotation = true
		}
	}

	for nDigits < minNDigits {
		buffer = append(buffer, '0')
		nDigits++
	}

	if exponentialNotation {
		/* Insert a decimal point if more than one significand digit */
		if nDigits != 1 {
			buffer = insert(buffer, startPos+1, '.')
		}
		buffer = append(buffer, 'e')
		if decPt-1 >= 0 {
			buffer = append(buffer, '+')
		}
		buffer = strconv.AppendInt(buffer, int64(decPt-1), 10)
	} else if decPt != nDigits {
		/* Some kind of a fraction in fixed notation */
		//                JS_ASSERT(decPt <= nDigits);
		if decPt > 0 {
			/* dd...dd . dd...dd */
			buffer = insert(buffer, startPos+decPt, '.')
		} else {
			/* 0 . 00...00dd...dd */
			buffer = expand(buffer, 2-decPt)
			copy(buffer[startPos+2-decPt:], buffer[startPos:])
			buffer[startPos] = '0'
			buffer[startPos+1] = '.'
			for i := startPos + 2; i < startPos+2-decPt; i++ {
				buffer[i] = '0'
			}
		}
	}

	return buffer
}
