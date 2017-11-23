package highlight

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// RunePos returns the rune index of a given byte index
// This could cause problems if the byte index is between code points
func runePos(p int, str string) int {
	if p < 0 {
		return 0
	}
	if p >= len(str) {
		return utf8.RuneCountInString(str)
	}
	return utf8.RuneCountInString(str[:p])
}

func combineLineMatch(src, dst LineMatch) LineMatch {
	for k, v := range src {
		if g, ok := dst[k]; ok {
			if g == 0 {
				dst[k] = v
			}
		} else {
			dst[k] = v
		}
	}
	return dst
}

// A State represents the region at the end of a line
type State *region

// LineStates is an interface for a buffer-like object which can also store the states and matches for every line
type LineStates interface {
	Line(n int) string
	LinesNum() int
	State(lineN int) State
	SetState(lineN int, s State)
	SetMatch(lineN int, m LineMatch)
}

// A Highlighter contains the information needed to highlight a string
type Highlighter struct {
	lastRegion *region
	Def        *Def
}

// NewHighlighter returns a new highlighter from the given syntax definition
func NewHighlighter(def *Def) *Highlighter {
	h := new(Highlighter)
	h.Def = def
	return h
}

// LineMatch represents the syntax highlighting matches for one line. Each index where the coloring is changed is marked with that
// color's group (represented as one byte)
type LineMatch map[int]Group

func findIndex(regex *regexp.Regexp, skip *regexp.Regexp, str []rune, canMatchStart, canMatchEnd bool) []int {
	regexStr := regex.String()
	if strings.Contains(regexStr, "^") {
		if !canMatchStart {
			return nil
		}
	}
	if strings.Contains(regexStr, "$") {
		if !canMatchEnd {
			return nil
		}
	}

	var strbytes []byte
	if skip != nil {
		strbytes = skip.ReplaceAllFunc([]byte(string(str)), func(match []byte) []byte {
			res := make([]byte, utf8.RuneCount(match))
			return res
		})
	} else {
		strbytes = []byte(string(str))
	}

	match := regex.FindIndex(strbytes)
	if match == nil {
		return nil
	}
	// return []int{match.Index, match.Index + match.Length}
	return []int{runePos(match[0], string(str)), runePos(match[1], string(str))}
}

func findAllIndex(regex *regexp.Regexp, str []rune, canMatchStart, canMatchEnd bool) [][]int {
	regexStr := regex.String()
	if strings.Contains(regexStr, "^") {
		if !canMatchStart {
			return nil
		}
	}
	if strings.Contains(regexStr, "$") {
		if !canMatchEnd {
			return nil
		}
	}
	matches := regex.FindAllIndex([]byte(string(str)), -1)
	for i, m := range matches {
		matches[i][0] = runePos(m[0], string(str))
		matches[i][1] = runePos(m[1], string(str))
	}
	return matches
}

func (h *Highlighter) highlightRegion(highlights LineMatch, start int, canMatchEnd bool, lineNum int, line []rune, curRegion *region, statesOnly bool) LineMatch {
	// highlights := make(LineMatch)

	if start == 0 {
		if !statesOnly {
			if _, ok := highlights[0]; !ok {
				highlights[0] = curRegion.group
			}
		}
	}

	loc := findIndex(curRegion.end, curRegion.skip, line, start == 0, canMatchEnd)
	if loc != nil {
		if !statesOnly {
			highlights[start+loc[0]] = curRegion.limitGroup
		}
		if curRegion.parent == nil {
			if !statesOnly {
				highlights[start+loc[1]] = 0
				h.highlightRegion(highlights, start, false, lineNum, line[:loc[0]], curRegion, statesOnly)
			}
			h.highlightEmptyRegion(highlights, start+loc[1], canMatchEnd, lineNum, line[loc[1]:], statesOnly)
			return highlights
		}
		if !statesOnly {
			highlights[start+loc[1]] = curRegion.parent.group
			h.highlightRegion(highlights, start, false, lineNum, line[:loc[0]], curRegion, statesOnly)
		}
		h.highlightRegion(highlights, start+loc[1], canMatchEnd, lineNum, line[loc[1]:], curRegion.parent, statesOnly)
		return highlights
	}

	if len(line) == 0 || statesOnly {
		if canMatchEnd {
			h.lastRegion = curRegion
		}

		return highlights
	}

	firstLoc := []int{len(line), 0}

	var firstRegion *region
	for _, r := range curRegion.rules.regions {
		loc := findIndex(r.start, nil, line, start == 0, canMatchEnd)
		if loc != nil {
			if loc[0] < firstLoc[0] {
				firstLoc = loc
				firstRegion = r
			}
		}
	}
	if firstLoc[0] != len(line) {
		highlights[start+firstLoc[0]] = firstRegion.limitGroup
		h.highlightRegion(highlights, start, false, lineNum, line[:firstLoc[0]], curRegion, statesOnly)
		h.highlightRegion(highlights, start+firstLoc[1], canMatchEnd, lineNum, line[firstLoc[1]:], firstRegion, statesOnly)
		return highlights
	}

	fullHighlights := make([]Group, len([]rune(string(line))))
	for i := 0; i < len(fullHighlights); i++ {
		fullHighlights[i] = curRegion.group
	}

	for _, p := range curRegion.rules.patterns {
		matches := findAllIndex(p.regex, line, start == 0, canMatchEnd)
		for _, m := range matches {
			for i := m[0]; i < m[1]; i++ {
				fullHighlights[i] = p.group
			}
		}
	}
	for i, h := range fullHighlights {
		if i == 0 || h != fullHighlights[i-1] {
			// if _, ok := highlights[start+i]; !ok {
			highlights[start+i] = h
			// }
		}
	}

	if canMatchEnd {
		h.lastRegion = curRegion
	}

	return highlights
}

