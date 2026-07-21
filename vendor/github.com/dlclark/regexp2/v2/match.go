package regexp2

import (
	"bytes"
	"fmt"
	"unicode/utf8"
)

// Match is a single regex result match that contains groups and repeated captures
//
//		-Groups
//	   -Capture
type Match struct {
	Group //embeded group 0

	regex       *Regexp
	otherGroups []Group

	// input to the match
	textpos   int
	textstart int

	capcount   int
	sparseCaps map[int]int

	// output from the match
	matches    [][]int
	matchcount []int

	// whether we've done any balancing with this match.  If we
	// have done balancing, we'll need to do extra work in Tidy().
	balancing bool
}

// Group is an explicit or implit (group 0) matched group within the pattern
type Group struct {
	Capture // the last capture of this group is embeded for ease of use

	Name     string    // group name
	Captures []Capture // captures of this group
}

// Capture is a single capture of text within the larger original string
type Capture struct {
	// the original string
	text *matchText
	// RuneIndex is the position in the underlying rune slice where the first character of
	// captured substring was found. Even if you pass in a string this will be in Runes.
	RuneIndex int
	// RuneLength is the number of runes in the captured substring.
	RuneLength int
}

type matchText struct {
	runes            []rune
	input            string
	hasStringInput   bool
	byteOffsets      []int
	byteOffsetsReady bool
}

// String returns the captured text as a String
func (c *Capture) String() string {
	return string(c.text.runes[c.RuneIndex : c.RuneIndex+c.RuneLength])
}

// Runes returns the captured text as a rune slice
func (c *Capture) Runes() []rune {
	return c.text.runes[c.RuneIndex : c.RuneIndex+c.RuneLength]
}

// ByteRange returns the UTF-8 byte index and byte length of the captured
// substring. The first call lazily caches byte offsets on shared match text,
// so it is not safe to call concurrently with ByteRange on another capture
// from the same match until the cache has been initialized.
func (c *Capture) ByteRange() (index, length int) {
	if c.text == nil {
		return c.RuneIndex, c.RuneLength
	}
	return c.text.byteRange(c.RuneIndex, c.RuneLength)
}

func newMatchText(r []rune) *matchText {
	return &matchText{runes: r}
}

func newStringMatchText(input string, r []rune) *matchText {
	return &matchText{runes: r, input: input, hasStringInput: true}
}

func (t *matchText) byteRange(runeIndex, runeLength int) (int, int) {
	if !t.byteOffsetsReady {
		t.byteOffsets = t.buildByteOffsets()
		t.byteOffsetsReady = true
	}
	if t.byteOffsets == nil {
		return runeIndex, runeLength
	}
	byteIndex := t.byteOffsets[runeIndex]
	return byteIndex, t.byteOffsets[runeIndex+runeLength] - byteIndex
}

func (t *matchText) buildByteOffsets() []int {
	if t.hasStringInput {
		return stringByteOffsets(t.input)
	}
	return runeByteOffsets(t.runes)
}

func stringByteOffsets(s string) []int {
	var byteOffsets []int
	runeIndex := 0
	for strIdx, ch := range s {
		if byteOffsets != nil {
			byteOffsets[runeIndex] = strIdx
		}
		runeLen := utf8.RuneLen(ch)
		if ch == utf8.RuneError {
			_, runeLen = utf8.DecodeRuneInString(s[strIdx:])
		}
		if byteOffsets == nil && (strIdx != runeIndex || runeLen != 1) {
			byteOffsets = make([]int, len(s)+1)
			for i := 0; i < runeIndex; i++ {
				byteOffsets[i] = i
			}
			byteOffsets[runeIndex] = strIdx
		}
		runeIndex++
	}
	if byteOffsets != nil {
		byteOffsets[runeIndex] = len(s)
		return byteOffsets[:runeIndex+1]
	}
	return nil
}

func runeByteOffsets(runes []rune) []int {
	var byteOffsets []int
	bytePos := 0
	for i, ch := range runes {
		if byteOffsets != nil {
			byteOffsets[i] = bytePos
		}
		runeLen := utf8.RuneLen(ch)
		if runeLen < 0 {
			runeLen = utf8.RuneLen(utf8.RuneError)
		}
		if byteOffsets == nil && runeLen != 1 {
			byteOffsets = make([]int, len(runes)+1)
			for j := 0; j < i; j++ {
				byteOffsets[j] = j
			}
			byteOffsets[i] = bytePos
		}
		bytePos += runeLen
	}
	if byteOffsets != nil {
		byteOffsets[len(runes)] = bytePos
	}
	return byteOffsets
}

