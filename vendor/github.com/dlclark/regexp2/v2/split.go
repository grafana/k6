package regexp2

import (
	"errors"
	"math"
	"slices"
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

	startAt, ok, err := re.findStringMatchStart(input, -1)
	if err != nil {
		return nil, err
	}
	if !ok {
		return []string{input}, nil
	}
	d := decodeInput(input, startAt, re.decodeFrom(input, startAt), re.optimizations.MaxCachedRuneBufferLength, false)
	runner := re.getRunner()
	defer func() {
		re.putRunner(runner)
		d.release()
	}()
	text := newStringMatchTextAt(input, d.runes, 0, d.byteOffset)

	// Keep captures in the reusable match, but only materialize output strings.
	// Passing text also ensures registered engines use their capturing program.
	priorIndex := 0
	if re.RightToLeft() {
		priorIndex = len(input)
	}
	var retVal []string
	matched := false

	origin := re.stringSearchOrigin(input, -1, d.runeStart)
	m, err := runner.scan(d.runes, text, origin, d.runeStart, -1, true, re.MatchTimeout)

	for ; m != nil && count > 0; m, err = runner.scan(d.runes, text, m.textpos, m.textpos, m.RuneLength, true, re.MatchTimeout) {
		if m.balancing {
			compactBalancedMatches(m)
		}
		matched = true
		start, end := matchInputSpan(m)
		if re.RightToLeft() {
			retVal = append(retVal, input[end:priorIndex])
		} else {
			retVal = append(retVal, input[priorIndex:start])
		}
		// Preserve group order and empty strings for unmatched groups without
		// allocating Group objects or their capture histories.
		for group := 1; group < len(m.matchcount); group++ {
			value := ""
			if m.matchcount[group] > 0 {
				capture := newCapture(text, m.matchIndex(group), m.matchLength(group))
				value = capture.String()
			}
			retVal = append(retVal, value)
		}
		priorIndex = end
		if re.RightToLeft() {
			priorIndex = start
		}
		count--
	}

	if err != nil {
		return nil, err
	}

	if !matched {
		return []string{input}, nil
	}

	if re.RightToLeft() {
		retVal = append(retVal, input[:priorIndex])
		slices.Reverse(retVal)
	} else {
		retVal = append(retVal, input[priorIndex:])
	}
	return retVal, nil
}
