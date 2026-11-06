package helpers

import (
	"fmt"
	"math"
	"slices"
	"unicode"
)

type AsciiSearchValues struct {
	// each ascii byte is represented by a bit in this array
	// there are 128bits here and ascii has 128 possible chars
	set [2]uint64
}

func NewAsciiSearchValues(vals string) AsciiSearchValues {
	// pre-calc ascii table stuff to make this go faster
	sv := AsciiSearchValues{}
	for i := 0; i < len(vals); i++ {
		c := vals[i]
		if c > unicode.MaxASCII {
			// a bug got us here. that's bad.
			panic(fmt.Errorf("non-ascii value found in ascii search values: %s", vals))
		}
		idx := c / 64
		shift := c % 64
		sv.set[idx] |= 1 << shift
	}

	return sv
}

// return the first index of our original vals values within the slice given
func (s AsciiSearchValues) IndexOfAny(chars []rune) int {
	for i := 0; i < len(chars); i++ {
		c := chars[i]
		if c > unicode.MaxASCII {
			continue
		}
		idx := c / 64
		shift := c % 64
		if s.set[idx]&(1<<shift) != 0 {
			return i
		}
	}
	return -1
}

// return the first index of anything except our original vals values within the slice given
func (s AsciiSearchValues) IndexOfAnyExcept(chars []rune) int {
	for i := 0; i < len(chars); i++ {
		c := chars[i]
		if c > unicode.MaxASCII {
			return i
		}
		idx := c / 64
		shift := c % 64
		if s.set[idx]&(1<<shift) == 0 {
			return i
		}
	}
	return -1
}

// return the last index of our original vals values within the slice given
func (s AsciiSearchValues) LastIndexOfAny(chars []rune) int {
	for i := len(chars) - 1; i >= 0; i-- {
		c := chars[i]
		if c > unicode.MaxASCII {
			continue
		}
		if s.set[c/64]&(1<<(c%64)) != 0 {
			return i
		}
	}
	return -1
}

// return the last index of our original vals values within the slice given
func (s AsciiSearchValues) LastIndexOfAnyExcept(chars []rune) int {
	for i := len(chars) - 1; i >= 0; i-- {
		c := chars[i]
		if c > unicode.MaxASCII {
			return i
		}
		if s.set[c/64]&(1<<(c%64)) == 0 {
			return i
		}
	}
	return -1
}

type RuneSearchValues struct {
	vals []rune
}

func newRuneSearchValues(vals []rune) RuneSearchValues {
	return RuneSearchValues{vals: vals}
}

func NewRuneSearchValues(vals string) RuneSearchValues {
	return newRuneSearchValues([]rune(vals))
}

func (s RuneSearchValues) IndexOfAny(chars []rune) int {
	return IndexOfAny(chars, s.vals)
}

func (s RuneSearchValues) IndexOfAnyExcept(chars []rune) int {
	return IndexOfAnyExcept(chars, s.vals)
}

func (s RuneSearchValues) LastIndexOfAny(chars []rune) int {
	if len(s.vals) == 1 {
		return LastIndexOfAny1(chars, s.vals[0])
	}
	for i := len(chars) - 1; i >= 0; i-- {
		if slices.Contains(s.vals, chars[i]) {
			return i
		}
	}
	return -1
}

func (s RuneSearchValues) LastIndexOfAnyExcept(chars []rune) int {
	for i := len(chars) - 1; i >= 0; i-- {
		if !slices.Contains(s.vals, chars[i]) {
			return i
		}
	}
	return -1
}

type StringSearchValues struct {
	vals        [][]rune
	ignoreCase  bool
	shortestVal int

	firstChars RuneSearchValues
}

func NewStringSearchValues(vals [][]rune, ignoreCase bool) StringSearchValues {
	min := math.MaxInt
	firstLetters := make([]rune, 0, len(vals))
	for _, val := range vals {
		if min > len(val) {
			min = len(val)
		}
		if !slices.Contains(firstLetters, val[0]) {
			firstLetters = append(firstLetters, val[0])
			if ignoreCase && val[0] != unicode.ToUpper(val[0]) {
				//if we're ignoring case and this letter is impacted by case, add it to our set
				firstLetters = append(firstLetters, unicode.ToUpper(val[0]))
			}
		}
	}

	return StringSearchValues{
		vals:        vals,
		ignoreCase:  ignoreCase,
		shortestVal: min,
		firstChars:  newRuneSearchValues(firstLetters),
	}
}

func (s StringSearchValues) StartsWith(chars []rune) int {
	for _, val := range s.vals {
		if StartsWith(chars, val) {
			return 0
		}
	}
	return -1
}

func (s StringSearchValues) StartsWithIgnoreCase(chars []rune) int {
	for _, val := range s.vals {
		if StartsWithIgnoreCase(chars, val) {
			return 0
		}
	}
	return -1
}

func (s StringSearchValues) IndexOfAny(in []rune) int {
	// go through our input once
	end := len(in) - s.shortestVal
	for i := 0; i <= end; i++ {
		// check if the char are in our starting chars
		j := s.firstChars.IndexOfAny(in[i:])
		// first chars not found at all
		if j < 0 {
			return -1
		}
		j += i
		// found a first char, do our full search through each item
		for _, val := range s.vals {
			if len(in)-j >= len(val) && Equals(in, j, len(val), val) {
				return j
			}
			if s.ignoreCase && len(in)-j >= len(val) && EqualsIgnoreCase(in, j, len(val), val) {
				return j
			}
		}
		//skip ahead
		i = j
	}

	return -1
}
