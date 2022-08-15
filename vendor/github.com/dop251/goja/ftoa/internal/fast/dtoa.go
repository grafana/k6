package fast

import (
	"fmt"
	"strconv"
)

const (
	kMinimalTargetExponent = -60
	kMaximalTargetExponent = -32

	kTen4 = 10000
	kTen5 = 100000
	kTen6 = 1000000
	kTen7 = 10000000
	kTen8 = 100000000
	kTen9 = 1000000000
)

type Mode int

const (
	ModeShortest Mode = iota
	ModePrecision
)

// Adjusts the last digit of the generated number, and screens out generated
// solutions that may be inaccurate. A solution may be inaccurate if it is
// outside the safe interval, or if we cannot prove that it is closer to the
// input than a neighboring representation of the same length.
//
// Input: * buffer containing the digits of too_high / 10^kappa
//   - distance_too_high_w == (too_high - w).f() * unit
//   - unsafe_interval == (too_high - too_low).f() * unit
//   - rest = (too_high - buffer * 10^kappa).f() * unit
//   - ten_kappa = 10^kappa * unit
//   - unit = the common multiplier
//
// Output: returns true if the buffer is guaranteed to contain the closest
//
//	  representable number to the input.
//	Modifies the generated digits in the buffer to approach (round towards) w.
func roundWeed(buffer []byte, distance_too_high_w, unsafe_interval, rest, ten_kappa, unit uint64) bool {
	small_distance := distance_too_high_w - unit
	big_distance := distance_too_high_w + unit

	// Let w_low  = too_high - big_distance, and
	//     w_high = too_high - small_distance.
	// Note: w_low < w < w_high
	//
	// The real w (* unit) must lie somewhere inside the interval
	// ]w_low; w_high[ (often written as "(w_low; w_high)")

	// Basically the buffer currently contains a number in the unsafe interval
	// ]too_low; too_high[ with too_low < w < too_high
	//
	//  too_high - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	//                     ^v 1 unit            ^      ^                 ^      ^
	//  boundary_high ---------------------     .      .                 .      .
	//                     ^v 1 unit            .      .                 .      .
	//   - - - - - - - - - - - - - - - - - - -  +  - - + - - - - - -     .      .
	//                                          .      .         ^       .      .
	//                                          .  big_distance  .       .      .
	//                                          .      .         .       .    rest
	//                              small_distance     .         .       .      .
	//                                          v      .         .       .      .
	//  w_high - - - - - - - - - - - - - - - - - -     .         .       .      .
	//                     ^v 1 unit                   .         .       .      .
	//  w ----------------------------------------     .         .       .      .
	//                     ^v 1 unit                   v         .       .      .
	//  w_low  - - - - - - - - - - - - - - - - - - - - -         .       .      .
	//                                                           .       .      v
	//  buffer --------------------------------------------------+-------+--------
	//                                                           .       .
	//                                                  safe_interval    .
	//                                                           v       .
	//   - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -     .
	//                     ^v 1 unit                                     .
	//  boundary_low -------------------------                     unsafe_interval
	//                     ^v 1 unit                                     v
	//  too_low  - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - - -
	//
	//
	// Note that the value of buffer could lie anywhere inside the range too_low
	// to too_high.
	//
	// boundary_low, boundary_high and w are approximations of the real boundaries
	// and v (the input number). They are guaranteed to be precise up to one unit.
	// In fact the error is guaranteed to be strictly less than one unit.
	//
	// Anything that lies outside the unsafe interval is guaranteed not to round
	// to v when read again.
	// Anything that lies inside the safe interval is guaranteed to round to v
	// when read again.
	// If the number inside the buffer lies inside the unsafe interval but not
	// inside the safe interval then we simply do not know and bail out (returning
	// false).
	//
	// Similarly we have to take into account the imprecision of 'w' when finding
	// the closest representation of 'w'. If we have two potential
	// representations, and one is closer to both w_low and w_high, then we know
	// it is closer to the actual value v.
	//
	// By generating the digits of too_high we got the largest (closest to
	// too_high) buffer that is still in the unsafe interval. In the case where
	// w_high < buffer < too_high we try to decrement the buffer.
	// This way the buffer approaches (rounds towards) w.
	// There are 3 conditions that stop the decrementation process:
	//   1) the buffer is already below w_high
	//   2) decrementing the buffer would make it leave the unsafe interval
	//   3) decrementing the buffer would yield a number below w_high and farther
	//      away than the current number. In other words:
	//              (buffer{-1} < w_high) && w_high - buffer{-1} > buffer - w_high
	// Instead of using the buffer directly we use its distance to too_high.
	// Conceptually rest ~= too_high - buffer
	// We need to do the following tests in this order to avoid over- and
	// underflows.
	_DCHECK(rest <= unsafe_interval)
	for rest < small_distance && // Negated condition 1
		unsafe_interval-rest >= ten_kappa && // Negated condition 2
		(rest+ten_kappa < small_distance || // buffer{-1} > w_high
			small_distance-rest >= rest+ten_kappa-small_distance) {
		buffer[len(buffer)-1]--
		rest += ten_kappa
	}

	// We have approached w+ as much as possible. We now test if approaching w-
	// would require changing the buffer. If yes, then we have two possible
	// representations close to w, but we cannot decide which one is closer.
	if rest < big_distance && unsafe_interval-rest >= ten_kappa &&
		(rest+ten_kappa < big_distance ||
			big_distance-rest > rest+ten_kappa-big_distance) {
		return false
	}

	// Weeding test.
	//   The safe interval is [too_low + 2 ulp; too_high - 2 ulp]
	//   Since too_low = too_high - unsafe_interval this is equivalent to
	//      [too_high - unsafe_interval + 4 ulp; too_high - 2 ulp]
	//   Conceptually we have: rest ~= too_high - buffer
	return (2*unit <= rest) && (rest <= unsafe_interval-4*unit)
}

