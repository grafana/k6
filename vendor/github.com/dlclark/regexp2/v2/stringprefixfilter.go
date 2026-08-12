package regexp2

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/dlclark/regexp2/v2/helpers"
	"github.com/dlclark/regexp2/v2/syntax"
)

const maxStringFilterLiteralLen = 8

var (
	errStringStartAtTooLarge        = errors.New("startAt must be less than the length of the input string")
	errStringStartAtNotRuneBoundary = errors.New("startAt must align to the start of a valid rune in the input string")
)

// StringPrefixFilter optionally searches string input before the engine decodes it
// to runes. It returns a byte index for a candidate match start, or ok=false if
// the regex cannot match. The filter must be conservative: false positives are
// allowed, false negatives are not.
type StringPrefixFilter func(input string, startAt int) (candidateByteIndex int, ok bool)

func newStringPrefixFilter(code *syntax.Code) StringPrefixFilter {
	if code == nil || code.RightToLeft || code.FindOptimizations == nil {
		return nil
	}

	opts := code.FindOptimizations
	minRequiredLength := opts.MinRequiredLength

	switch opts.FindMode {
	case syntax.LeadingString_LeftToRight:
		return stringIndexPrefixFilter(opts.LeadingPrefix, false, minRequiredLength)
	case syntax.LeadingString_OrdinalIgnoreCase_LeftToRight:
		return stringIndexPrefixFilter(opts.LeadingPrefix, true, minRequiredLength)
	case syntax.LeadingStrings_LeftToRight:
		return stringIndexPrefixesFilter(opts.LeadingPrefixes, false, minRequiredLength)
	case syntax.LeadingStrings_OrdinalIgnoreCase_LeftToRight:
		return stringIndexPrefixesFilter(opts.LeadingPrefixes, true, minRequiredLength)
	case syntax.LeadingSet_LeftToRight:
		if len(opts.FixedDistanceSets) == 0 {
			return nil
		}
		set := opts.FixedDistanceSets[0]
		if set.Range == nil && (len(set.Chars) == 0 || len(set.Chars) > 5) {
			return nil
		}
		return stringFixedDistanceSetFilter(set, minRequiredLength)
	case syntax.FixedDistanceChar_LeftToRight:
		return stringFixedDistanceCharFilter(opts.FixedDistanceLiteral.C, opts.FixedDistanceLiteral.Distance, minRequiredLength)
	case syntax.FixedDistanceString_LeftToRight:
		return stringFixedDistanceStringFilter(opts.FixedDistanceLiteral.S, opts.FixedDistanceLiteral.Distance, minRequiredLength)
	case syntax.LiteralAfterLoop_LeftToRight:
		return stringLiteralAfterLoopFilter(opts.LiteralAfterLoop, minRequiredLength)
	default:
		return nil
	}
}

type asciiSetStringScanner struct {
	chars    string
	first    byte
	last     byte
	useRange bool
	distance int
}

func newASCIISetStringScanner(set syntax.FixedDistanceSet) (asciiSetStringScanner, bool) {
	if set.Negated || set.Distance < 0 {
		return asciiSetStringScanner{}, false
	}
	if set.Range != nil {
		if set.Range.First < 0 || set.Range.Last > utf8.RuneSelf-1 {
			return asciiSetStringScanner{}, false
		}
		return asciiSetStringScanner{
			first:    byte(set.Range.First),
			last:     byte(set.Range.Last),
			useRange: true,
			distance: set.Distance,
		}, true
	}
	if len(set.Chars) == 0 {
		return asciiSetStringScanner{}, false
	}
	chars := make([]byte, len(set.Chars))
	for i, ch := range set.Chars {
		if ch < 0 || ch > utf8.RuneSelf-1 {
			return asciiSetStringScanner{}, false
		}
		chars[i] = byte(ch)
	}
	return asciiSetStringScanner{chars: string(chars), distance: set.Distance}, true
}

func stringFixedDistanceSetFilter(set syntax.FixedDistanceSet, minRequiredLength int) StringPrefixFilter {
	scanner, ok := newASCIISetStringScanner(set)
	if !ok {
		return nil
	}

	return func(input string, startAt int) (candidateByteIndex int, ok bool) {
		if !hasMinRequiredBytes(input, startAt, minRequiredLength) {
			return 0, false
		}

		for searchAt := startAt; searchAt < len(input); {
			offset := scanner.index(input[searchAt:])
			if offset < 0 {
				return 0, false
			}
			setByteIndex := searchAt + offset
			candidateByteIndex, valid := stringFixedDistanceCandidateStart(input, startAt, setByteIndex, scanner.distance)
			if valid && hasMinRequiredBytes(input, candidateByteIndex, minRequiredLength) {
				return candidateByteIndex, true
			}
			if valid {
				return 0, false
			}
			searchAt = setByteIndex + 1
		}
		return 0, false
	}
}

