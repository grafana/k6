package regexp2

// equalASCIIPrefixSearch searches several equal-length prefixes in parallel
// using one bit per character. Equal lengths ensure that the first completed
// prefix also has the earliest start. The masks are immutable after compilation
// and can be shared by concurrent runners.
type equalASCIIPrefixSearch struct {
	masks             [128]uint64
	starts            uint64
	ends              uint64
	width             int
	sharedFirst       bool
	minRuneCandidates uint8
}

func compileEqualASCIIPrefixSearch(prefixes []string) *equalASCIIPrefixSearch {
	if len(prefixes) < 2 || len(prefixes[0]) == 0 {
		return nil
	}
	width := len(prefixes[0])
	if width > 64/len(prefixes) {
		return nil
	}
	for _, prefix := range prefixes {
		if len(prefix) != width || !isASCIIString(prefix) {
			return nil
		}
	}
	search := &equalASCIIPrefixSearch{width: width, minRuneCandidates: prefixSearchSampleSize / 8}
	// Checking more alternatives at each candidate makes a combined search
	// worthwhile at a lower density. Two-prefix searches retain a higher bar.
	if len(prefixes) >= 3 {
		search.minRuneCandidates = prefixSearchSampleSize / 16
	}
	var firstSeen [128]bool
	for k, prefix := range prefixes {
		for i := range len(prefix) {
			ch := prefix[i]
			search.masks[ch] |= uint64(1) << (k*width + i)
		}
		search.starts |= uint64(1) << (k * width)
		search.ends |= uint64(1) << (k*width + width - 1)
		first := prefix[0]
		search.sharedFirst = search.sharedFirst || firstSeen[first]
		firstSeen[first] = true
	}
	return search
}

const prefixSearchSampleSize = 64

// Retain the existing first-character scanners when candidates are sparse.
// Sampling is bounded, and short inputs retain their cheaper existing search.
func (s *equalASCIIPrefixSearch) shouldUseRunes(input []rune, startAt int) bool {
	if startAt < 0 || startAt > len(input)-prefixSearchSampleSize {
		return false
	}
	candidates := 0
	for _, ch := range input[startAt : startAt+prefixSearchSampleSize] {
		if uint32(ch) < 128 && s.masks[ch]&s.starts != 0 {
			candidates++
		}
	}
	return candidates >= int(s.minRuneCandidates)
}

func (s *equalASCIIPrefixSearch) shouldUseString(input string, startAt int) bool {
	// Distinct first bytes already use strings.Index per prefix, which is
	// especially effective for a small number of alternatives on sparse input.
	if !s.sharedFirst || startAt < 0 || startAt > len(input)-prefixSearchSampleSize {
		return false
	}
	candidates := 0
	for i := startAt; i < startAt+prefixSearchSampleSize; i++ {
		ch := input[i]
		if ch < 128 && s.masks[ch]&s.starts != 0 {
			candidates++
		}
	}
	return candidates >= prefixSearchSampleSize/4
}

// indexRunes returns an absolute rune index, or -1. A non-ASCII rune (including
// invalid rune values accepted by rune APIs) cannot occur in these prefixes and
// therefore resets all partial matches.
func (s *equalASCIIPrefixSearch) indexRunes(input []rune, startAt int) int {
	if startAt < 0 || startAt > len(input)-s.width {
		return -1
	}
	var state uint64
	for i := startAt; i < len(input); i++ {
		ch := input[i]
		if uint32(ch) >= 128 {
			state = 0
			continue
		}
		state = ((state << 1) | s.starts) & s.masks[ch]
		if state&s.ends != 0 {
			return i - s.width + 1
		}
	}
	return -1
}

// indexString returns an absolute byte index, or -1. ASCII prefixes can only
// start at rune boundaries, even in strings containing invalid UTF-8 bytes.
func (s *equalASCIIPrefixSearch) indexString(input string, startAt int) int {
	if startAt < 0 || startAt > len(input)-s.width {
		return -1
	}
	var state uint64
	for i := startAt; i < len(input); i++ {
		ch := input[i]
		if ch >= 128 {
			state = 0
			continue
		}
		state = ((state << 1) | s.starts) & s.masks[ch]
		if state&s.ends != 0 {
			return i - s.width + 1
		}
	}
	return -1
}