// Rounds the buffer upwards if the result is closer to v by possibly adding
// 1 to the buffer. If the precision of the calculation is not sufficient to
// round correctly, return false.
// The rounding might shift the whole buffer in which case the kappa is
// adjusted. For example "99", kappa = 3 might become "10", kappa = 4.
//
// If 2*rest > ten_kappa then the buffer needs to be round up.
// rest can have an error of +/- 1 unit. This function accounts for the
// imprecision and returns false, if the rounding direction cannot be
// unambiguously determined.
//
// Precondition: rest < ten_kappa.
func roundWeedCounted(buffer []byte, rest, ten_kappa, unit uint64, kappa *int) bool {
	_DCHECK(rest < ten_kappa)
	// The following tests are done in a specific order to avoid overflows. They
	// will work correctly with any uint64 values of rest < ten_kappa and unit.
	//
	// If the unit is too big, then we don't know which way to round. For example
	// a unit of 50 means that the real number lies within rest +/- 50. If
	// 10^kappa == 40 then there is no way to tell which way to round.
	if unit >= ten_kappa {
		return false
	}
	// Even if unit is just half the size of 10^kappa we are already completely
	// lost. (And after the previous test we know that the expression will not
	// over/underflow.)
	if ten_kappa-unit <= unit {
		return false
	}
	// If 2 * (rest + unit) <= 10^kappa we can safely round down.
	if (ten_kappa-rest > rest) && (ten_kappa-2*rest >= 2*unit) {
		return true
	}

	// If 2 * (rest - unit) >= 10^kappa, then we can safely round up.
	if (rest > unit) && (ten_kappa-(rest-unit) <= (rest - unit)) {
		// Increment the last digit recursively until we find a non '9' digit.
		buffer[len(buffer)-1]++
		for i := len(buffer) - 1; i > 0; i-- {
			if buffer[i] != '0'+10 {
				break
			}
			buffer[i] = '0'
			buffer[i-1]++
		}
		// If the first digit is now '0'+ 10 we had a buffer with all '9's. With the
		// exception of the first digit all digits are now '0'. Simply switch the
		// first digit to '1' and adjust the kappa. Example: "99" becomes "10" and
		// the power (the kappa) is increased.
		if buffer[0] == '0'+10 {
			buffer[0] = '1'
			*kappa += 1
		}
		return true
	}
	return false
}

