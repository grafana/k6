package regexp2

import (
	"bytes"
	"errors"

	"github.com/dlclark/regexp2/v2/syntax"
)

const (
	replaceSpecials     = 4
	replaceLeftPortion  = -1
	replaceRightPortion = -2
	replaceLastGroup    = -3
	replaceWholeString  = -4
)

// MatchEvaluator is a function that takes a match and returns a replacement string to be used
type MatchEvaluator func(Match) string

// Three very similar algorithms appear below: replace (pattern),
// replace (evaluator), and split.

func writeRunes(buf *bytes.Buffer, text []rune, start, end int) {
	for i := start; i < end; i++ {
		buf.WriteRune(text[i])
	}
}

func compactBalancedMatches(m *Match) {
	for cap := 0; cap < len(m.matchcount); cap++ {
		limit := m.matchcount[cap] * 2
		matcharray := m.matches[cap]

		var i, j int
		for i = 0; i < limit; i++ {
			if matcharray[i] < 0 {
				break
			}
		}

		for j = i; i < limit; i++ {
			if matcharray[i] < 0 {
				j--
			} else {
				if i != j {
					matcharray[j] = matcharray[i]
				}
				j++
			}
		}

		m.matchcount[cap] = j / 2
	}
	m.balancing = false
}

// Replace Replaces all occurrences of the regex in the string with the
// replacement pattern.
//
// Note that the special case of no matches is handled on its own:
// with no matches, the input string is returned unchanged.
// The right-to-left case is split out because StringBuilder
// doesn't handle right-to-left string building directly very well.
func replace(regex *Regexp, data *syntax.ReplacerData, evaluator MatchEvaluator, input string, startAt, count int) (string, error) {
	if count < -1 {
		return "", errors.New("count too small")
	}
	if count == 0 {
		return "", nil
	}

	if evaluator == nil {
		if !regex.RightToLeft() {
			return replaceRunnerLTR(regex, data, input, startAt, count)
		}
		return replaceRunnerRTL(regex, data, input, startAt, count)
	}

	m, err := regex.FindStringMatchStartingAt(input, startAt)

	if err != nil {
		return "", err
	}
	if m == nil {
		return input, nil
	}

	buf := &bytes.Buffer{}
	text := m.text.runes

	if !regex.RightToLeft() {
		prevat := 0
		for m != nil {
			if m.RuneIndex != prevat {
				buf.WriteString(string(text[prevat:m.RuneIndex]))
			}
			prevat = m.RuneIndex + m.RuneLength
			buf.WriteString(evaluator(*m))

			count--
			if count == 0 {
				break
			}
			m, err = regex.FindNextMatch(m)
			if err != nil {
				return "", err
			}
		}

		if prevat < len(text) {
			buf.WriteString(string(text[prevat:]))
		}
	} else {
		prevat := len(text)
		var al []string

		for m != nil {
			if m.RuneIndex+m.RuneLength != prevat {
				al = append(al, string(text[m.RuneIndex+m.RuneLength:prevat]))
			}
			prevat = m.RuneIndex
			al = append(al, evaluator(*m))

			count--
			if count == 0 {
				break
			}
			m, err = regex.FindNextMatch(m)
			if err != nil {
				return "", err
			}
		}

		if prevat > 0 {
			buf.WriteString(string(text[:prevat]))
		}

		for i := len(al) - 1; i >= 0; i-- {
			buf.WriteString(al[i])
		}
	}

	return buf.String(), nil
}

func replaceRunnerLTR(regex *Regexp, data *syntax.ReplacerData, input string, startAt, count int) (string, error) {
	if startAt > len(input) {
		return "", errors.New("startAt must be less than the length of the input string")
	}

	runner := regex.getRunner()
	text, runeStart, pooledText := runner.decodeStringWithStart(input, startAt)
	textInfo := newStringMatchText(input, text)
	defer func() {
		regex.putRunner(runner)
		if pooledText != nil {
			pooledRuneBuffers.put(pooledText)
		}
	}()
	if startAt >= 0 && runeStart < 0 {
		return "", errors.New("startAt must align to the start of a valid rune in the input string")
	}
	if runeStart < 0 {
		runeStart = 0
	}

	m, err := runner.scan(text, textInfo, runeStart, true, regex.MatchTimeout)
	if err != nil {
		return "", err
	}
	if m == nil {
		return input, nil
	}

	buf, pooledBuf := getPooledReplaceBuffer(len(input), regex.optimizations.MaxCachedReplaceBufferLength)
	if pooledBuf != nil {
		defer putPooledReplaceBuffer(buf, pooledBuf)
	}

	prevat := 0
	for m != nil {
		if m.balancing {
			compactBalancedMatches(m)
		}

		if m.RuneIndex != prevat {
			writeRunes(buf, text, prevat, m.RuneIndex)
		}
		prevat = m.RuneIndex + m.RuneLength
		replacementImpl(data, buf, m)

		count--
		if count == 0 {
			break
		}

		scanStart := m.textpos
		if m.RuneLength == 0 {
			if scanStart >= len(text) {
				break
			}
			scanStart++
		}

		m, err = runner.scan(text, textInfo, scanStart, true, regex.MatchTimeout)
		if err != nil {
			return "", err
		}
	}

	if prevat < len(text) {
		writeRunes(buf, text, prevat, len(text))
	}
	return buf.String(), nil
}

