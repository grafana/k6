package fast

import "math"

const (
	diyFpKSignificandSize        = 64
	kSignificandSize             = 53
	kUint64MSB            uint64 = 1 << 63

	kSignificandMask = 0x000FFFFFFFFFFFFF
	kHiddenBit       = 0x0010000000000000
	kExponentMask    = 0x7FF0000000000000

	kPhysicalSignificandSize = 52 // Excludes the hidden bit.
	kExponentBias            = 0x3FF + kPhysicalSignificandSize
	kDenormalExponent        = -kExponentBias + 1
)

type double float64

type diyfp struct {
	f uint64
	e int
}

// f =- o.
// The exponents of both numbers must be the same and the significand of this
// must be bigger than the significand of other.
// The result will not be normalized.
func (f *diyfp) subtract(o diyfp) {
	_DCHECK(f.e == o.e)
	_DCHECK(f.f >= o.f)
	f.f -= o.f
}

// Returns f - o
// The exponents of both numbers must be the same and this must be bigger
// than other. The result will not be normalized.
func (f diyfp) minus(o diyfp) diyfp {
	res := f
	res.subtract(o)
	return res
}

// f *= o
func (f *diyfp) mul(o diyfp) {
	// Simply "emulates" a 128 bit multiplication.
	// However: the resulting number only contains 64 bits. The least
	// significant 64 bits are only used for rounding the most significant 64
	// bits.
	const kM32 uint64 = 0xFFFFFFFF
	a := f.f >> 32
	b := f.f & kM32
	c := o.f >> 32
	d := o.f & kM32
	ac := a * c
	bc := b * c
	ad := a * d
	bd := b * d
	tmp := (bd >> 32) + (ad & kM32) + (bc & kM32)
	// By adding 1U << 31 to tmp we round the final result.
	// Halfway cases will be round up.
	tmp += 1 << 31
	result_f := ac + (ad >> 32) + (bc >> 32) + (tmp >> 32)
	f.e += o.e + 64
	f.f = result_f
}

// Returns f * o
func (f diyfp) times(o diyfp) diyfp {
	res := f
	res.mul(o)
	return res
}

func (f *diyfp) _normalize() {
	f_, e := f.f, f.e
	// This method is mainly called for normalizing boundaries. In general
	// boundaries need to be shifted by 10 bits. We thus optimize for this case.
	const k10MSBits uint64 = 0x3FF << 54
	for f_&k10MSBits == 0 {
		f_ <<= 10
		e -= 10
	}
	for f_&kUint64MSB == 0 {
		f_ <<= 1
		e--
	}
	f.f, f.e = f_, e
}

func normalizeDiyfp(f diyfp) diyfp {
	res := f
	res._normalize()
	return res
}

// f must be strictly greater than 0.
func (d double) toNormalizedDiyfp() diyfp {
	f, e := d.sigExp()

	// The current float could be a denormal.
	for (f & kHiddenBit) == 0 {
		f <<= 1
		e--
	}
	// Do the final shifts in one go.
	f <<= diyFpKSignificandSize - kSignificandSize
	e -= diyFpKSignificandSize - kSignificandSize
	return diyfp{f, e}
}

// Returns the two boundaries of this.
// The bigger boundary (m_plus) is normalized. The lower boundary has the same
// exponent as m_plus.
// Precondition: the value encoded by this Double must be greater than 0.
func (d double) normalizedBoundaries() (m_minus, m_plus diyfp) {
	v := d.toDiyFp()
	significand_is_zero := v.f == kHiddenBit
	m_plus = normalizeDiyfp(diyfp{f: (v.f << 1) + 1, e: v.e - 1})
	if significand_is_zero && v.e != kDenormalExponent {
		// The boundary is closer. Think of v = 1000e10 and v- = 9999e9.
		// Then the boundary (== (v - v-)/2) is not just at a distance of 1e9 but
		// at a distance of 1e8.
		// The only exception is for the smallest normal: the largest denormal is
		// at the same distance as its successor.
		// Note: denormals have the same exponent as the smallest normals.
		m_minus = diyfp{f: (v.f << 2) - 1, e: v.e - 2}
	} else {
		m_minus = diyfp{f: (v.f << 1) - 1, e: v.e - 1}
	}
	m_minus.f <<= m_minus.e - m_plus.e
	m_minus.e = m_plus.e
	return
}

func (d double) toDiyFp() diyfp {
	f, e := d.sigExp()
	return diyfp{f: f, e: e}
}

func (d double) sigExp() (significand uint64, exponent int) {
	d64 := math.Float64bits(float64(d))
	significand = d64 & kSignificandMask
	if d64&kExponentMask != 0 { // not denormal
		significand += kHiddenBit
		exponent = int((d64&kExponentMask)>>kPhysicalSignificandSize) - kExponentBias
	} else {
		exponent = kDenormalExponent
	}
	return
}
