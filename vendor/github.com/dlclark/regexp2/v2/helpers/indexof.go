package helpers

import (
	"bytes"
	"slices"
	"strings"
	"unicode"
	"unsafe"

	"github.com/dlclark/regexp2/v2/syntax"
)

func IndexOfAny(in []rune, find []rune) int {
	// special case
	if len(find) == 0 {
		return -1
	}
	// naive version
	for i, c := range in {
		if slices.Contains(find, c) {
			return i
		}
	}
	return -1
}

func IndexOfAny1(in []rune, find rune) int {
	//TODO: bytes optimization?
	return slices.Index(in, find)
}

func IndexOfAny2(in []rune, find1, find2 rune) int {
	for i, c := range in {
		if c == find1 || c == find2 {
			return i
		}
	}

	return -1
}

func IndexOfAny3(in []rune, find1, find2, find3 rune) int {
	for i, c := range in {
		if c == find1 || c == find2 || c == find3 {
			return i
		}
	}

	return -1
}

func IndexOfAnyInRange(in []rune, first, last rune) int {
	for i, c := range in {
		if c >= first && c <= last {
			return i
		}
	}
	return -1
}

func IndexOfAnyExcept(in []rune, bad []rune) int {
	for i, c := range in {
		found := false
		for _, b := range bad {
			if b == c {
				found = true
				break
			}
		}
		if !found {
			return i
		}
	}
	return -1
}

func IndexOfAnyExcept1(in []rune, bad rune) int {
	for i, c := range in {
		if c != bad {
			return i
		}
	}
	return -1
}

func IndexOfAnyExcept2(in []rune, bad1, bad2 rune) int {
	for i, c := range in {
		if c != bad1 && c != bad2 {
			return i
		}
	}

	return -1
}

func IndexOfAnyExcept3(in []rune, bad1, bad2, bad3 rune) int {
	for i, c := range in {
		if c != bad1 && c != bad2 && c != bad3 {
			return i
		}
	}

	return -1
}

func IndexOfAnyExceptInRange(in []rune, first, last rune) int {
	for i, c := range in {
		if c > last {
			return i
		}
		if c < first {
			return i
		}
	}
	return -1
}

func IndexFunc(in []rune, f func(ch rune) bool) int {
	for i := range in {
		if f(in[i]) {
			return i
		}
	}
	return -1
}

func IndexOfAnyExceptInSet(in []rune, set syntax.CharSet) int {
	//TODO: this
	panic("not implemented")
}

func LastIndexOf(in []rune, find []rune) int {
	end := len(in) - len(find)
	first := find[0]
	lastOffset := len(find) - 1
	last := find[lastOffset]
	for i := end; i >= 0; i-- {
		//TODO: check 2 chars needed?
		// match start and end...check the middle
		if in[i] == first && in[i+lastOffset] == last {
			// found our first char
			// check if the rest are equal
			if bytesEqual(in[i:i+len(find)], find) {
				return i
			}
		}
	}

	//not found
	return -1
}

func LastIndexOfAnyExcept1(in []rune, not rune) int {
	for i := len(in) - 1; i >= 0; i-- {
		if in[i] != not {
			return i
		}
	}
	return -1
}

func LastIndexOfAny1(in []rune, find rune) int {
	for i := len(in) - 1; i >= 0; i-- {
		if in[i] == find {
			// found our char
			return i
		}
	}

	//not found
	return -1
}

func LastIndexOfAnyInRange(in []rune, first, last rune) int {
	for i := len(in) - 1; i >= 0; i-- {
		if in[i] >= first && in[i] <= last {
			return i
		}
	}
	return -1
}

//TODO: LastIndexOf methods
//IndexOfAnyInRange
//LastIndexOfAnyInRange
//LastIndexOfAnyExceptInRange