func (s asciiSetStringScanner) index(input string) int {
	if !s.useRange {
		if len(s.chars) == 1 {
			return strings.IndexByte(input, s.chars[0])
		}
		return strings.IndexAny(input, s.chars)
	}
	for i := 0; i < len(input); i++ {
		if input[i] >= s.first && input[i] <= s.last {
			return i
		}
	}
	return -1
}

func stringIndexPrefixFilter(prefix string, ignoreCase bool, minRequiredLength int) StringPrefixFilter {
	if prefix == "" {
		return nil
	}
	if ignoreCase && !isASCIIString(prefix) {
		return nil
	}

	return func(input string, startAt int) (candidateByteIndex int, ok bool) {
		if !hasMinRequiredBytes(input, startAt, minRequiredLength) {
			return 0, false
		}

		var offset int
		if ignoreCase {
			offset = helpers.IndexStringIgnoreCaseASCII(input[startAt:], prefix)
		} else {
			offset = strings.Index(input[startAt:], prefix)
		}
		if offset < 0 {
			return 0, false
		}
		return startAt + offset, true
	}
}

func stringIndexPrefixesFilter(prefixes []string, ignoreCase bool, minRequiredLength int) StringPrefixFilter {
	if len(prefixes) == 0 {
		return nil
	}
	if ignoreCase {
		for _, prefix := range prefixes {
			if !isASCIIString(prefix) {
				return nil
			}
		}
	}

	if filter, ok := compileASCIIStringSetPrefixFilter(prefixes, ignoreCase, minRequiredLength); ok {
		return filter.index
	}

	return func(input string, startAt int) (candidateByteIndex int, ok bool) {
		return indexAnyPrefixFallback(input, startAt, prefixes, ignoreCase, minRequiredLength)
	}
}

func indexAnyPrefixFallback(input string, startAt int, prefixes []string, ignoreCase bool, minRequiredLength int) (candidateByteIndex int, ok bool) {
	if !hasMinRequiredBytes(input, startAt, minRequiredLength) {
		return 0, false
	}

	best := -1
	remaining := input[startAt:]
	for _, prefix := range prefixes {
		var offset int
		if ignoreCase {
			offset = helpers.IndexStringIgnoreCaseASCII(remaining, prefix)
		} else {
			offset = strings.Index(remaining, prefix)
		}
		if offset >= 0 && (best < 0 || offset < best) {
			best = offset
		}
	}
	if best < 0 {
		return 0, false
	}
	return startAt + best, true
}

type asciiStringSetPrefixFilter struct {
	firstChars       string
	prefixesByFirst  [256][]string
	minRequiredBytes int
}

// compileASCIIStringSetPrefixFilter builds a byte-oriented multi-prefix scanner
// for the narrow shape where it beats running strings.Index once per prefix:
// case-sensitive ASCII prefixes with at least two prefixes sharing a first byte.
// It indexes possible first bytes with strings.IndexAny, then verifies only the
// bucket for the byte found. Other shapes fall back to the old implementation.
func compileASCIIStringSetPrefixFilter(prefixes []string, ignoreCase bool, minRequiredLength int) (*asciiStringSetPrefixFilter, bool) {
	if ignoreCase {
		return nil, false
	}

	filter := &asciiStringSetPrefixFilter{
		minRequiredBytes: minRequiredLength,
	}
	var firstChars [256]bool
	var hasSharedFirst bool
	for _, prefix := range prefixes {
		if prefix == "" || !isASCIIString(prefix) {
			return nil, false
		}

		first := prefix[0]
		filter.prefixesByFirst[first] = append(filter.prefixesByFirst[first], prefix)
		if len(filter.prefixesByFirst[first]) > 1 {
			hasSharedFirst = true
		}
		firstChars[first] = true
	}

	if !hasSharedFirst {
		return nil, false
	}

	firstBytes := make([]byte, 0, len(prefixes)*2)
	for i, ok := range firstChars {
		if ok {
			firstBytes = append(firstBytes, byte(i))
		}
	}
	if len(firstBytes) == 0 {
		return nil, false
	}
	filter.firstChars = string(firstBytes)
	return filter, true
}

func (f *asciiStringSetPrefixFilter) index(input string, startAt int) (candidateByteIndex int, ok bool) {
	if !hasMinRequiredBytes(input, startAt, f.minRequiredBytes) {
		return 0, false
	}

	for searchAt := startAt; searchAt < len(input); {
		offset := strings.IndexAny(input[searchAt:], f.firstChars)
		if offset < 0 {
			return 0, false
		}
		i := searchAt + offset
		first := input[i]
		for _, prefix := range f.prefixesByFirst[first] {
			if len(input)-i >= len(prefix) && strings.HasPrefix(input[i:], prefix) {
				return i, true
			}
		}
		searchAt = i + 1
	}
	return 0, false
}

