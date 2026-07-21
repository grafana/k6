package regexp2

import (
	"errors"
	"math"
)

// Split splits the given input string using the pattern and returns
// a slice of the parts. Count limits the number of matches to process.
// If Count is -1, then it will process the input fully.
// If Count is 0, returns nil. If Count is 1, returns the original input.
// The only expected error is a Timeout, if it's set.
//
// If capturing parentheses are used in the Regex expression, any captured
// text is included in the resulting string array
// For example, a pattern of "-" Split("a-b") will return ["a", "b"]
// but a pattern with "(-)" Split ("a-b") will return ["a", "-", "b"]
func (re *Regexp) Split(input string, count int) ([]string, error) {
	if count < -1 {
		return nil, errors.New("count too small")
	}
	if count == 0 {
		return nil, nil
	}
	if count == 1 {
		return []string{input}, nil
	}
	if count == -1 {
		// no limit
		count = math.MaxInt
	}

	// iterate through the matches
	priorIndex := 0
	var retVal []string
	var txt []rune

	m, err := re.FindStringMatch(input)

	for ; m != nil && count > 0; m, err = re.FindNextMatch(m) {
		txt = m.text.runes
		// if we have an m, we don't have an err
		// append our match
		retVal = append(retVal, string(txt[priorIndex:m.RuneIndex]))
		// append any capture groups, skipping group 0
		gs := m.Groups()
		for i := 1; i < len(gs); i++ {
			retVal = append(retVal, gs[i].String())
		}
		priorIndex = m.RuneIndex + m.RuneLength
		count--
	}

	if err != nil {
		return nil, err
	}

	if txt == nil {
		// we never matched, return the original string
		return []string{input}, nil
	}

	// append our remainder
	retVal = append(retVal, string(txt[priorIndex:]))

	return retVal, nil
}