func replaceRunnerRTL(regex *Regexp, data *syntax.ReplacerData, input string, startAt, count int) (string, error) {
	if startAt > len(input) {
		return "", errors.New("startAt must be less than the length of the input string")
	}

	runner := regex.getRunner()
	text, runeStart, pooledText := runner.decodeStringWithStart(input, startAt)
	textInfo := newStringMatchText(input, text)
	defer func() {
		regex.putRunner(runner)
		if pooledText != nil {
			pooledRuneBuffers.put(pooledText)
		}
	}()
	if startAt >= 0 && runeStart < 0 {
		return "", errors.New("startAt must align to the start of a valid rune in the input string")
	}
	if runeStart < 0 {
		runeStart = len(text)
	}

	m, err := runner.scan(text, textInfo, runeStart, true, regex.MatchTimeout)
	if err != nil {
		return "", err
	}
	if m == nil {
		return input, nil
	}

	buf, pooledBuf := getPooledReplaceBuffer(len(input), regex.optimizations.MaxCachedReplaceBufferLength)
	if pooledBuf != nil {
		defer putPooledReplaceBuffer(buf, pooledBuf)
	}

	prevat := len(text)
	var al []string

	for m != nil {
		if m.balancing {
			compactBalancedMatches(m)
		}

		if m.RuneIndex+m.RuneLength != prevat {
			al = append(al, string(text[m.RuneIndex+m.RuneLength:prevat]))
		}
		prevat = m.RuneIndex
		replacementImplRTL(data, &al, m)

		count--
		if count == 0 {
			break
		}

		scanStart := m.textpos
		if m.RuneLength == 0 {
			if scanStart <= 0 {
				break
			}
			scanStart--
		}

		m, err = runner.scan(text, textInfo, scanStart, true, regex.MatchTimeout)
		if err != nil {
			return "", err
		}
	}

	if prevat > 0 {
		writeRunes(buf, text, 0, prevat)
	}
	for i := len(al) - 1; i >= 0; i-- {
		buf.WriteString(al[i])
	}
	return buf.String(), nil
}

// Given a Match, emits into the StringBuilder the evaluated
// substitution pattern.
func replacementImpl(data *syntax.ReplacerData, buf *bytes.Buffer, m *Match) {
	for _, r := range data.Rules {

		if r >= 0 { // string lookup
			buf.WriteString(data.Strings[r])
		} else if r < -replaceSpecials { // group lookup
			m.groupValueAppendToBuf(-replaceSpecials-1-r, buf)
		} else {
			switch -replaceSpecials - 1 - r { // special insertion patterns
			case replaceLeftPortion:
				for i := 0; i < m.RuneIndex; i++ {
					buf.WriteRune(m.text.runes[i])
				}
			case replaceRightPortion:
				for i := m.RuneIndex + m.RuneLength; i < len(m.text.runes); i++ {
					buf.WriteRune(m.text.runes[i])
				}
			case replaceLastGroup:
				m.groupValueAppendToBuf(m.GroupCount()-1, buf)
			case replaceWholeString:
				for i := 0; i < len(m.text.runes); i++ {
					buf.WriteRune(m.text.runes[i])
				}
			}
		}
	}
}

func replacementImplRTL(data *syntax.ReplacerData, al *[]string, m *Match) {
	l := *al
	buf := &bytes.Buffer{}

	for _, r := range data.Rules {
		buf.Reset()
		if r >= 0 { // string lookup
			l = append(l, data.Strings[r])
		} else if r < -replaceSpecials { // group lookup
			m.groupValueAppendToBuf(-replaceSpecials-1-r, buf)
			l = append(l, buf.String())
		} else {
			switch -replaceSpecials - 1 - r { // special insertion patterns
			case replaceLeftPortion:
				for i := 0; i < m.RuneIndex; i++ {
					buf.WriteRune(m.text.runes[i])
				}
			case replaceRightPortion:
				for i := m.RuneIndex + m.RuneLength; i < len(m.text.runes); i++ {
					buf.WriteRune(m.text.runes[i])
				}
			case replaceLastGroup:
				m.groupValueAppendToBuf(m.GroupCount()-1, buf)
			case replaceWholeString:
				for i := 0; i < len(m.text.runes); i++ {
					buf.WriteRune(m.text.runes[i])
				}
			}
			l = append(l, buf.String())
		}
	}

	*al = l
}