func stringFixedDistanceCharFilter(ch rune, distance, minRequiredLength int) StringPrefixFilter {
	if distance < 0 {
		return nil
	}

	return func(input string, startAt int) (candidateByteIndex int, ok bool) {
		if !hasMinRequiredBytes(input, startAt, minRequiredLength) {
			return 0, false
		}

		searchAt := startAt
		for {
			offset := strings.IndexRune(input[searchAt:], ch)
			if offset < 0 {
				return 0, false
			}
			byteIndex := searchAt + offset
			candidateByteIndex, ok := stringFixedDistanceCandidateStart(input, startAt, byteIndex, distance)
			if ok && hasMinRequiredBytes(input, candidateByteIndex, minRequiredLength) {
				return candidateByteIndex, true
			}
			if ok {
				return 0, false
			}
			_, size := utf8.DecodeRuneInString(input[byteIndex:])
			if size == 0 {
				return 0, false
			}
			searchAt = byteIndex + size
		}
	}
}

func stringFixedDistanceStringFilter(literal string, distance, minRequiredLength int) StringPrefixFilter {
	if literal == "" || distance < 0 || len(literal) > maxStringFilterLiteralLen {
		return nil
	}

	return func(input string, startAt int) (candidateByteIndex int, ok bool) {
		if !hasMinRequiredBytes(input, startAt, minRequiredLength) {
			return 0, false
		}

		searchAt := startAt
		for searchAt <= len(input)-len(literal) {
			offset := strings.Index(input[searchAt:], literal)
			if offset < 0 {
				return 0, false
			}
			literalIndex := searchAt + offset
			candidateByteIndex, ok := stringFixedDistanceCandidateStart(input, startAt, literalIndex, distance)
			if ok && hasMinRequiredBytes(input, candidateByteIndex, minRequiredLength) {
				return candidateByteIndex, true
			}
			if ok {
				return 0, false
			}
			searchAt = literalIndex + 1
		}
		return 0, false
	}
}

func stringLiteralAfterLoopFilter(literal *syntax.LiteralAfterLoop, minRequiredLength int) StringPrefixFilter {
	if literal == nil || literal.LoopNode == nil || literal.LoopNode.Set == nil {
		return nil
	}
	if literal.StringIgnoreCase && (literal.String == "" || !isASCIIString(literal.String)) {
		return nil
	}

	return func(input string, startAt int) (candidateByteIndex int, ok bool) {
		if !hasMinRequiredBytes(input, startAt, minRequiredLength) {
			return 0, false
		}
		if !stringHasLiteralAfterLoop(input, startAt, literal) {
			return 0, false
		}
		return startAt, true
	}
}

func stringHasLiteralAfterLoop(input string, searchAt int, literal *syntax.LiteralAfterLoop) bool {
	switch {
	case literal.String != "":
		if literal.StringIgnoreCase {
			return helpers.IndexStringIgnoreCaseASCII(input[searchAt:], literal.String) >= 0
		}
		return strings.Contains(input[searchAt:], literal.String)
	case len(literal.Chars) > 0:
		needle := string(literal.Chars)
		return strings.ContainsAny(input[searchAt:], needle)
	default:
		return strings.ContainsRune(input[searchAt:], literal.Char)
	}
}

func stringFixedDistanceCandidateStart(input string, startAt, byteIndex, distance int) (int, bool) {
	candidateByteIndex := byteIndex
	for i := 0; i < distance; i++ {
		if candidateByteIndex <= startAt {
			return 0, false
		}
		_, size := utf8.DecodeLastRuneInString(input[:candidateByteIndex])
		if size == 0 {
			return 0, false
		}
		candidateByteIndex -= size
	}
	return candidateByteIndex, true
}

func (re *Regexp) findStringPrefixCandidate(input string, startAt int) (candidateByteIndex int, ok bool) {
	if re.stringPrefixFilter == nil || re.RightToLeft() {
		return startAt, true
	}
	candidateByteIndex, ok = re.stringPrefixFilter(input, startAt)
	if !ok {
		return 0, false
	}
	if candidateByteIndex < startAt || candidateByteIndex > len(input) || !isStringRuneBoundary(input, candidateByteIndex) {
		return startAt, true
	}
	return candidateByteIndex, true
}

func (re *Regexp) findStringMatchStart(input string, startAt int) (candidateByteIndex int, ok bool, err error) {
	if startAt > len(input) {
		return 0, false, errStringStartAtTooLarge
	}
	if startAt >= 0 && !isStringRuneBoundary(input, startAt) {
		return 0, false, errStringStartAtNotRuneBoundary
	}

	if startAt < 0 {
		if re.RightToLeft() {
			startAt = len(input)
		} else {
			startAt = 0
		}
	}

	candidateByteIndex, ok = re.findStringPrefixCandidate(input, startAt)
	return candidateByteIndex, ok, nil
}

func hasMinRequiredBytes(input string, startAt, minRequiredLength int) bool {
	if startAt < 0 || startAt > len(input) {
		return false
	}
	return minRequiredLength <= 0 || len(input)-startAt >= minRequiredLength
}

func isStringRuneBoundary(s string, index int) bool {
	if index == 0 || index == len(s) {
		return true
	}
	if index < 0 || index > len(s) {
		return false
	}
	for strIdx := range s {
		if strIdx == index {
			return true
		}
		if strIdx > index {
			return false
		}
	}
	return false
}

func isASCIIString(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}
