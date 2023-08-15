package goja

// inspired by https://gist.github.com/orlp/3551590

var overflows = [64]int64{
	9223372036854775807, 9223372036854775807, 3037000499, 2097151,
	55108, 6208, 1448, 511,
	234, 127, 78, 52,
	38, 28, 22, 18,
	15, 13, 11, 9,
	8, 7, 7, 6,
	6, 5, 5, 5,
	4, 4, 4, 4,
	3, 3, 3, 3,
	3, 3, 3, 3,
	2, 2, 2, 2,
	2, 2, 2, 2,
	2, 2, 2, 2,
	2, 2, 2, 2,
	2, 2, 2, 2,
	2, 2, 2, 2,
}

var highestBitSet = [63]byte{
	0, 1, 2, 2, 3, 3, 3, 3,
	4, 4, 4, 4, 4, 4, 4, 4,
	5, 5, 5, 5, 5, 5, 5, 5,
	5, 5, 5, 5, 5, 5, 5, 5,
	6, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6, 6,
	6, 6, 6, 6, 6, 6, 6,
}

func ipow(base, exp int64) (result int64) {
	if exp >= 63 {
		if base == 1 {
			return 1
		}

		if base == -1 {
			return 1 - 2*(exp&1)
		}

		return 0
	}

	if base > overflows[exp] || -base > overflows[exp] {
		return 0
	}

	result = 1

	switch highestBitSet[byte(exp)] {
	case 6:
		if exp&1 != 0 {
			result *= base
		}
		exp >>= 1
		base *= base
		fallthrough
	case 5:
		if exp&1 != 0 {
			result *= base
		}
		exp >>= 1
		base *= base
		fallthrough
	case 4:
		if exp&1 != 0 {
			result *= base
		}
		exp >>= 1
		base *= base
		fallthrough
	case 3:
		if exp&1 != 0 {
			result *= base
		}
		exp >>= 1
		base *= base
		fallthrough
	case 2:
		if exp&1 != 0 {
			result *= base
		}
		exp >>= 1
		base *= base
		fallthrough
	case 1:
		if exp&1 != 0 {
			result *= base
		}
		fallthrough
	default:
		return result
	}
}
