package regexp2

import (
	"unicode/utf8"

	"github.com/dlclark/regexp2/v2/helpers"
)

// decodedInput is the engine's rune view of a string. When the pattern cannot
// look behind a candidate start, the slice may begin at that candidate instead
// of byte 0.
type decodedInput struct {
	runes      []rune
	pooled     *[]rune
	runeStart  int  // index in runes of the requested startAt; -1 if not a rune boundary
	runeOffset int  // original-string rune index of runes[0]
	byteOffset int  // original-string byte index of runes[0]
	ascii      bool // every decoded rune is ASCII; invalid UTF-8 is not ASCII
}

// decodeFrom is the first original-string byte that must be decoded. 0 means
// the whole string; a later index is only used when slicing is safe.
func (re *Regexp) decodeFrom(s string, startAt int) int {
	if startAt <= 0 || re.RightToLeft() {
		return 0
	}
	need := re.decodeLeftContextRunes()
	if need < 0 {
		return 0
	}
	if need > 0 {
		return prevRuneByte(s, startAt)
	}
	return startAt
}

func (re *Regexp) decodeLeftContextRunes() int {
	if re.code != nil {
		return re.code.LeftContextRunes
	}
	return re.leftContextRunes
}

// stringSearchOrigin preserves the caller's origin when \G may observe it.
// Such patterns retain the whole input, so no decode offset is needed.
// Otherwise the origin is unobservable and the candidate is sufficient.
func (re *Regexp) stringSearchOrigin(input string, startAt, candidate int) int {
	if !re.RightToLeft() && re.decodeLeftContextRunes() < 0 {
		if startAt <= 0 {
			return 0
		}
		return utf8.RuneCountInString(input[:startAt])
	}
	return candidate
}

// decodeString converts s to []rune for the MatchString startAt<=0 path.
// Keep this as close as possible to a single UTF-8 walk so 4–30 byte
// matches stay cheap.
func decodeString(s string, maxCachedLength int) ([]rune, *[]rune) {
	buf, pooled := pooledRuneBuffers.get(len(s), maxCachedLength)
	if len(s) >= helpers.ASCIIScanMin {
		n, _ := helpers.DecodeString(s, buf)
		return buf[:n], pooled
	}
	n := 0
	for _, ch := range s {
		buf[n] = ch
		n++
	}
	return buf[:n], pooled
}

func (re *Regexp) decodeStringInput(s string, startAt int, pooled bool) decodedInput {
	maxCachedLength := 0
	if pooled {
		maxCachedLength = re.optimizations.MaxCachedRuneBufferLength
	}
	return decodeInput(s, startAt, re.decodeFrom(s, startAt), maxCachedLength, true)
}

// decodeInput converts s[decodeFrom:] to runes. startAt is a byte index in s.
// If startAt < 0 or is not a rune boundary, runeStart is -1. needOffsets walks
// the skipped prefix to fill runeOffset; MatchString does not need that field.
func decodeInput(s string, startAt, decodeFrom, maxCachedLength int, needOffsets bool) decodedInput {
	if decodeFrom < 0 {
		decodeFrom = 0
	}
	if decodeFrom > len(s) {
		decodeFrom = len(s)
	}

	substr := s[decodeFrom:]
	buf, pooledBuffer := pooledRuneBuffers.get(len(substr), maxCachedLength)
	n, ascii := helpers.DecodeString(substr, buf)
	runes := buf[:n]

	out := decodedInput{
		runes:      runes,
		pooled:     pooledBuffer,
		byteOffset: decodeFrom,
		ascii:      ascii,
		runeStart:  startRuneIndex(s, startAt, decodeFrom, n, ascii),
	}

	if !needOffsets {
		return out
	}

	// Byte index equals rune index only when the skipped prefix is also ASCII.
	if ascii && (decodeFrom == 0 || helpers.IsASCII(s[:decodeFrom])) {
		out.runeOffset = decodeFrom
		return out
	}
	if decodeFrom > 0 {
		out.runeOffset = utf8.RuneCountInString(s[:decodeFrom])
	}
	return out
}

func startRuneIndex(s string, startAt, decodeFrom, runeCount int, ascii bool) int {
	if startAt < 0 {
		return -1
	}
	if startAt < decodeFrom || startAt > len(s) {
		return -1
	}
	if startAt == decodeFrom {
		return 0
	}
	if startAt == len(s) {
		return runeCount
	}
	if ascii {
		return startAt - decodeFrom
	}
	rel := startAt - decodeFrom
	i := 0
	for strIdx := range s[decodeFrom:] {
		if strIdx == rel {
			return i
		}
		if strIdx > rel {
			return -1
		}
		i++
	}
	return -1
}

func prevRuneByte(s string, byteIndex int) int {
	if byteIndex <= 0 {
		return 0
	}
	_, size := utf8.DecodeLastRuneInString(s[:byteIndex])
	if size <= 0 {
		return byteIndex - 1
	}
	return byteIndex - size
}

func (d decodedInput) release() {
	if d.pooled != nil {
		*d.pooled = d.runes
		pooledRuneBuffers.put(d.pooled)
	}
}
