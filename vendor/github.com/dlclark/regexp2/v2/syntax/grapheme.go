package syntax

import "unicode"

type graphemeBreakClass uint8

const (
	graphemeOther graphemeBreakClass = iota
	graphemeCR
	graphemeLF
	graphemeControl
	graphemeExtend
	graphemeZWJ
	graphemeRegionalIndicator
	graphemePrepend
	graphemeSpacingMark
	graphemeL
	graphemeV
	graphemeT
	graphemeLV
	graphemeLVT
)

type indicConjunctBreakClass uint8

const (
	indicNone indicConjunctBreakClass = iota
	indicConsonant
	indicExtend
	indicLinker
)

type graphemeProperties struct {
	breakClass           graphemeBreakClass
	indicClass           indicConjunctBreakClass
	extendedPictographic bool
}

// NextGraphemeClusterBoundary returns the first extended-grapheme boundary
// after start, or -1 when start is not a valid input position. It implements
// the Unicode 17 rules from UAX #29 with a small state machine, avoiding the
// backtracking and repeated character-class probes of an equivalent regexp.
func NextGraphemeClusterBoundary(text []rune, start int) int {
	if start < 0 || start >= len(text) {
		return -1
	}

	previous := graphemePropertiesFor(text[start])
	state := graphemeForwardState{}
	state.consume(previous)

	for pos := start + 1; pos < len(text); pos++ {
		current := graphemePropertiesFor(text[pos])
		if isGraphemeBoundaryForward(previous.breakClass, current, state) {
			return pos
		}
		state.consume(current)
		previous = current
	}
	return len(text)
}

// PreviousGraphemeClusterBoundary returns the first extended-grapheme boundary
// before end, or -1 when end is not a valid input position.
func PreviousGraphemeClusterBoundary(text []rune, end int) int {
	if end <= 0 || end > len(text) {
		return -1
	}
	for pos := end - 1; pos > 0; pos-- {
		if isGraphemeBoundary(text, pos) {
			return pos
		}
	}
	return 0
}

type graphemeForwardState struct {
	regionalIndicatorCount  int
	extendedPictographicRun bool
	zwjAfterPictographic    bool
	indicConsonantRun       bool
	indicLinkerSeen         bool
}

func (s *graphemeForwardState) consume(properties graphemeProperties) {
	if properties.breakClass == graphemeRegionalIndicator {
		s.regionalIndicatorCount++
	} else {
		s.regionalIndicatorCount = 0
	}

	if properties.breakClass == graphemeZWJ {
		s.zwjAfterPictographic = s.extendedPictographicRun
	} else {
		s.zwjAfterPictographic = false
	}
	if properties.extendedPictographic {
		s.extendedPictographicRun = true
	} else if properties.breakClass != graphemeExtend {
		s.extendedPictographicRun = false
	}

	switch properties.indicClass {
	case indicConsonant:
		s.indicConsonantRun = true
		s.indicLinkerSeen = false
	case indicExtend:
		// Extend preserves a preceding consonant/linker run.
	case indicLinker:
		if s.indicConsonantRun {
			s.indicLinkerSeen = true
		}
	default:
		s.indicConsonantRun = false
		s.indicLinkerSeen = false
	}
}

func isGraphemeBoundaryForward(previous graphemeBreakClass, current graphemeProperties, state graphemeForwardState) bool {
	if previous == graphemeCR && current.breakClass == graphemeLF { // GB3
		return false
	}
	if isGraphemeControl(previous) || isGraphemeControl(current.breakClass) { // GB4, GB5
		return true
	}
	if hangulNoBreak(previous, current.breakClass) { // GB6, GB7, GB8
		return false
	}
	if current.breakClass == graphemeExtend || current.breakClass == graphemeZWJ || current.breakClass == graphemeSpacingMark { // GB9, GB9a
		return false
	}
	if previous == graphemePrepend { // GB9b
		return false
	}
	if current.indicClass == indicConsonant && state.indicConsonantRun && state.indicLinkerSeen { // GB9c
		return false
	}
	if current.extendedPictographic && state.zwjAfterPictographic { // GB11
		return false
	}
	if previous == graphemeRegionalIndicator && current.breakClass == graphemeRegionalIndicator && state.regionalIndicatorCount%2 == 1 { // GB12, GB13
		return false
	}
	return true
}

