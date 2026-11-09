package regexp2

import "errors"

// MatchView provides the capture positions of a match during FindRunesMatches.
// It borrows the engine's storage and is valid only until the callback returns.
// Copy the values returned by GroupRange to retain positions; copying a
// MatchView does not extend its lifetime. The zero value has no groups.
type MatchView struct {
	match *Match
}

// GroupCount returns the number of groups defined by the pattern, including
// group 0 (the whole match) and groups that did not participate in this match.
// Explicit group numbers may be sparse; call Regexp.GetGroupNumbers before
// iteration to enumerate them rather than assuming they run from 0 to
// GroupCount()-1.
func (v MatchView) GroupCount() int {
	if v.match == nil {
		return 0
	}
	return len(v.match.matchcount)
}

// GroupRange returns the rune index and rune length of the last capture of the
// group with the given number. Indexes are relative to the original input,
// including when FindRunesMatches starts at a nonzero position. Group 0 is the
// whole match. It returns (0, 0, false) if the number does not exist or the group
// has no remaining capture; a participating empty capture has ok == true.
// For a successful result, input[index:index+length] is the captured text.
func (v MatchView) GroupRange(num int) (index, length int, ok bool) {
	m := v.match
	if m == nil {
		return 0, 0, false
	}
	if m.sparseCaps != nil {
		var exists bool
		num, exists = m.sparseCaps[num]
		if !exists {
			return 0, 0, false
		}
	}
	if num < 0 || num >= len(m.matchcount) || !m.isMatched(num) {
		return 0, 0, false
	}
	// These helpers resolve balancing references without compacting the entire
	// capture history. The interpreter and generated engines use them too.
	return m.matchIndex(num), m.matchLength(num), true
}

// FindRunesMatches calls yield for each successive match in input, starting at
// the rune index startAt. A negative startAt starts at the beginning of input,
// or at its end for RightToLeft patterns. A nil yield is a no-op; otherwise,
// a startAt beyond len(input) is an error.
// Match order and advancement after empty matches are the same as with
// FindRunesMatchStartingAt followed by FindNextMatch.
//
// The view is valid only during the callback. Copy out any positions to retain
// them, and do not modify input during iteration. Returning false stops the
// iteration successfully. Matching errors are returned after any preceding
// callbacks have completed. MatchTimeout applies to each search for the next
// match, not to the whole iteration or the callback.
//
// Matching reuses the runner and capture storage instead of allocating a Match
// and text metadata for every result. Initial setup and growing the engine's
// buffers may allocate; GroupRange and GroupCount do not allocate.
func (re *Regexp) FindRunesMatches(input []rune, startAt int, yield func(MatchView) bool) error {
	if yield == nil {
		return nil
	}
	if startAt > len(input) {
		return errors.New("regexp2: startAt must not exceed the input length")
	}
	if startAt < 0 {
		startAt = 0
		if re.RightToLeft() {
			startAt = len(input)
		}
	}

	runner := re.getRunner()
	defer re.putRunner(runner)
	previousMatchLength := -1
	for {
		m, err := runner.scan(input, nil, startAt, startAt, previousMatchLength, scanCaptures, re.MatchTimeout)
		if err != nil || m == nil {
			return err
		}
		if !yield(MatchView{match: m}) {
			return nil
		}
		startAt = m.textpos
		previousMatchLength = m.RuneLength
	}
}
