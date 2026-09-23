// Package name implements various naming conventions. The two categories are
// delimiter-separated and letter case-separated words. Each of the formatting
// functions support both techniques for input, without any context.
package name

import (
	"strings"
	"unicode"
)

// CamelCase returns the medial capitals form of the words in s.
// Words consist of Unicode letters and/or numbers in any order.
// Upper case sequences [abbreviations] are preserved.
//
// Argument upper forces the letter case for the first rune.
// Use true for UpperCamelCase, a.k.a. PascalCase.
// Use false for lowerCamelCase, a.k.a. dromedaryCase.
//
// BUG(pascaldekloe): Abbreviations at the beginning of a name
// may look odd in lowerCamelCase, i.e., "tCPConn".
//
// BUG(pascaldekloe): CamelCase concatenates abbreviations by
// design, i.e., "DB-API" becomes "DBAPI".
func CamelCase(s string, upper bool) string {
	var b strings.Builder
	b.Grow(len(s))

	// The conversion keeps any camel-casing as is.
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			if upper {
				r = unicode.ToUpper(r)
			} else if b.Len() == 0 {
				// force only on beginning of name
				r = unicode.ToLower(r)
			}

			fallthrough
		case unicode.IsNumber(r):
			b.WriteRune(r)
			upper = false // mark continuation

		default:
			// delimiter found
			upper = true // mark begin
		}
	}

	return b.String()
}

// SnakeCase returns Delimit(s, '_'), a.k.a. the snake_case.
func SnakeCase(s string) string {
	return Delimit(s, '_')
}

// DotSeparated returns Delimit(s, '.'), a.k.a. the dot notation.
func DotSeparated(s string) string {
	return Delimit(s, '.')
}

// Delimit returns the words in s delimited with separator sep.
// Words consist of Unicode letters and/or numbers in any order.
// Upper case sequences [abbreviations] are preserved.
// Use strings.ToLower or ToUpper to enforce one letter case.
func Delimit(s string, sep rune) string {
	var b strings.Builder
	b.Grow(len(s) + (len(s)+1)/4)

	var last rune   // previous rune is a pending write
	var wordLen int // number of runes in word up until last
	for _, r := range s {
		switch {
		case wordLen == 0:
			if unicode.IsLetter(r) || unicode.IsNumber(r) {
				if b.Len() == 0 { // special case
					last = unicode.ToUpper(r)
				} else { // delimit previous word
					b.WriteRune(sep)
					last = r
				}
				wordLen = 1
			}

			continue

		case unicode.IsUpper(r):
			if !unicode.IsUpper(last) {
				if b.Len() != 0 {
					b.WriteRune(last) // end of word
					last = sep        // enqueue separator instead
					wordLen = 0       // r is new begin
				}
			}

		case unicode.IsLetter(r): // lower-case
			if unicode.IsUpper(last) {
				if wordLen > 1 {
					// delimit previous word
					b.WriteRune(sep)
					wordLen = 1
				}
				last = unicode.ToLower(last)
			}

		case !unicode.IsNumber(r):
			// delimiter found
			if wordLen != 0 {
				// flush pending
				b.WriteRune(last)
				wordLen = 0
			}

			continue
		}

		b.WriteRune(last)
		last = r
		wordLen++
	}

	if wordLen != 0 {
		// flush pending
		b.WriteRune(last)
	}

	return b.String()
}