func (h *Highlighter) highlightEmptyRegion(highlights LineMatch, start int, canMatchEnd bool, lineNum int, line []rune, statesOnly bool) LineMatch {
	if len(line) == 0 {
		if canMatchEnd {
			h.lastRegion = nil
		}
		return highlights
	}

	firstLoc := []int{len(line), 0}
	var firstRegion *region
	for _, r := range h.Def.rules.regions {
		loc := findIndex(r.start, nil, line, start == 0, canMatchEnd)
		if loc != nil {
			if loc[0] < firstLoc[0] {
				firstLoc = loc
				firstRegion = r
			}
		}
	}
	if firstLoc[0] != len(line) {
		if !statesOnly {
			highlights[start+firstLoc[0]] = firstRegion.limitGroup
		}
		h.highlightEmptyRegion(highlights, start, false, lineNum, line[:firstLoc[0]], statesOnly)
		h.highlightRegion(highlights, start+firstLoc[1], canMatchEnd, lineNum, line[firstLoc[1]:], firstRegion, statesOnly)
		return highlights
	}

	if statesOnly {
		if canMatchEnd {
			h.lastRegion = nil
		}

		return highlights
	}

	fullHighlights := make([]Group, len(line))
	for _, p := range h.Def.rules.patterns {
		matches := findAllIndex(p.regex, line, start == 0, canMatchEnd)
		for _, m := range matches {
			for i := m[0]; i < m[1]; i++ {
				fullHighlights[i] = p.group
			}
		}
	}
	for i, h := range fullHighlights {
		if i == 0 || h != fullHighlights[i-1] {
			// if _, ok := highlights[start+i]; !ok {
			highlights[start+i] = h
			// }
		}
	}

	if canMatchEnd {
		h.lastRegion = nil
	}

	return highlights
}

// HighlightString syntax highlights a string
// Use this function for simple syntax highlighting and use the other functions for
// more advanced syntax highlighting. They are optimized for quick rehighlighting of the same
// text with minor changes made
func (h *Highlighter) HighlightString(input string) []LineMatch {
	lines := strings.Split(input, "\n")
	var lineMatches []LineMatch

	for i := 0; i < len(lines); i++ {
		line := []rune(lines[i])
		highlights := make(LineMatch)

		if i == 0 || h.lastRegion == nil {
			lineMatches = append(lineMatches, h.highlightEmptyRegion(highlights, 0, true, i, line, false))
		} else {
			lineMatches = append(lineMatches, h.highlightRegion(highlights, 0, true, i, line, h.lastRegion, false))
		}
	}

	return lineMatches
}

// HighlightStates correctly sets all states for the buffer
func (h *Highlighter) HighlightStates(input LineStates) {
	for i := 0; i < input.LinesNum(); i++ {
		line := []rune(input.Line(i))
		// highlights := make(LineMatch)

		if i == 0 || h.lastRegion == nil {
			h.highlightEmptyRegion(nil, 0, true, i, line, true)
		} else {
			h.highlightRegion(nil, 0, true, i, line, h.lastRegion, true)
		}

		curState := h.lastRegion

		input.SetState(i, curState)
	}
}

// HighlightMatches sets the matches for each line in between startline and endline
// It sets all other matches in the buffer to nil to conserve memory
// This assumes that all the states are set correctly
func (h *Highlighter) HighlightMatches(input LineStates, startline, endline int) {
	for i := startline; i < endline; i++ {
		if i >= input.LinesNum() {
			break
		}

		line := []rune(input.Line(i))
		highlights := make(LineMatch)

		var match LineMatch
		if i == 0 || input.State(i-1) == nil {
			match = h.highlightEmptyRegion(highlights, 0, true, i, line, false)
		} else {
			match = h.highlightRegion(highlights, 0, true, i, line, input.State(i-1), false)
		}

		input.SetMatch(i, match)
	}
}

// ReHighlightStates will scan down from `startline` and set the appropriate end of line state
// for each line until it comes across the same state in two consecutive lines
func (h *Highlighter) ReHighlightStates(input LineStates, startline int) {
	// lines := input.LineData()

	h.lastRegion = nil
	if startline > 0 {
		h.lastRegion = input.State(startline - 1)
	}
	for i := startline; i < input.LinesNum(); i++ {
		line := []rune(input.Line(i))
		// highlights := make(LineMatch)

		// var match LineMatch
		if i == 0 || h.lastRegion == nil {
			h.highlightEmptyRegion(nil, 0, true, i, line, true)
		} else {
			h.highlightRegion(nil, 0, true, i, line, h.lastRegion, true)
		}
		curState := h.lastRegion
		lastState := input.State(i)

		input.SetState(i, curState)

		if curState == lastState {
			break
		}
	}
}

// ReHighlightLine will rehighlight the state and match for a single line
func (h *Highlighter) ReHighlightLine(input LineStates, lineN int) {
	line := []rune(input.Line(lineN))
	highlights := make(LineMatch)

	h.lastRegion = nil
	if lineN > 0 {
		h.lastRegion = input.State(lineN - 1)
	}

	var match LineMatch
	if lineN == 0 || h.lastRegion == nil {
		match = h.highlightEmptyRegion(highlights, 0, true, lineN, line, false)
	} else {
		match = h.highlightRegion(highlights, 0, true, lineN, line, h.lastRegion, false)
	}
	curState := h.lastRegion

	input.SetMatch(lineN, match)
	input.SetState(lineN, curState)
}
