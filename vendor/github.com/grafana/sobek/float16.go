package sobek

import "math"

// Float64ToFloat16 converts f to the nearest IEEE 754 binary16 (half precision) value, rounding ties
// to even, and returns its bits. This is the conversion used when a value is stored into a Float16Array.
// It converts from float64 directly, as converting through float32 first could round twice.
func Float64ToFloat16(f float64) uint16 {
	sign := uint16(math.Float64bits(f)>>48) & 0x8000
	if math.IsNaN(f) {
		return 0x7e00
	}
	a := math.Abs(f)
	if a >= 65520 { // halfway between the largest finite value (65504) and 2^16, rounds to infinity
		return sign | 0x7c00
	}
	if a < 0x1p-14 { // subnormal (or zero): the value is m * 2^-24
		return sign | uint16(math.RoundToEven(a*0x1p24))
	}
	frac, exp := math.Frexp(a) // a = frac * 2^exp, frac in [0.5, 1)
	m := math.RoundToEven(frac * 2048)
	if m == 2048 {
		m = 1024
		exp++
	}
	return sign | uint16(exp+14)<<10 | (uint16(m) - 1024)
}

// Float16ToFloat64 returns the value of the IEEE 754 binary16 (half precision) number with the given
// bits, such as an element of an exported Float16Array.
func Float16ToFloat64(b uint16) float64 {
	exp := int(b>>10) & 0x1f
	m := float64(b & 0x3ff)
	var f float64
	switch exp {
	case 0:
		f = m * 0x1p-24
	case 0x1f:
		if m != 0 {
			return math.NaN()
		}
		f = math.Inf(1)
	default:
		f = math.Ldexp(1024+m, exp-25)
	}
	if b&0x8000 != 0 {
		f = -f
	}
	return f
}