// find should always be sent in lower-case
func IndexOfIgnoreCase(in []rune, find []rune) int {
	// search the in slice for the "find" slice, ignoring case in the comparisons
	end := len(in) - len(find)
	first := find[0]
	for i := 0; i <= end; i++ {
		if in[i] != first && unicode.ToLower(in[i]) != first {
			continue
		}
		match := true
		for j := 1; j < len(find); j++ {
			inChar := in[i+j]
			if inChar != find[j] && unicode.ToLower(inChar) != find[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func IndexOfIgnoreCaseAscii(in []rune, find []rune) int {
	// search the in slice for the "find" slice, ignoring case in the comparisons
	// we can assume the find chars are ascii and do simple masks on them
	if len(find) == 0 {
		return 0
	}
	end := len(in) - len(find)
	first := foldASCII(rune(find[0]))
	for i := 0; i <= end; i++ {
		if foldASCII(in[i]) != first {
			continue
		}
		match := true
		for j := 1; j < len(find); j++ {
			if foldASCII(in[i+j]) != foldASCII(find[j]) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func IndexStringIgnoreCaseASCII(s, prefix string) int {
	if len(prefix) == 0 {
		return 0
	}

	for start, end := 0, len(s)-len(prefix); start <= end; {
		offset := indexASCIIByteIgnoreCase(s[start:], prefix[0])
		if offset < 0 || start+offset > end {
			return -1
		}

		i := start + offset
		if EqualStringIgnoreCaseASCII(s[i:i+len(prefix)], prefix) {
			return i
		}
		start = i + 1
	}
	return -1
}

func EqualStringIgnoreCaseASCII(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if foldASCII(rune(s[i])) != foldASCII(rune(prefix[i])) {
			return false
		}
	}
	return true
}

func indexASCIIByteIgnoreCase(s string, ch byte) int {
	ch = byte(foldASCII(rune(ch)))
	lower := strings.IndexByte(s, ch)
	if ch < 'a' || ch > 'z' {
		return lower
	}
	upper := strings.IndexByte(s, ch-('a'-'A'))
	if lower < 0 {
		return upper
	}
	if upper >= 0 && upper < lower {
		return upper
	}
	return lower
}

func foldASCII(c rune) rune {
	if 'A' <= c && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

func IndexOf(in []rune, find []rune) int {
	/*
		Since we auto-gen the find code this shouldn't happen
		if len(find) == 0 {
			//special case
			return -1
		}*/
	end := len(in) - len(find)
	first := find[0]
	//TODO: benchmark checking last char too or first two chars
	for i := 0; i <= end; i++ {
		// match start...check the rest
		if in[i] == first {
			// found our first char
			// check if the rest are equal
			if bytesEqual(in[i:i+len(find)], find) {
				return i
			}
			/*if slices.Equal(in[i:i+len(find)], find) {
				return i
			}*/
		}
	}

	//not found
	return -1
}

func StartsWith(in []rune, find []rune) bool {
	// if text is less than our "begin" then can't find it
	if len(in) < len(find) {
		return false
	}

	return bytesEqual(in[:len(find)], find)

	/*for i := 0; i < len(find); i++ {
		if in[i] != find[i] {
			return false
		}
	}

	return true*/
}

//StartsWithIgnoreCaseAscii would be faster

// find should always be sent in lower-case
func StartsWithIgnoreCase(in []rune, find []rune) bool {
	// if text is less than our "begin" then can't find it
	if len(in) < len(find) {
		return false
	}

	for i := 0; i < len(find); i++ {
		if in[i] == find[i] {
			// if we match the char exactly then we're good
			continue
		}
		// if the to-lower still doesn't match then it's not a match
		if unicode.ToLower(in[i]) != find[i] {
			return false
		}
	}

	return true
}

// internal function, assumes the bounds are already set right on the slices for equality
// casts the rune slices to bytes to use framework fast []byte comparison
func bytesEqual(a, b []rune) bool {
	bytesA := unsafe.Slice((*byte)(unsafe.Pointer(&a[0])), len(a)*4)
	bytesB := unsafe.Slice((*byte)(unsafe.Pointer(&b[0])), len(b)*4)
	return bytes.Equal(bytesA, bytesB)
}

func Equals(in []rune, start int, length int, find []rune) bool {
	if len(find) == 0 {
		return true
	}
	return bytesEqual(in[start:start+length], find)
}

func EqualsIgnoreCase(in []rune, start int, length int, find []rune) bool {
	//fast path if case matches
	if Equals(in, start, length, find) {
		return true
	}

	// search the in slice for the "find" slice, ignoring case in the comparisons
	// we can't assume casing or ascii-ness for either letter, have to toLower them both
	for j := 0; j < len(find); j++ {
		inChar := in[start+j]
		findChar := find[j]
		if inChar != findChar && unicode.ToLower(inChar) != unicode.ToLower(findChar) {
			return false
		}
	}

	// we've checked all chars and found matches every time
	return true
}
