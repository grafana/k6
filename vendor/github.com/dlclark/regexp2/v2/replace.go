package regexp2

import (
	"bytes"
	"errors"

	"github.com/dlclark/regexp2/v2/helpers"
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

func writeUnmatched(buf *bytes.Buffer, input string, d *decodedInput, start, end int) {
	// Equal byte/rune lengths can include invalid UTF-8, which must be re-encoded.
	if d.ascii {
		buf.WriteString(input[start:end])
	} else {
		writeRunes(buf, d.runes, start, end)
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

	if !regex.RightToLeft() {
		prevat := 0
		for m != nil {
			start, end := matchInputSpan(m)
			if start > prevat {
				buf.WriteString(input[prevat:start])
			}
			prevat = end
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

		if prevat < len(input) {
			buf.WriteString(input[prevat:])
		}
	} else {
		prevat := len(input)
		var al []string

		for m != nil {
			start, end := matchInputSpan(m)
			if end < prevat {
				al = append(al, input[end:prevat])
			}
			prevat = start
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
			buf.WriteString(input[:prevat])
		}

		for i := len(al) - 1; i >= 0; i-- {
			buf.WriteString(al[i])
		}
	}

	return buf.String(), nil
}

// matchInputSpan returns the UTF-8 byte range of m in its original string input.
func matchInputSpan(m *Match) (start, end int) {
	if m.text != nil && m.text.hasStringInput {
		start, length := m.ByteRange()
		return start, start + length
	}
	return m.RuneIndex, m.RuneIndex + m.RuneLength
}

func replaceRunnerLTR(regex *Regexp, data *syntax.ReplacerData, input string, startAt, count int) (string, error) {
	if startAt > len(input) {
		return "", errors.New("startAt must be less than the length of the input string")
	}
	// Short inputs are cheaper to decode directly. For longer inputs, validate
	// startAt before rejecting a miss. Keep the original scan start: anchors
	// and replacement rules may need context before the candidate.
	if len(input) >= helpers.ASCIISearchMin {
		if _, ok, err := regex.findStringMatchStart(input, startAt); err != nil {
			return "", err
		} else if !ok {
			return input, nil
		}
	}

	runner := regex.getRunner()
	d := decodeInput(input, startAt, 0, regex.optimizations.MaxCachedRuneBufferLength, false)
	text := d.runes
	textInfo := newStringMatchText(input, text)
	defer func() {
		regex.putRunner(runner)
		d.release()
	}()
	runeStart := d.runeStart
	if startAt >= 0 && runeStart < 0 {
		return "", errors.New("startAt must align to the start of a valid rune in the input string")
	}
	if runeStart < 0 {
		runeStart = 0
	}

	m, err := runner.scan(text, textInfo, runeStart, runeStart, -1, true, regex.MatchTimeout)
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

		local := m.runeSliceIndex()
		if local != prevat {
			writeUnmatched(buf, input, &d, prevat, local)
		}
		prevat = local + m.RuneLength
		replacementImpl(data, buf, m)

		count--
		if count == 0 {
			break
		}

		m, err = runner.scan(text, textInfo, m.textpos, m.textpos, m.RuneLength, true, regex.MatchTimeout)
		if err != nil {
			return "", err
		}
	}

	if prevat < len(text) {
		writeUnmatched(buf, input, &d, prevat, len(text))
	}
	return buf.String(), nil
}

func replaceRunnerRTL(regex *Regexp, data *syntax.ReplacerData, input string, startAt, count int) (string, error) {
	if startAt > len(input) {
		return "", errors.New("startAt must be less than the length of the input string")
	}

	runner := regex.getRunner()
	d := decodeInput(input, startAt, 0, regex.optimizations.MaxCachedRuneBufferLength, false)
	text := d.runes
	textInfo := newStringMatchText(input, text)
	defer func() {
		regex.putRunner(runner)
		d.release()
	}()
	runeStart := d.runeStart
	if startAt >= 0 && runeStart < 0 {
		return "", errors.New("startAt must align to the start of a valid rune in the input string")
	}
	if runeStart < 0 {
		runeStart = len(text)
	}

	m, err := runner.scan(text, textInfo, runeStart, runeStart, -1, true, regex.MatchTimeout)
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

		local := m.runeSliceIndex()
		if local+m.RuneLength != prevat {
			// As in writeUnmatched, only ASCII permits copying original bytes.
			if d.ascii {
				al = append(al, input[local+m.RuneLength:prevat])
			} else {
				al = append(al, string(text[local+m.RuneLength:prevat]))
			}
		}
		prevat = local
		replacementImplRTL(data, &al, m)

		count--
		if count == 0 {
			break
		}

		m, err = runner.scan(text, textInfo, m.textpos, m.textpos, m.RuneLength, true, regex.MatchTimeout)
		if err != nil {
			return "", err
		}
	}

	if prevat > 0 {
		writeUnmatched(buf, input, &d, 0, prevat)
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
				end := m.runeSliceIndex()
				for i := 0; i < end; i++ {
					buf.WriteRune(m.text.runes[i])
				}
			case replaceRightPortion:
				for i := m.runeSliceIndex() + m.RuneLength; i < len(m.text.runes); i++ {
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

	// The caller reverses the complete segment list, so emit each
	// replacement's fragments in reverse order as well.
	for i := len(data.Rules) - 1; i >= 0; i-- {
		r := data.Rules[i]
		buf.Reset()
		if r >= 0 { // string lookup
			l = append(l, data.Strings[r])
		} else if r < -replaceSpecials { // group lookup
			m.groupValueAppendToBuf(-replaceSpecials-1-r, buf)
			l = append(l, buf.String())
		} else {
			switch -replaceSpecials - 1 - r { // special insertion patterns
			case replaceLeftPortion:
				end := m.runeSliceIndex()
				for i := 0; i < end; i++ {
					buf.WriteRune(m.text.runes[i])
				}
			case replaceRightPortion:
				for i := m.runeSliceIndex() + m.RuneLength; i < len(m.text.runes); i++ {
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