func isGraphemeBoundary(text []rune, pos int) bool {
	previous := graphemePropertiesFor(text[pos-1])
	current := graphemePropertiesFor(text[pos])
	if previous.breakClass == graphemeCR && current.breakClass == graphemeLF { // GB3
		return false
	}
	if isGraphemeControl(previous.breakClass) || isGraphemeControl(current.breakClass) { // GB4, GB5
		return true
	}
	if hangulNoBreak(previous.breakClass, current.breakClass) { // GB6, GB7, GB8
		return false
	}
	if current.breakClass == graphemeExtend || current.breakClass == graphemeZWJ || current.breakClass == graphemeSpacingMark { // GB9, GB9a
		return false
	}
	if previous.breakClass == graphemePrepend { // GB9b
		return false
	}
	if current.indicClass == indicConsonant && hasIndicConjunctBefore(text, pos) { // GB9c
		return false
	}
	if current.extendedPictographic && hasExtendedPictographicZWJBefore(text, pos) { // GB11
		return false
	}
	if previous.breakClass == graphemeRegionalIndicator && current.breakClass == graphemeRegionalIndicator { // GB12, GB13
		count := 0
		for i := pos - 1; i >= 0 && graphemeClass(text[i]) == graphemeRegionalIndicator; i-- {
			count++
		}
		return count%2 == 0
	}
	return true
}

func hasIndicConjunctBefore(text []rune, pos int) bool {
	linkerSeen := false
	for i := pos - 1; i >= 0; i-- {
		switch indicClass(text[i]) {
		case indicExtend:
			continue
		case indicLinker:
			linkerSeen = true
			continue
		case indicConsonant:
			return linkerSeen
		default:
			return false
		}
	}
	return false
}

func hasExtendedPictographicZWJBefore(text []rune, pos int) bool {
	i := pos - 1
	if i < 0 || graphemeClass(text[i]) != graphemeZWJ {
		return false
	}
	for i--; i >= 0 && graphemeClass(text[i]) == graphemeExtend; i-- {
	}
	return i >= 0 && unicode.Is(unicodeAliasExtended_Pictographic, text[i])
}

func isGraphemeControl(class graphemeBreakClass) bool {
	return class == graphemeCR || class == graphemeLF || class == graphemeControl
}

func hangulNoBreak(previous, current graphemeBreakClass) bool {
	return previous == graphemeL && (current == graphemeL || current == graphemeV || current == graphemeLV || current == graphemeLVT) ||
		(previous == graphemeLV || previous == graphemeV) && (current == graphemeV || current == graphemeT) ||
		(previous == graphemeLVT || previous == graphemeT) && current == graphemeT
}

func indicClass(ch rune) indicConjunctBreakClass {
	if ch < 0x300 {
		return indicNone
	}
	if unicode.Is(unicodeAliasIndic_Conjunct_Break_Consonant, ch) {
		return indicConsonant
	}
	if unicode.Is(unicodeAliasIndic_Conjunct_Break_Linker, ch) {
		return indicLinker
	}
	if unicode.Is(unicodeAliasIndic_Conjunct_Break_Extend, ch) {
		return indicExtend
	}
	return indicNone
}

func graphemePropertiesFor(ch rune) graphemeProperties {
	if ch <= unicode.MaxASCII {
		return graphemeProperties{breakClass: graphemeClass(ch)}
	}
	return graphemeProperties{
		breakClass:           graphemeClass(ch),
		indicClass:           indicClass(ch),
		extendedPictographic: ch >= 0xA9 && unicode.Is(unicodeAliasExtended_Pictographic, ch),
	}
}

func graphemeClass(ch rune) graphemeBreakClass {
	if ch <= unicode.MaxASCII {
		switch ch {
		case '\r':
			return graphemeCR
		case '\n':
			return graphemeLF
		default:
			if ch < ' ' || ch == 0x7F {
				return graphemeControl
			}
			return graphemeOther
		}
	}
	if ch == 0x200D {
		return graphemeZWJ
	}
	if ch >= 0x1F1E6 && ch <= 0x1F1FF {
		return graphemeRegionalIndicator
	}
	if ch >= 0xAC00 && ch <= 0xD7A3 {
		if (ch-0xAC00)%28 == 0 {
			return graphemeLV
		}
		return graphemeLVT
	}
	if ch >= 0x1100 && ch <= 0x115F || ch >= 0xA960 && ch <= 0xA97C {
		return graphemeL
	}
	if ch >= 0x1160 && ch <= 0x11A7 || ch >= 0xD7B0 && ch <= 0xD7C6 {
		return graphemeV
	}
	if ch >= 0x11A8 && ch <= 0x11FF || ch >= 0xD7CB && ch <= 0xD7FB {
		return graphemeT
	}
	if unicode.Is(unicodeAliasGrapheme_Cluster_Break_Control, ch) {
		return graphemeControl
	}
	if unicode.Is(unicodeAliasGrapheme_Cluster_Break_Extend, ch) {
		return graphemeExtend
	}
	if unicode.Is(unicodeAliasGrapheme_Cluster_Break_Prepend, ch) {
		return graphemePrepend
	}
	if unicode.Is(unicodeAliasGrapheme_Cluster_Break_SpacingMark, ch) {
		return graphemeSpacingMark
	}
	return graphemeOther
}