func newMatch(regex *Regexp, capcount int, text *matchText, startpos int) *Match {
	m := Match{
		regex:      regex,
		matchcount: make([]int, capcount),
		matches:    make([][]int, capcount),
		textstart:  startpos,
		balancing:  false,
	}
	if (regex.options & ECMAScript) == 0 {
		m.Name = "0"
	}
	m.text = text
	m.matches[0] = make([]int, 2)
	return &m
}

func newMatchSparse(regex *Regexp, caps map[int]int, capcount int, text *matchText, startpos int) *Match {
	m := newMatch(regex, capcount, text, startpos)
	m.sparseCaps = caps
	return m
}

func (m *Match) reset(text *matchText, textstart int) {
	m.text = text
	m.textstart = textstart
	for i := 0; i < len(m.matchcount); i++ {
		m.matchcount[i] = 0
	}
	m.balancing = false
}

func (m *Match) tidy(textpos int) {

	interval := m.matches[0]
	setCaptureFields(&m.Capture, interval[0], interval[1])
	m.textpos = textpos
	m.capcount = m.matchcount[0]
	//copy our root capture to the list
	m.Captures = []Capture{m.Capture}

	if m.balancing {
		// The idea here is that we want to compact all of our unbalanced captures.  To do that we
		// use j basically as a count of how many unbalanced captures we have at any given time
		// (really j is an index, but j/2 is the count).  First we skip past all of the real captures
		// until we find a balance captures.  Then we check each subsequent entry.  If it's a balance
		// capture (it's negative), we decrement j.  If it's a real capture, we increment j and copy
		// it down to the last free position.
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
					// skip negative values
					j--
				} else {
					// but if we find something positive (an actual capture), copy it back to the last
					// unbalanced position.
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
}

// isMatched tells if a group was matched by capnum
func (m *Match) isMatched(cap int) bool {
	return cap < len(m.matchcount) && m.matchcount[cap] > 0 && m.matches[cap][m.matchcount[cap]*2-1] != (-3+1)
}

// matchIndex returns the index of the last specified matched group by capnum
func (m *Match) matchIndex(cap int) int {
	i := m.matches[cap][m.matchcount[cap]*2-2]
	if i >= 0 {
		return i
	}

	return m.matches[cap][-3-i]
}

// matchLength returns the length of the last specified matched group by capnum
func (m *Match) matchLength(cap int) int {
	i := m.matches[cap][m.matchcount[cap]*2-1]
	if i >= 0 {
		return i
	}

	return m.matches[cap][-3-i]
}

// Nonpublic builder: add a capture to the group specified by "c"
func (m *Match) addMatch(c, start, l int) {

	if m.matches[c] == nil {
		m.matches[c] = make([]int, 2)
	}

	capcount := m.matchcount[c]

	if capcount*2+2 > len(m.matches[c]) {
		oldmatches := m.matches[c]
		newmatches := make([]int, capcount*8)
		copy(newmatches, oldmatches[:capcount*2])
		m.matches[c] = newmatches
	}

	m.matches[c][capcount*2] = start
	m.matches[c][capcount*2+1] = l
	m.matchcount[c] = capcount + 1
	//log.Printf("addMatch: c=%v, i=%v, l=%v ... matches: %v", c, start, l, m.matches)
}

// Nonpublic builder: Add a capture to balance the specified group.  This is used by the
//
//	balanced match construct. (?<foo-foo2>...)
//
// If there were no such thing as backtracking, this would be as simple as calling RemoveMatch(c).
// However, since we have backtracking, we need to keep track of everything.
func (m *Match) balanceMatch(c int) {
	m.balancing = true

	// we'll look at the last capture first
	capcount := m.matchcount[c]
	target := capcount*2 - 2

	// first see if it is negative, and therefore is a reference to the next available
	// capture group for balancing.  If it is, we'll reset target to point to that capture.
	if m.matches[c][target] < 0 {
		target = -3 - m.matches[c][target]
	}

	// move back to the previous capture
	target -= 2

	// if the previous capture is a reference, just copy that reference to the end.  Otherwise, point to it.
	if target >= 0 && m.matches[c][target] < 0 {
		m.addMatch(c, m.matches[c][target], m.matches[c][target+1])
	} else {
		m.addMatch(c, -3-target, -4-target /* == -3 - (target + 1) */)
	}
}

// Nonpublic builder: removes a group match by capnum
func (m *Match) removeMatch(c int) {
	m.matchcount[c]--
}

// GroupCount returns the number of groups this match has matched
func (m *Match) GroupCount() int {
	return len(m.matchcount)
}

// GroupByName returns a group based on the name of the group, or nil if the group name does not exist
func (m *Match) GroupByName(name string) *Group {
	num := m.regex.GroupNumberFromName(name)
	if num < 0 {
		return nil
	}
	return m.GroupByNumber(num)
}

// GroupByNumber returns a group based on the number of the group, or nil if the group number does not exist
func (m *Match) GroupByNumber(num int) *Group {
	// check our sparse map
	if m.sparseCaps != nil {
		if newNum, ok := m.sparseCaps[num]; ok {
			num = newNum
		}
	}
	if num >= len(m.matchcount) || num < 0 {
		return nil
	}

	if num == 0 {
		return &m.Group
	}

	m.populateOtherGroups()

	return &m.otherGroups[num-1]
}

// Groups returns all the capture groups, starting with group 0 (the full match)
func (m *Match) Groups() []Group {
	m.populateOtherGroups()
	g := make([]Group, len(m.otherGroups)+1)
	g[0] = m.Group
	copy(g[1:], m.otherGroups)
	return g
}

func (m *Match) populateOtherGroups() {
	// Construct all the Group objects first time called
	if m.otherGroups == nil {
		m.otherGroups = make([]Group, len(m.matchcount)-1)
		for i := 0; i < len(m.otherGroups); i++ {
			m.otherGroups[i] = newGroup(m.regex.GroupNameFromNumber(i+1), m.text, m.matches[i+1], m.matchcount[i+1])
		}
	}
}

func (m *Match) groupValueAppendToBuf(groupnum int, buf *bytes.Buffer) {
	c := m.matchcount[groupnum]
	if c == 0 {
		return
	}

	matches := m.matches[groupnum]

	index := matches[(c-1)*2]
	last := index + matches[(c*2)-1]

	for ; index < last; index++ {
		buf.WriteRune(m.text.runes[index])
	}
}

func newGroup(name string, text *matchText, caps []int, capcount int) Group {
	g := Group{}
	g.text = text
	if capcount > 0 {
		setCaptureFields(&g.Capture, caps[(capcount-1)*2], caps[(capcount*2)-1])
	}
	g.Name = name
	g.Captures = make([]Capture, capcount)
	for i := 0; i < capcount; i++ {
		g.Captures[i] = newCapture(text, caps[i*2], caps[i*2+1])
	}
	//log.Printf("newGroup! capcount %v, %+v", capcount, g)

	return g
}

func newCapture(text *matchText, runeIndex, runeLength int) Capture {
	c := Capture{text: text}
	setCaptureFields(&c, runeIndex, runeLength)
	return c
}

func setCaptureFields(c *Capture, runeIndex, runeLength int) {
	c.RuneIndex = runeIndex
	c.RuneLength = runeLength
}

func (m *Match) dump() string {
	buf := &bytes.Buffer{}
	buf.WriteRune('\n')
	if len(m.sparseCaps) > 0 {
		for k, v := range m.sparseCaps {
			fmt.Fprintf(buf, "Slot %v -> %v\n", k, v)
		}
	}

	for i, g := range m.Groups() {
		fmt.Fprintf(buf, "Group %v (%v), %v caps:\n", i, g.Name, len(g.Captures))

		for _, c := range g.Captures {
			fmt.Fprintf(buf, "  (%v, %v) %v\n", c.RuneIndex, c.RuneLength, c.String())
		}
	}
	/*
		for i := 0; i < len(m.matchcount); i++ {
			fmt.Fprintf(buf, "\nGroup %v (%v):\n", i, m.regex.GroupNameFromNumber(i))

			for j := 0; j < m.matchcount[i]; j++ {
				text := ""

				if m.matches[i][j*2] >= 0 {
					start := m.matches[i][j*2]
					text = m.text.runes[start : start+m.matches[i][j*2+1]]
				}

				fmt.Fprintf(buf, "  (%v, %v) %v\n", m.matches[i][j*2], m.matches[i][j*2+1], text)
			}
		}
	*/
	return buf.String()
}