// Returns the biggest power of ten that is less than or equal than the given
// number. We furthermore receive the maximum number of bits 'number' has.
// If number_bits == 0 then 0^-1 is returned
// The number of bits must be <= 32.
// Precondition: number < (1 << (number_bits + 1)).
func biggestPowerTen(number uint32, number_bits int) (power uint32, exponent int) {
	switch number_bits {
	case 32, 31, 30:
		if kTen9 <= number {
			power = kTen9
			exponent = 9
			break
		}
		fallthrough
	case 29, 28, 27:
		if kTen8 <= number {
			power = kTen8
			exponent = 8
			break
		}
		fallthrough
	case 26, 25, 24:
		if kTen7 <= number {
			power = kTen7
			exponent = 7
			break
		}
		fallthrough
	case 23, 22, 21, 20:
		if kTen6 <= number {
			power = kTen6
			exponent = 6
			break
		}
		fallthrough
	case 19, 18, 17:
		if kTen5 <= number {
			power = kTen5
			exponent = 5
			break
		}
		fallthrough
	case 16, 15, 14:
		if kTen4 <= number {
			power = kTen4
			exponent = 4
			break
		}
		fallthrough
	case 13, 12, 11, 10:
		if 1000 <= number {
			power = 1000
			exponent = 3
			break
		}
		fallthrough
	case 9, 8, 7:
		if 100 <= number {
			power = 100
			exponent = 2
			break
		}
		fallthrough
	case 6, 5, 4:
		if 10 <= number {
			power = 10
			exponent = 1
			break
		}
		fallthrough
	case 3, 2, 1:
		if 1 <= number {
			power = 1
			exponent = 0
			break
		}
		fallthrough
	case 0:
		power = 0
		exponent = -1
	}
	return
}

// Generates the digits of input number w.
// w is a floating-point number (DiyFp), consisting of a significand and an
// exponent. Its exponent is bounded by kMinimalTargetExponent and
// kMaximalTargetExponent.
//
//	Hence -60 <= w.e() <= -32.
//
// Returns false if it fails, in which case the generated digits in the buffer
// should not be used.
// Preconditions:
//   - low, w and high are correct up to 1 ulp (unit in the last place). That
//     is, their error must be less than a unit of their last digits.
//   - low.e() == w.e() == high.e()
//   - low < w < high, and taking into account their error: low~ <= high~
//   - kMinimalTargetExponent <= w.e() <= kMaximalTargetExponent
//
// Postconditions: returns false if procedure fails.
//
//	otherwise:
//	  * buffer is not null-terminated, but len contains the number of digits.
//	  * buffer contains the shortest possible decimal digit-sequence
//	    such that LOW < buffer * 10^kappa < HIGH, where LOW and HIGH are the
//	    correct values of low and high (without their error).
//	  * if more than one decimal representation gives the minimal number of
//	    decimal digits then the one closest to W (where W is the correct value
//	    of w) is chosen.
//
// Remark: this procedure takes into account the imprecision of its input
//
//	numbers. If the precision is not enough to guarantee all the postconditions
//	then false is returned. This usually happens rarely (~0.5%).
//
// Say, for the sake of example, that
//
//	w.e() == -48, and w.f() == 0x1234567890ABCDEF
//
// w's value can be computed by w.f() * 2^w.e()
// We can obtain w's integral digits by simply shifting w.f() by -w.e().
//
//	-> w's integral part is 0x1234
//	w's fractional part is therefore 0x567890ABCDEF.
//
// Printing w's integral part is easy (simply print 0x1234 in decimal).
// In order to print its fraction we repeatedly multiply the fraction by 10 and
// get each digit. Example the first digit after the point would be computed by
//
//	(0x567890ABCDEF * 10) >> 48. -> 3
//
// The whole thing becomes slightly more complicated because we want to stop
// once we have enough digits. That is, once the digits inside the buffer
// represent 'w' we can stop. Everything inside the interval low - high
// represents w. However we have to pay attention to low, high and w's
// imprecision.
func digitGen(low, w, high diyfp, buffer []byte) (kappa int, buf []byte, res bool) {
	_DCHECK(low.e == w.e && w.e == high.e)
	_DCHECK(low.f+1 <= high.f-1)
	_DCHECK(kMinimalTargetExponent <= w.e && w.e <= kMaximalTargetExponent)
	// low, w and high are imprecise, but by less than one ulp (unit in the last
	// place).
	// If we remove (resp. add) 1 ulp from low (resp. high) we are certain that
	// the new numbers are outside of the interval we want the final
	// representation to lie in.
	// Inversely adding (resp. removing) 1 ulp from low (resp. high) would yield
	// numbers that are certain to lie in the interval. We will use this fact
	// later on.
	// We will now start by generating the digits within the uncertain
	// interval. Later we will weed out representations that lie outside the safe
	// interval and thus _might_ lie outside the correct interval.
	unit := uint64(1)
	too_low := diyfp{f: low.f - unit, e: low.e}
	too_high := diyfp{f: high.f + unit, e: high.e}
	// too_low and too_high are guaranteed to lie outside the interval we want the
	// generated number in.
	unsafe_interval := too_high.minus(too_low)
	// We now cut the input number into two parts: the integral digits and the
	// fractionals. We will not write any decimal separator though, but adapt
	// kappa instead.
	// Reminder: we are currently computing the digits (stored inside the buffer)
	// such that:   too_low < buffer * 10^kappa < too_high
	// We use too_high for the digit_generation and stop as soon as possible.
	// If we stop early we effectively round down.
	one := diyfp{f: 1 << -w.e, e: w.e}
	// Division by one is a shift.
	integrals := uint32(too_high.f >> -one.e)
	// Modulo by one is an and.
	fractionals := too_high.f & (one.f - 1)
	divisor, divisor_exponent := biggestPowerTen(integrals, diyFpKSignificandSize-(-one.e))
	kappa = divisor_exponent + 1
	buf = buffer
	for kappa > 0 {
		digit := int(integrals / divisor)
		buf = append(buf, byte('0'+digit))
		integrals %= divisor
		kappa--
		// Note that kappa now equals the exponent of the divisor and that the
		// invariant thus holds again.
		rest := uint64(integrals)<<-one.e + fractionals
		// Invariant: too_high = buffer * 10^kappa + DiyFp(rest, one.e)
		// Reminder: unsafe_interval.e == one.e
		if rest < unsafe_interval.f {
			// Rounding down (by not emitting the remaining digits) yields a number
			// that lies within the unsafe interval.
			res = roundWeed(buf, too_high.minus(w).f,
				unsafe_interval.f, rest,
				uint64(divisor)<<-one.e, unit)
			return
		}
		divisor /= 10
	}
	// The integrals have been generated. We are at the point of the decimal
	// separator. In the following loop we simply multiply the remaining digits by
	// 10 and divide by one. We just need to pay attention to multiply associated
	// data (like the interval or 'unit'), too.
	// Note that the multiplication by 10 does not overflow, because w.e >= -60
	// and thus one.e >= -60.
	_DCHECK(one.e >= -60)
	_DCHECK(fractionals < one.f)
	_DCHECK(0xFFFFFFFFFFFFFFFF/10 >= one.f)
	for {
		fractionals *= 10
		unit *= 10
		unsafe_interval.f *= 10
		// Integer division by one.
		digit := byte(fractionals >> -one.e)
		buf = append(buf, '0'+digit)
		fractionals &= one.f - 1 // Modulo by one.
		kappa--
		if fractionals < unsafe_interval.f {
			res = roundWeed(buf, too_high.minus(w).f*unit, unsafe_interval.f, fractionals, one.f, unit)
			return
		}
	}
}

