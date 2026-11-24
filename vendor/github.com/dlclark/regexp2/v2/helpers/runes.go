package helpers

import "unicode"

func IsBetween(val rune, first, last rune) bool {
	if val > last {
		return false
	}
	if val >= first {
		return true
	}
	return false
}

// According to UTS#18 Unicode Regular Expressions (http://www.unicode.org/reports/tr18/)
// RL 1.4 Simple Word Boundaries  The class of <word_character> includes all Alphabetic
// values from the Unicode character database, from UnicodeData.txt [UData], plus the U+200C
// ZERO WIDTH NON-JOINER and U+200D ZERO WIDTH JOINER.
func IsWordChar(r rune) bool {
	// matches charclass.go

	//TODO: add optimization here for ascii

	//"L", "Mn", "Nd", "Pc"
	return unicode.In(r,
		unicode.Categories["L"], unicode.Categories["Mn"],
		unicode.Categories["Nd"], unicode.Categories["Pc"]) || r == '\u200D' || r == '\u200C'
	//return 'A' <= r && r <= 'Z' || 'a' <= r && r <= 'z' || '0' <= r && r <= '9' || r == '_'
}

func IsInMask32(ch rune, mask uint32) bool {
	//BDFHJLNPRTVX = 10101010 10101010 10101010 00000000
	//B            = 00000000 00000000 00000000 01000010
	//char=B-B     = 00000000 00000000 00000000 00000000
	//BDFH.. << 0  = 10101010 10101010 10101010 00000000
	//char-32      = 11111111 11111111 11111111 11100000
	//&            = 10101010 10101010 10101010 00000000
	// first bit is 1 then negative, so it matches

	//charMinusLowUInt32 := int32(ch - low)
	return int32((mask<<uint16(ch))&uint32(ch-32)) < 0
}

func IsInMask64(ch rune, mask uint64) bool {
	//64-bit version of the above
	//charMinusLowUInt64 := int64(ch - low)
	return int64((mask<<uint32(ch))&uint64(ch-64)) < 0
}

func IsInASCIIBitmap(ch rune, lo uint64, hi uint64) bool {
	if ch < 64 {
		return lo&(1<<uint(ch)) != 0
	}
	if ch < 128 {
		return hi&(1<<uint(ch-64)) != 0
	}
	return false
}
