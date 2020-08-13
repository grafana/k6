/*
Package ftoa provides ECMAScript-compliant floating point number conversion to string.

It contains code ported from Rhino (https://github.com/mozilla/rhino/blob/master/src/org/mozilla/javascript/DToA.java)
as well as from the original code by David M. Gay.

See LICENSE_LUCENE for the original copyright message and disclaimer.

*/
package ftoa

import (
	"math"
)

const (
	frac_mask = 0xfffff
	exp_shift = 20
	exp_msk1  = 0x100000

	exp_shiftL       = 52
	exp_mask_shifted = 0x7ff
	frac_maskL       = 0xfffffffffffff
	exp_msk1L        = 0x10000000000000
	exp_shift1       = 20
	exp_mask         = 0x7ff00000
	bias             = 1023
	p                = 53
	bndry_mask       = 0xfffff
	log2P            = 1
)

func lo0bits(x uint32) (k int) {

	if (x & 7) != 0 {
		if (x & 1) != 0 {
			return 0
		}
		if (x & 2) != 0 {
			return 1
		}
		return 2
	}
	if (x & 0xffff) == 0 {
		k = 16
		x >>= 16
	}
	if (x & 0xff) == 0 {
		k += 8
		x >>= 8
	}
	if (x & 0xf) == 0 {
		k += 4
		x >>= 4
	}
	if (x & 0x3) == 0 {
		k += 2
		x >>= 2
	}
	if (x & 1) == 0 {
		k++
		x >>= 1
		if (x & 1) == 0 {
			return 32
		}
	}
	return
}

func hi0bits(x uint32) (k int) {

	if (x & 0xffff0000) == 0 {
		k = 16
		x <<= 16
	}
	if (x & 0xff000000) == 0 {
		k += 8
		x <<= 8
	}
	if (x & 0xf0000000) == 0 {
		k += 4
		x <<= 4
	}
	if (x & 0xc0000000) == 0 {
		k += 2
		x <<= 2
	}
	if (x & 0x80000000) == 0 {
		k++
		if (x & 0x40000000) == 0 {
			return 32
		}
	}
	return
}

func stuffBits(bits []byte, offset int, val uint32) {
	bits[offset] = byte(val >> 24)
	bits[offset+1] = byte(val >> 16)
	bits[offset+2] = byte(val >> 8)
	bits[offset+3] = byte(val)
}

func d2b(d float64, b []byte) (e, bits int, dblBits []byte) {
	dBits := math.Float64bits(d)
	d0 := uint32(dBits >> 32)
	d1 := uint32(dBits)

	z := d0 & frac_mask
	d0 &= 0x7fffffff /* clear sign bit, which we ignore */

	var de, k, i int
	if de = int(d0 >> exp_shift); de != 0 {
		z |= exp_msk1
	}

	y := d1
	if y != 0 {
		dblBits = b[:8]
		k = lo0bits(y)
		y >>= k
		if k != 0 {
			stuffBits(dblBits, 4, y|z<<(32-k))
			z >>= k
		} else {
			stuffBits(dblBits, 4, y)
		}
		stuffBits(dblBits, 0, z)
		if z != 0 {
			i = 2
		} else {
			i = 1
		}
	} else {
		dblBits = b[:4]
		k = lo0bits(z)
		z >>= k
		stuffBits(dblBits, 0, z)
		k += 32
		i = 1
	}

	if de != 0 {
		e = de - bias - (p - 1) + k
		bits = p - k
	} else {
		e = de - bias - (p - 1) + 1 + k
		bits = 32*i - hi0bits(z)
	}
	return
}