// Generates (at most) requested_digits of input number w.
// w is a floating-point number (DiyFp), consisting of a significand and an
// exponent. Its exponent is bounded by kMinimalTargetExponent and
// kMaximalTargetExponent.
//
//	Hence -60 <= w.e() <= -32.
//
// Returns false if it fails, in which case the generated digits in the buffer
// should not be used.
// Preconditions:
//   - w is correct up to 1 ulp (unit in the last place). That
//     is, its error must be strictly less than a unit of its last digit.
//   - kMinimalTargetExponent <= w.e() <= kMaximalTargetExponent
//
// Postconditions: returns false if procedure fails.
//
//	otherwise:
//	  * buffer is not null-terminated, but length contains the number of
//	    digits.
//	  * the representation in buffer is the most precise representation of
//	    requested_digits digits.
//	  * buffer contains at most requested_digits digits of w. If there are less
//	    than requested_digits digits then some trailing '0's have been removed.
//	  * kappa is such that
//	         w = buffer * 10^kappa + eps with |eps| < 10^kappa / 2.
//
// Remark: This procedure takes into account the imprecision of its input
//
//	numbers. If the precision is not enough to guarantee all the postconditions
//	then false is returned. This usually happens rarely, but the failure-rate
//	increases with higher requested_digits.
func digitGenCounted(w diyfp, requested_digits int, buffer []byte) (kappa int, buf []byte, res bool) {
	_DCHECK(kMinimalTargetExponent <= w.e && w.e <= kMaximalTargetExponent)

	// w is assumed to have an error less than 1 unit. Whenever w is scaled we
	// also scale its error.
	w_error := uint64(1)
	// We cut the input number into two parts: the integral digits and the
	// fractional digits. We don't emit any decimal separator, but adapt kappa
	// instead. Example: instead of writing "1.2" we put "12" into the buffer and
	// increase kappa by 1.
	one := diyfp{f: 1 << -w.e, e: w.e}
	// Division by one is a shift.
	integrals := uint32(w.f >> -one.e)
	// Modulo by one is an and.
	fractionals := w.f & (one.f - 1)
	divisor, divisor_exponent := biggestPowerTen(integrals, diyFpKSignificandSize-(-one.e))
	kappa = divisor_exponent + 1
	buf = buffer
	// Loop invariant: buffer = w / 10^kappa  (integer division)
	// The invariant holds for the first iteration: kappa has been initialized
	// with the divisor exponent + 1. And the divisor is the biggest power of ten
	// that is smaller than 'integrals'.
	for kappa > 0 {
		digit := byte(integrals / divisor)
		buf = append(buf, '0'+digit)
		requested_digits--
		integrals %= divisor
		kappa--
		// Note that kappa now equals the exponent of the divisor and that the
		// invariant thus holds again.
		if requested_digits == 0 {
			break
		}
		divisor /= 10
	}

	if requested_digits == 0 {
		rest := uint64(integrals)<<-one.e + fractionals
		res = roundWeedCounted(buf, rest, uint64(divisor)<<-one.e, w_error, &kappa)
		return
	}

	// The integrals have been generated. We are at the point of the decimal
	// separator. In the following loop we simply multiply the remaining digits by
	// 10 and divide by one. We just need to pay attention to multiply associated
	// data (the 'unit'), too.
	// Note that the multiplication by 10 does not overflow, because w.e >= -60
	// and thus one.e >= -60.
	_DCHECK(one.e >= -60)
	_DCHECK(fractionals < one.f)
	_DCHECK(0xFFFFFFFFFFFFFFFF/10 >= one.f)
	for requested_digits > 0 && fractionals > w_error {
		fractionals *= 10
		w_error *= 10
		// Integer division by one.
		digit := byte(fractionals >> -one.e)
		buf = append(buf, '0'+digit)
		requested_digits--
		fractionals &= one.f - 1 // Modulo by one.
		kappa--
	}
	if requested_digits != 0 {
		res = false
	} else {
		res = roundWeedCounted(buf, fractionals, one.f, w_error, &kappa)
	}
	return
}

// Provides a decimal representation of v.
// Returns true if it succeeds, otherwise the result cannot be trusted.
// There will be *length digits inside the buffer (not null-terminated).
// If the function returns true then
//
//	v == (double) (buffer * 10^decimal_exponent).
//
// The digits in the buffer are the shortest representation possible: no
// 0.09999999999999999 instead of 0.1. The shorter representation will even be
// chosen even if the longer one would be closer to v.
// The last digit will be closest to the actual v. That is, even if several
// digits might correctly yield 'v' when read again, the closest will be
// computed.
func grisu3(f float64, buffer []byte) (digits []byte, decimal_exponent int, result bool) {
	v := double(f)
	w := v.toNormalizedDiyfp()

	// boundary_minus and boundary_plus are the boundaries between v and its
	// closest floating-point neighbors. Any number strictly between
	// boundary_minus and boundary_plus will round to v when convert to a double.
	// Grisu3 will never output representations that lie exactly on a boundary.
	boundary_minus, boundary_plus := v.normalizedBoundaries()
	ten_mk_minimal_binary_exponent := kMinimalTargetExponent - (w.e + diyFpKSignificandSize)
	ten_mk_maximal_binary_exponent := kMaximalTargetExponent - (w.e + diyFpKSignificandSize)
	ten_mk, mk := getCachedPowerForBinaryExponentRange(ten_mk_minimal_binary_exponent, ten_mk_maximal_binary_exponent)

	_DCHECK(
		(kMinimalTargetExponent <=
			w.e+ten_mk.e+diyFpKSignificandSize) &&
			(kMaximalTargetExponent >= w.e+ten_mk.e+diyFpKSignificandSize))
	// Note that ten_mk is only an approximation of 10^-k. A DiyFp only contains a
	// 64 bit significand and ten_mk is thus only precise up to 64 bits.

	// The DiyFp::Times procedure rounds its result, and ten_mk is approximated
	// too. The variable scaled_w (as well as scaled_boundary_minus/plus) are now
	// off by a small amount.
	// In fact: scaled_w - w*10^k < 1ulp (unit in the last place) of scaled_w.
	// In other words: let f = scaled_w.f() and e = scaled_w.e(), then
	//           (f-1) * 2^e < w*10^k < (f+1) * 2^e
	scaled_w := w.times(ten_mk)
	_DCHECK(scaled_w.e ==
		boundary_plus.e+ten_mk.e+diyFpKSignificandSize)
	// In theory it would be possible to avoid some recomputations by computing
	// the difference between w and boundary_minus/plus (a power of 2) and to
	// compute scaled_boundary_minus/plus by subtracting/adding from
	// scaled_w. However the code becomes much less readable and the speed
	// enhancements are not terrific.
	scaled_boundary_minus := boundary_minus.times(ten_mk)
	scaled_boundary_plus := boundary_plus.times(ten_mk)
	// DigitGen will generate the digits of scaled_w. Therefore we have
	// v == (double) (scaled_w * 10^-mk).
	// Set decimal_exponent == -mk and pass it to DigitGen. If scaled_w is not an
	// integer than it will be updated. For instance if scaled_w == 1.23 then
	// the buffer will be filled with "123" und the decimal_exponent will be
	// decreased by 2.
	var kappa int
	kappa, digits, result = digitGen(scaled_boundary_minus, scaled_w, scaled_boundary_plus, buffer)
	decimal_exponent = -mk + kappa
	return
}

// The "counted" version of grisu3 (see above) only generates requested_digits
// number of digits. This version does not generate the shortest representation,
// and with enough requested digits 0.1 will at some point print as 0.9999999...
// Grisu3 is too imprecise for real halfway cases (1.5 will not work) and
// therefore the rounding strategy for halfway cases is irrelevant.
func grisu3Counted(v float64, requested_digits int, buffer []byte) (digits []byte, decimal_exponent int, result bool) {
	w := double(v).toNormalizedDiyfp()
	ten_mk_minimal_binary_exponent := kMinimalTargetExponent - (w.e + diyFpKSignificandSize)
	ten_mk_maximal_binary_exponent := kMaximalTargetExponent - (w.e + diyFpKSignificandSize)
	ten_mk, mk := getCachedPowerForBinaryExponentRange(ten_mk_minimal_binary_exponent, ten_mk_maximal_binary_exponent)

	_DCHECK(
		(kMinimalTargetExponent <=
			w.e+ten_mk.e+diyFpKSignificandSize) &&
			(kMaximalTargetExponent >= w.e+ten_mk.e+diyFpKSignificandSize))
	// Note that ten_mk is only an approximation of 10^-k. A DiyFp only contains a
	// 64 bit significand and ten_mk is thus only precise up to 64 bits.

	// The DiyFp::Times procedure rounds its result, and ten_mk is approximated
	// too. The variable scaled_w (as well as scaled_boundary_minus/plus) are now
	// off by a small amount.
	// In fact: scaled_w - w*10^k < 1ulp (unit in the last place) of scaled_w.
	// In other words: let f = scaled_w.f() and e = scaled_w.e(), then
	//           (f-1) * 2^e < w*10^k < (f+1) * 2^e
	scaled_w := w.times(ten_mk)
	// We now have (double) (scaled_w * 10^-mk).
	// DigitGen will generate the first requested_digits digits of scaled_w and
	// return together with a kappa such that scaled_w ~= buffer * 10^kappa. (It
	// will not always be exactly the same since DigitGenCounted only produces a
	// limited number of digits.)
	var kappa int
	kappa, digits, result = digitGenCounted(scaled_w, requested_digits, buffer)
	decimal_exponent = -mk + kappa

	return
}

// v must be > 0 and must not be Inf or NaN
func Dtoa(v float64, mode Mode, requested_digits int, buffer []byte) (digits []byte, decimal_point int, result bool) {
	defer func() {
		if x := recover(); x != nil {
			if x == dcheckFailure {
				panic(fmt.Errorf("DCHECK assertion failed while formatting %s in mode %d", strconv.FormatFloat(v, 'e', 50, 64), mode))
			}
			panic(x)
		}
	}()
	var decimal_exponent int
	startPos := len(buffer)
	switch mode {
	case ModeShortest:
		digits, decimal_exponent, result = grisu3(v, buffer)
	case ModePrecision:
		digits, decimal_exponent, result = grisu3Counted(v, requested_digits, buffer)
	}
	if result {
		decimal_point = len(digits) - startPos + decimal_exponent
	} else {
		digits = digits[:startPos]
	}
	return
}
