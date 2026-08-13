package syntax

import (
	"bytes"
	"cmp"
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
	"unsafe"
)

// Computes a character class for the first character in tree.  This uses a more robust algorithm
// than is used by TryFindFixedLiterals and thus can find starting sets it couldn't.  For example,
// fixed literals won't find the starting set for a*b, as the a isn't guaranteed and the b is at a
// variable position, but this will find [ab] as it's instead looking for anything that under any
// circumstance could possibly start a match.
func findFirstCharClass(root *RegexNode) *CharSet {
	// Explore the graph, adding found chars into a result set, which is lazily initialized so that
	// we can initialize it to a parsed set if we discover one first (this is helpful not just for allocation
	// but because it enables supporting starting negated sets, which wouldn't work if they had to be merged
	// into a non-negated default set). If the operation returns true, we successfully explore all relevant nodes
	// in the graph.  If it returns false, we were unable to successfully explore all relevant nodes, typically
	// due to conflicts when trying to add characters into the result set, e.g. we may have read a negated set
	// and were then unable to merge into that a subsequent non-negated set.  If it returns null, it means the
	// whole pattern was nullable such that it could match an empty string, in which case we
	// can't make any statements about what begins a match.
	var cc *CharSet
	if tryFindFirstCharClass(root, &cc) == 1 {
		return cc
	}
	return nil
}

// Walks the nodes of the expression looking for any node that could possibly match the first
// character of a match, e.g. in `a*b*c+d`, we'd find [abc], or in `(abc|d*e)?[fgh]`, we'd find
// [adefgh].  The function is called for each node, recurring into children where appropriate,
// and returns:
//   - 1 if the child was successfully processed and represents a stopping point, e.g. a single
//     char loop with a minimum bound greater than 0 such that nothing after that node in a
//     concatenation could possibly match the first character.
//   - 0 if the child failed to be processed but needed to be, such that the results can't
//     be trusted.  If any node returns false, the whole operation fails.
//   - -1 if the child was successfully processed but doesn't represent a stopping point, i.e.
//     it's zero-width (e.g. empty, a lookaround, an anchor, etc.) or it could be zero-width
//     (e.g. a loop with a min bound of 0).  A concatenation processing a child that returns
//     -1 needs to keep processing the next child.
func tryFindFirstCharClass(node *RegexNode, ccIn **CharSet) int {
	cc := *ccIn

	switch node.T {
	// Base cases where we have results to add to the result set. Add the values into the result set, if possible.
	// If this is a loop and it has a lower bound of 0, then it's zero-width, so return null.
	case NtOne, NtOneloop, NtOnelazy, NtOneloopatomic:
		if cc == nil {
			cc = &CharSet{}
			*ccIn = cc
		}
		if cc.IsMergeable() {
			cc.addChar(node.Ch)
			if node.T == NtOne || node.M > 0 {
				return 1
			}
			return -1
		}
		return 0

	case NtNotone, NtNotoneloop, NtNotonelazy, NtNotoneloopatomic:
		if cc == nil {
			cc = &CharSet{}
			*ccIn = cc
		}
		if cc.IsMergeable() {
			cc.addChar(node.Ch)
			cc.negate = true
			/*if node.Ch > 0 {
				// Add the range before the excluded char.
				cc.addRange(0, (node.Ch - 1))
			}
			if node.Ch < unicode.MaxRune {
				// Add the range after the excluded char.
				cc.addRange(node.Ch+1, unicode.MaxRune)
			}*/
			if node.T == NtNotone || node.M > 0 {
				return 1
			}
			return -1
		}
		return 0

	case NtSet, NtSetloop, NtSetlazy, NtSetloopatomic:
		{
			setSuccess := false
			if cc == nil {
				cp := node.Set.Copy()
				*ccIn = &cp
				setSuccess = true
			} else if cc.IsMergeable() && node.Set.IsMergeable() {
				cc.addSet(*node.Set)
				setSuccess = true
			}
			if !setSuccess {
				return 0
			} else if node.T == NtSet || node.M > 0 {
				return 1
			}
			return -1
		}

	case NtMulti:
		if cc == nil {
			cc = &CharSet{}
			*ccIn = cc
		}
		if cc.IsMergeable() {
			if node.Options&RightToLeft != 0 {
				cc.addChar(node.Str[len(node.Str)-1])
			} else {
				cc.addChar(node.Str[0])
			}
			return 1
		}
		return 0

	// Zero-width elements.  These don't contribute to the starting set, so return null to indicate a caller
	// should keep looking past them.
	case NtEmpty, NtNothing, NtBol, NtEol, NtBoundary, NtNonboundary, NtECMABoundary, NtNonECMABoundary,
		NtBeginning, NtStart, NtEndZ, NtEnd, NtUpdateBumpalong, NtPosLook, NtNegLook:
		return -1

	// Groups.  These don't contribute anything of their own, and are just pass-throughs to their children.
	case NtAtomic, NtCapture:
		return tryFindFirstCharClass(node.Children[0], ccIn)

	// Loops.  Like groups, these are mostly pass-through: if the child fails, then the whole operation needs
	// to fail, and if the child is nullable, then the loop is as well.  However, if the child succeeds but
	// the loop has a lower bound of 0, then the loop is still nullable.
	case NtLoop, NtLazyloop:
		val := tryFindFirstCharClass(node.Children[0], ccIn)
		if val <= 0 || node.M != 0 {
			return val
		}
		return -1

	// Concatenation.  Loop through the children as long as they're nullable.  The moment a child returns true,
	// we don't need or want to look further, as that child represents non-zero-width and nothing beyond it can
	// contribute to the starting character set.  The moment a child returns false, we need to fail the whole thing.
	// If every child is nullable, then the concatenation is also nullable.
	case NtConcatenate:
		for i := 0; i < len(node.Children); i++ {
			childResult := tryFindFirstCharClass(node.Children[i], ccIn)
			if childResult != -1 {
				return childResult
			}
		}
		return -1

	// Alternation. Every child is its own fork/branch and contributes to the starting set.  As with concatenation,
	// the moment any child fails, fail.  And if any child is nullable, the alternation is also nullable (since that
	// zero-width path could be taken).  Otherwise, if every branch returns true, so too does the alternation.
	case NtAlternate:
		anyChildWasNull := false
		for i := 0; i < len(node.Children); i++ {
			childResult := tryFindFirstCharClass(node.Children[i], ccIn)
			switch childResult {
			case -1:
				anyChildWasNull = true
			case 0:
				return 0
			}
		}
		if anyChildWasNull {
			return -1
		}
		return 1

	// Conditionals.  Just like alternation for their "yes"/"no" child branches.  If either returns false, return false.
	// If either is nullable, this is nullable. If both return true, return true.
	case NtBackRefCond, NtExprCond:
		branchStart := 1
		if node.T == NtBackRefCond {
			branchStart = 0
		}
		// conditional without all the branches
		if branchStart+1 >= len(node.Children) {
			return -1
		}
		start := tryFindFirstCharClass(node.Children[branchStart], ccIn)
		next := tryFindFirstCharClass(node.Children[branchStart+1], ccIn)
		if start == -1 || next == -1 {
			return -1
		}
		if start == 0 || next == 0 {
			return 0
		}
		return 1

	// Backreferences.  We can't easily make any claims about what content they might match, so just give up.
	case NtRef:
		return 0
	}

	// Unknown node.
	return 0
}

// Computes the leading ordinal case-insensitive substring in node
func findPrefixOrdinalCaseInsensitive(node *RegexNode) string {
	for {
		// Search down the left side of the tree looking for a concatenation.  If we find one,
		// ask it for any ordinal case-insensitive prefix it has.
		switch node.T {
		case NtLoop, NtLazyloop:
			if node.M <= 0 {
				return ""
			}
			fallthrough
		case NtAtomic, NtCapture:
			node = node.Children[0]
			continue

		case NtConcatenate:
			_, _, caseInsensitiveString := node.TryGetOrdinalCaseInsensitiveString(0, len(node.Children), true)
			return caseInsensitiveString

		default:
			return ""
		}
	}
}

// Computes the leading substring in node may be empty.
func findPrefix(node *RegexNode) string {
	vsb := &bytes.Buffer{}
	tryFindPrefix(node, vsb)
	return vsb.String()
}

// Processes the node, adding any prefix text to the builder.
// Returns whether processing should continue with subsequent nodes.
func tryFindPrefix(node *RegexNode, vsb *bytes.Buffer) bool {
	// We don't bother to handle reversed input, so process at most one node
	// when handling RightToLeft.
	rtl := (node.Options & RightToLeft) != 0

	switch node.T {
	// Concatenation
	case NtConcatenate:
		for i := 0; i < len(node.Children); i++ {
			if !tryFindPrefix(node.Children[i], vsb) {
				return false
			}
		}
		return !rtl
	// Alternation: find a string that's a shared prefix of all branches
	case NtAlternate:
		// for RTL we'd need to be matching the suffixes of the alternation cases
		if rtl {
			return false
		}

		// Store the initial branch into the target builder, keeping track
		// of how much was appended. Any of this contents that doesn't overlap
		// will every other branch will be removed before returning.
		initialLength := vsb.Len()
		tryFindPrefix(node.Children[0], vsb)
		addedLength := vsb.Len() - initialLength

		// Then explore the rest of the branches, finding the length
		// of prefix they all share in common with the initial branch.
		if addedLength != 0 {
			alternateSb := &bytes.Buffer{}
			vsbSlice := vsb.Bytes()[initialLength : initialLength+addedLength]

			// Process each branch.  If we reach a point where we've proven there's
			// no overlap, we can bail early.
			for i := 1; i < len(node.Children) && addedLength != 0; i++ {
				alternateSb.Reset()

				// Process the branch into a temporary builder.
				tryFindPrefix(node.Children[i], alternateSb)

				// Find how much overlap there is between this branch's prefix
				// and the smallest amount of prefix that overlapped with all
				// the previously seen branches.
				addedLength = commonPrefixLen(vsbSlice, alternateSb.Bytes())
			}

			// Then cull back on what was added based on the other branches.
			vsb.Truncate(initialLength + addedLength)
		}

		// Don't explore anything after the alternation.  We could make this work if desirable,
		// but it's currently not worth the extra complication.  The entire contents of every
		// branch would need to be identical other than zero-width anchors/assertions.
		return false

	// One character
	case NtOne:
		vsb.WriteRune(node.Ch)
		return !rtl

	// Multiple characters
	case NtMulti:
		vsb.WriteString(string(node.Str))
		return !rtl

	// Loop of one character
	case NtOneloop /*NtOneloopatomic,*/, NtOnelazy:
		if node.M <= 0 {
			return false
		}
		// arbitrary cut-off to avoid creating super long strings unnecessarily
		count := 32
		if node.M < count {
			count = node.M
		}
		vsb.WriteString(strings.Repeat(string(node.Ch), count))
		return count == node.N && !rtl

	// Loop of a node
	case NtLoop, NtLazyloop:
		if node.M <= 0 {
			return false
		}

		// arbitrary cut-off to avoid creating super long strings unnecessarily
		limit := 4
		if node.M < limit {
			limit = node.M
		}
		for i := 0; i < limit; i++ {
			if tryFindPrefix(node.Children[0], vsb) {
				return false
			}
		}
		return limit == node.N && !rtl

	// Grouping nodes for which we only care about their single child
	case NtAtomic, NtCapture:
		return tryFindPrefix(node.Children[0], vsb)

	// Zero-width anchors and assertions
	case NtBol, NtEol, NtBoundary, NtECMABoundary, NtNonboundary, NtNonECMABoundary, NtBeginning,
		NtStart, NtEndZ, NtEnd, NtEmpty, NtUpdateBumpalong, NtPosLook, NtNegLook:
		return true
	}
	// Give up for anything else
	return false
}

// commonPrefixLen returns the length of the common prefix of two strings.
func commonPrefixLen(a, b []byte) int {
	commonLen := min(len(a), len(b))
	i := 0
	// Optimization: load and compare word-sized chunks at a time.
	// This is about 6x faster than the naive approach when len > 64.
	//
	// TODO(adonovan): further optimizations are possible,
	// at the cost of portability, for example by:
	// - better elimination of bounds checks;
	// - use of uint64 instead of an array may result in better
	//   registerization, and allows computing the final portion
	//   from the bitmask:
	//
	// 	cmp := load64le(a, i) ^ load64le(b, i)
	// 	if cmp != 0 {
	// 		return i + bits.LeadingZeros64(cmp)/8
	// 	}
	//
	// - use of vector instructions in the manner of
	//   runtime.cmpstring, which is expected to achieve 3x
	//   further improvement when len > 32.
	//
	const wordsize = int(unsafe.Sizeof(uint(0)))
	var aword, bword [wordsize]byte
	for i+wordsize <= commonLen {
		copy(aword[:], a[i:i+wordsize])
		copy(bword[:], b[i:i+wordsize])
		if aword != bword {
			break
		}
		i += wordsize
	}
	// naive implementation
	for i < commonLen {
		if a[i] != b[i] {
			return i
		}
		i++
	}
	return i
}
func min(x, y int) int {
	if x < y {
		return x
	} else {
		return y
	}
}
func max(x, y int) int {
	if x > y {
		return x
	} else {
		return y
	}
}

// Minimum string length for prefixes to be useful. If any prefix has length 1,
// then we're generally better off just using IndexOfAny with chars.
const minPrefixLength = 2

// Arbitrary string length limit (with some wiggle room) to avoid creating strings that are longer than is useful and consuming too much memory.
const maxPrefixLength = 8

// Arbitrary limit on the number of prefixes to find. If we find more than this, we're likely to be spending too much time finding prefixes that won't be useful.
const maxPrefixes = 16

// Finds an array of multiple prefixes that a node can begin with.
// If a fixed set of prefixes is found, such that a match for this node is guaranteed to begin
// with one of those prefixes, an array of those prefixes is returned.  Otherwise, null.
func findPrefixes(node *RegexNode, ignoreCase bool) []string {

	// Analyze the node to find prefixes.
	results := []*bytes.Buffer{{}}
	findPrefixesCore(node, &results, ignoreCase)

	// If we found too many prefixes or if any found is too short, fail.
	if len(results) > maxPrefixes || slices.ContainsFunc(results, func(sb *bytes.Buffer) bool { return sb.Len() < minPrefixLength }) {
		return nil
	}

	// Return the prefixes.
	resultStrings := make([]string, len(results))
	for i := 0; i < len(results); i++ {
		resultStrings[i] = results[i].String()
	}
	return resultStrings
}

// Updates the results list with found prefixes. All existing strings in the list are treated as existing
// discovered prefixes prior to the node being processed. The method returns true if subsequent nodes after
// this one should be examined, or returns false if they shouldn't be because the node wasn't guaranteed
// to be fully processed.
func findPrefixesCore(node *RegexNode, res *[]*bytes.Buffer, ignoreCase bool) bool {
	results := *res
	// If we're too deep to analyze further, we can't trust what we've already computed, so stop iterating.
	// Also bail if any of our results is already hitting the threshold, or if this node is RTL, which is
	// not worth the complexity of handling.
	// Or if we've already discovered more than the allowed number of prefixes.

	if slices.ContainsFunc(results, func(sb *bytes.Buffer) bool { return sb.Len() >= maxPrefixLength }) ||
		(node.Options&RightToLeft) != 0 ||
		len(results) > maxPrefixes {
		return false
	}

	// These limits are approximations. We'll stop trying to make strings longer once we exceed the max length,
	// and if we exceed the max number of prefixes by a non-trivial amount, we'll fail the operation.
	// limit how many chars we get from a set based on the max prefixes we care about
	//setChars := make([]rune, 0, maxPrefixes)

	// Loop down the left side of the tree, looking for a starting node we can handle. We only loop through
	// atomic and capture nodes, as the child is guaranteed to execute once, as well as loops with a positive
	// minimum and thus at least one guaranteed iteration.
	for {
		switch node.T {
		// These nodes are all guaranteed to execute at least once, so we can just
		// skip through them to their child.
		case NtAtomic, NtCapture:
			node = node.Children[0]
			continue

		// Zero-width anchors and assertions don't impact a prefix and may be skipped over.
		case NtBol, NtEol, NtBoundary, NtECMABoundary, NtNonboundary, NtNonECMABoundary,
			NtBeginning, NtStart, NtEndZ, NtEnd, NtEmpty, NtUpdateBumpalong,
			NtPosLook, NtNegLook:
			return true

		// If we hit a single character, we can just return that character.
		// This is only relevant for case-sensitive searches, as for case-insensitive we'd have sets for anything
		// that produces a different result when case-folded, or for strings composed entirely of characters that
		// don't participate in case conversion. Single character loops are handled the same as single characters
		// up to the min iteration limit. We can continue processing after them as well if they're repeaters such
		// that their min and max are the same.
		case NtOne, NtOneloop, NtOnelazy, NtOneloopatomic:
			if !ignoreCase || !participatesInCaseConversion(node.Ch) {
				reps := maxPrefixLength
				if node.T == NtOne {
					reps = 1
				} else if node.M < reps {
					reps = node.M
				}
				if reps == 1 {
					for _, sb := range results {
						sb.WriteRune(node.Ch)
					}
				} else {
					for _, sb := range results {
						sb.WriteString(strings.Repeat(string(node.Ch), reps))
					}
				}
				return node.T == NtOne || reps == node.N
			}

		// If we hit a string, we can just return that string.
		// As with One above, this is only relevant for case-sensitive searches.
		case NtMulti:
			if !ignoreCase {
				for _, sb := range results {
					sb.WriteString(string(node.Str))
				}
			} else {
				// If we're ignoring case, then only append up through characters that don't participate in case conversion.
				// If there are any beyond that, we can't go further and need to stop with what we have.
				for _, c := range node.Str {
					if participatesInCaseConversion(c) {
						return false
					}

					for _, sb := range results {
						sb.WriteRune(c)
					}
				}
			}
			return true

		// For case-sensitive,  try to extract the characters that comprise it, and if there are
		// any and there aren't more than the max number of prefixes, we can return
		// them each as a prefix. Effectively, this is an alternation of the characters
		// that comprise the set. For case-insensitive, we need the set to be two ASCII letters that case fold to the same thing.
		// As with One and loops, set loops are handled the same as sets up to the min iteration limit.
		case NtSet, NtSetloop, NtSetlazy, NtSetloopatomic:

			setChars := node.Set.GetSetChars(maxPrefixes)

			if len(setChars) == 0 {
				return false
			}

			reps := maxPrefixLength
			if node.T == NtSet {
				reps = 1
			} else if node.M < reps {
				reps = node.M
			}
			if !ignoreCase {
				for rep := 0; rep < reps; rep++ {
					existingCount := len(results)
					if existingCount*len(setChars) > maxPrefixes {
						return false
					}

					// Duplicate all of the existing strings for all of the new suffixes, other than the first.
					for _, suffix := range setChars[1:] {
						for existing := 0; existing < existingCount; existing++ {
							newSb := bytes.NewBuffer(slices.Clone(results[existing].Bytes()))
							newSb.WriteRune(suffix)
							results = append(results, newSb)
						}
					}
					*res = results

					// Then append the first suffix to all of the existing strings.
					for existing := 0; existing < existingCount; existing++ {
						results[existing].WriteRune(setChars[0])
					}
				}
			} else {
				// For ignore-case, we currently only handle the simple (but common) case of a single
				// ASCII character that case folds to the same char.
				ok, setChars := node.Set.containsAsciiIgnoreCaseCharacter()
				if !ok {
					return false
				}

				// Append it to each.
				for _, sb := range results {
					sb.WriteString(strings.Repeat(string(setChars[1]), reps))
				}
			}

			*res = results
			return node.T == NtSet || reps == node.N

		case NtConcatenate:
			for i := 0; i < len(node.Children); i++ {
				if !findPrefixesCore(node.Children[i], res, ignoreCase) {
					return false
				}
			}

			return true

		// We can append any guaranteed iterations as if they were a concatenation.
		case NtLoop, NtLazyloop:
			if node.M > 0 {
				// MaxPrefixLength here is somewhat arbitrary, as a single loop iteration could yield multiple chars
				limit := maxPrefixLength
				if node.M < limit {
					limit = node.M
				}
				for i := 0; i < limit; i++ {
					if !findPrefixesCore(node.Children[0], res, ignoreCase) {
						return false
					}
				}
				return limit == node.N
			}

		// For alternations, we need to find a prefix for every branch; if we can't compute a
		// prefix for any one branch, we can't trust the results and need to give up, since we don't
		// know if our set of prefixes is complete.
		case NtAlternate:

			// If there are more children than our maximum, just give up immediately, as we
			// won't be able to get a prefix for every branch and have it be within our max.
			if len(node.Children) > maxPrefixes {
				return false
			}

			// Build up the list of all prefixes across all branches.
			var allBranchResults []*bytes.Buffer
			alternateBranchResults := []*bytes.Buffer{{}}
			for i := 0; i < len(node.Children); i++ {
				findPrefixesCore(node.Children[i], &alternateBranchResults, ignoreCase)

				// If we now have too many results, bail.
				if len(allBranchResults)+len(alternateBranchResults) > maxPrefixes {
					return false
				}

				for _, sb := range alternateBranchResults {
					// If a branch yields an empty prefix, then none of the other branches
					// matter, e.g. if the pattern is abc(def|ghi|), then this would result
					// in prefixes abcdef, abcghi, and abc, and since abc is a prefix of both
					// abcdef and abcghi, the former two would never be used.
					if sb.Len() == 0 {
						return false
					}
				}

				if allBranchResults == nil {
					allBranchResults = alternateBranchResults
					alternateBranchResults = []*bytes.Buffer{{}}
				} else {
					allBranchResults = append(allBranchResults, alternateBranchResults...)
					alternateBranchResults = []*bytes.Buffer{{}}
				}
			}

			// At this point, we know we can successfully incorporate the alternation's results
			// into the main results.

			// If the results are currently empty (meaning a single empty StringBuilder), we can remove
			// that builder and just replace the results with the alternation's results. We would otherwise
			// be creating a dot product of every builder in the results with every branch's result, which
			// is logically the same thing.
			if len(results) == 1 && results[0].Len() == 0 {
				results = allBranchResults
			} else {
				existingCount := len(results)

				// Duplicate all of the existing strings for all of the new suffixes, other than the first.
				for i := 1; i < len(allBranchResults); i++ {
					suffix := allBranchResults[i]
					for existing := 0; existing < existingCount; existing++ {
						newSb := &bytes.Buffer{}
						newSb.Write(results[existing].Bytes())
						newSb.Write(suffix.Bytes())
						results = append(results, newSb)
					}
				}

				// Then append the first suffix to all of the existing strings.
				for existing := 0; existing < existingCount; existing++ {
					results[existing].Write(allBranchResults[0].Bytes())
				}
			}
			*res = results

			// We don't know that we fully processed every branch, so we can't iterate through what comes after this node.
			// The results were successfully updated, but return false to indicate that nothing after this node should be examined.
		}
		return false
	}
}

const maxLoopExpansion = 20 // arbitrary cut-off to avoid loops adding significant overhead to processing
const maxFixedResults = 50  // arbitrary cut-off to avoid generating lots of sets unnecessarily

// Finds sets at fixed-offsets from the beginning of the pattern/</summary>
// set "thorough" to true to spend more time finding sets (e.g. through alternations); false to do a faster analysis that's potentially more incomplete.
// Returns the array of found sets, or null if there aren't any.
func findFixedDistanceSets(root *RegexNode, thorough bool) []FixedDistanceSet {

	// Find all fixed-distance sets.
	results := []FixedDistanceSet{}
	distance := 0
	tryFindRawFixedSets(root, &results, &distance, thorough)

	// Remove any sets that match everything; they're not helpful.  (This check exists primarily to weed
	// out use of . in Singleline mode, but also filters out explicit sets like [\s\S].)
	results = slices.DeleteFunc(results, func(s FixedDistanceSet) bool { return s.Set.IsAnything() })

	// If we don't have any results, try harder to compute one for the starting character.
	// This is a more involved computation that can find things the fixed-distance investigation
	// doesn't.
	if len(results) == 0 {
		// weed out match-all, same as above
		c := findFirstCharClass(root)
		if c == nil || c.IsAnything() {
			return nil
		}

		results = append(results, FixedDistanceSet{Set: c, Distance: 0})
	}

	// For every entry, try to get the chars that make up the set, if there are few enough.
	// For any for which we couldn't get the small chars list, see if we can get other useful info.
	//scratch := make([]rune, 0, 128)
	for i := 0; i < len(results); i++ {
		result := results[i]
		result.Negated = result.Set.IsNegated()

		// prefer IndexOfAny for tiny sets of 1 or 2 elements
		if r := result.Set.GetIfNRanges(1); len(r) == 1 && r[0].Last-r[0].First > 1 {
			result.Range = &r[0]
		} else {
			scratch := result.Set.GetSetChars(128)
			if len(scratch) > 0 {
				result.Chars = scratch
			}
		}

		results[i] = result
	}

	return results
}

// Starting from the specified root node, populates results with any characters at a fixed distance
// from the node's starting position.  The function returns true if the entire contents of the node
// is at a fixed distance, in which case distance will have been updated to include the full length
// of the node.  If it returns false, the node isn't entirely fixed, in which case subsequent nodes
// shouldn't be examined and distance should no longer be trusted.  However, regardless of whether it
// returns true or false, it may have populated results, and all populated results are valid. All
// FixedDistanceSet result will only have its Set string and Distance populated; the rest is left
// to be populated by FindFixedDistanceSets after this returns.
func tryFindRawFixedSets(node *RegexNode, res *[]FixedDistanceSet, distance *int, thorough bool) bool {
	results := *res
	if node.Options&RightToLeft != 0 {
		return false
	}

	switch node.T {
	case NtOne:
		if len(results) < maxFixedResults {
			set := &CharSet{}
			set.addChar(node.Ch)
			results = append(results, FixedDistanceSet{Set: set, Distance: *distance})
			*res = results
			*distance++
			return true
		}
		return false

	case NtOnelazy, NtOneloop, NtOneloopatomic:
		if node.M > 0 {
			set := &CharSet{}
			set.addChar(node.Ch)
			minIterations := maxLoopExpansion
			if node.M < minIterations {
				minIterations = node.M
			}
			i := 0
			for ; i < minIterations && len(results) < maxFixedResults; i++ {
				results = append(results, FixedDistanceSet{Set: set, Distance: *distance})
				*distance++
			}
			*res = results
			return i == node.M && i == node.N
		}

	case NtMulti:
		i := 0
		for ; i < len(node.Str) && len(results) < maxFixedResults; i++ {
			set := &CharSet{}
			set.addChar(node.Str[i])
			results = append(results, FixedDistanceSet{Set: set, Distance: *distance})
			*distance++
		}
		*res = results
		return i == len(node.Str)

	case NtSet:
		if len(results) < maxFixedResults {
			results = append(results, FixedDistanceSet{Set: node.Set, Distance: *distance})
			*res = results
			*distance++
			return true
		}
		return false

	case NtSetlazy, NtSetloop, NtSetloopatomic:
		if node.M > 0 {
			minIterations := maxLoopExpansion
			if node.M < minIterations {
				minIterations = node.M
			}
			i := 0
			for ; i < minIterations && len(results) < maxFixedResults; i++ {
				results = append(results, FixedDistanceSet{Set: node.Set, Distance: *distance})
				*distance++
			}
			*res = results
			return i == node.M && i == node.N
		}

	case NtNotone:
		// We could create a set out of Notone, but it will be of little value in helping to improve
		// the speed of finding the first place to match, as almost every character will match it.
		*distance++
		return true

	case NtNotonelazy, NtNotoneloop, NtNotoneloopatomic:
		if node.M == node.N {
			*distance += node.M
			return true
		}
	case NtBeginning, NtBol, NtBoundary, NtECMABoundary, NtEmpty, NtEnd, NtEndZ, NtEol,
		NtNonboundary, NtNonECMABoundary, NtUpdateBumpalong,
		NtStart, NtNegLook, NtPosLook:
		// Zero-width anchors and assertions.  In theory, for PositiveLookaround and NegativeLookaround we could also
		// investigate them and use the learned knowledge to impact the generated sets, at least for lookaheads.
		// For now, we don't bother.
		return true

	case NtAtomic, NtGroup, NtCapture:
		return tryFindRawFixedSets(node.Children[0], res, distance, thorough)

	case NtLazyloop, NtLoop:
		if node.M > 0 {
			// This effectively only iterates the loop once.  If deemed valuable,
			// it could be updated in the future to duplicate the found results
			// (updated to incorporate distance from previous iterations) and
			// summed distance for all node.M iterations.  If node.M == node.N,
			// this would then also allow continued evaluation of the rest of the
			// expression after the loop.
			tryFindRawFixedSets(node.Children[0], res, distance, thorough)
			return false
		}

	case NtConcatenate:
		for i := 0; i < len(node.Children); i++ {
			if !tryFindRawFixedSets(node.Children[i], res, distance, thorough) {
				return false
			}
		}
		return true

	case NtAlternate:
		if thorough {
			allSameSize := true
			sameDistance := -1
			var combined = make(map[int]struct {
				Set   *CharSet
				Count int
			})
			var localResults []FixedDistanceSet

			for i := 0; i < len(node.Children); i++ {
				localResults = []FixedDistanceSet{}
				localDistance := 0
				allSameSize = allSameSize && tryFindRawFixedSets(node.Children[i], &localResults, &localDistance, thorough)

				if len(localResults) == 0 {
					return false
				}

				if allSameSize {
					if sameDistance == -1 {
						sameDistance = localDistance
					} else if sameDistance != localDistance {
						allSameSize = false
					}
				}

				for _, fixedSet := range localResults {
					if v, ok := combined[fixedSet.Distance]; ok {
						if v.Set.IsMergeable() && fixedSet.Set.IsMergeable() {
							v.Set.addSet(*fixedSet.Set)
							v.Count++
							combined[fixedSet.Distance] = v
						}
					} else {
						setCopy := fixedSet.Set.Copy()
						combined[fixedSet.Distance] = struct {
							Set   *CharSet
							Count int
						}{Set: &setCopy, Count: 1}
					}
				}
			}

			for k, v := range combined {
				if len(results) >= maxFixedResults {
					allSameSize = false
					break
				}

				if v.Count == len(node.Children) {
					results = append(results, FixedDistanceSet{Set: v.Set, Distance: k + *distance})
				}
			}
			*res = results

			if allSameSize {
				*distance += sameDistance
				return true
			}

			return false
		}
	}
	return false
}

// CompareFunc for a set of fixed-distance set results from best to worst quality.</summary>
func compareFixedDistanceSetsByQuality(s1, s2 FixedDistanceSet) int {
	// Finally, try to move the "best" results to be earlier.  "best" here are ones we're able to search
	// for the fastest and that have the best chance of matching as few false positives as possible.
	s1RangeLength := getRangeLength(s1.Range, s1.Negated)
	s2RangeLength := getRangeLength(s2.Range, s2.Negated)

	// If one set is negated and the other isn't, prefer the non-negated set. In general, negated
	// sets are large and thus likely to match more frequently, making them slower to search for.
	if s1.Negated != s2.Negated {
		if s2.Negated {
			return -1
		}
		return 1
	}

	// If we extracted only a few chars and the sets are negated, they both represent very large
	// sets that are difficult to compare for quality.
	if !s1.Negated {
		// If both have chars, prioritize the one with the smaller frequency for those chars.
		if len(s1.Chars) > 0 && len(s2.Chars) > 0 {
			// Prefer sets with less frequent values.  The frequency is only an approximation,
			// used as a tie-breaker when we'd otherwise effectively be picking randomly.
			// True frequencies will vary widely based on the actual data being searched, the language of the data, etc.
			s1Frequency := sumFrequencies(s1.Chars)
			s2Frequency := sumFrequencies(s2.Chars)

			if s1Frequency != s2Frequency {
				return cmp.Compare(s1Frequency, s2Frequency)
			}

			if !isAsciiRunes(s1.Chars) && !isAsciiRunes(s2.Chars) {
				// Prefer the set with fewer values.
				return cmp.Compare(len(s1.Chars), len(s2.Chars))
			}
		}

		// If one has chars and the other has a range, prefer the shorter set.
		if (len(s1.Chars) > 0 && s2RangeLength > 0) || (s1RangeLength > 0 && len(s2.Chars) > 0) {
			s1Len := s1RangeLength
			if len(s1.Chars) > s1Len {
				s1Len = len(s1.Chars)
			}
			s2Len := s2RangeLength
			if len(s2.Chars) > s2Len {
				s2Len = len(s2.Chars)
			}
			c := cmp.Compare(s1Len, s2Len)
			if c != 0 {
				return c
			}

			// If lengths are the same, prefer the chars.
			if len(s1.Chars) > 0 {
				return -1
			}
			return 1
		}

		// If one has chars and the other doesn't, prioritize the one with chars.
		if (len(s1.Chars) > 0) != (len(s2.Chars) > 0) {
			if len(s1.Chars) > 0 {
				return -1
			}
			return 1
		}
	}

	// If one has a range and the other doesn't, prioritize the one with a range.
	if (s1RangeLength > 0) != (s2RangeLength > 0) {
		if s1RangeLength > 0 {
			return -1
		}
		return 1
	}

	// If both have ranges, prefer the one that includes fewer characters.
	if s1RangeLength > 0 {
		return cmp.Compare(s1RangeLength, s2RangeLength)
	}

	// As a tiebreaker, prioritize the earlier one.
	return cmp.Compare(s1.Distance, s2.Distance)
}

func getRangeLength(r *SingleRange, negated bool) int {
	if r == nil {
		return 0
	}
	if negated {
		return int(unicode.MaxRune - (r.Last - r.First))
	}
	return int(r.Last - r.First + 1)
}

func sumFrequencies(chars []rune) float32 {
	var sum float32
	for _, c := range chars {
		// Lookup each character in the table.  Values >= 128 are ignored
		// and thus we'll get skew in the data.  It's already a gross approximation, though,
		// and it is primarily meant for disambiguation of ASCII letters.
		if c < unicode.MaxASCII {
			sum += frequency[c]
		}
	}
	return sum
}

func hasHighFrequencyChars(set FixedDistanceSet) bool {
	if set.Negated {
		return true
	}

	// Sets without extracted chars can't be frequency-analyzed.
	// Single-char sets use IndexOf, which is a strong filter regardless of frequency.
	if len(set.Chars) <= 1 {
		return false
	}

	totalFrequency := sumFrequencies(set.Chars)

	// If the average frequency of the set's chars exceeds this threshold, the
	// characters are common enough that a multi-string search may be a better filter.
	const highFrequencyThreshold = 0.6
	return totalFrequency >= highFrequencyThreshold*float32(len(set.Chars))
}

func mayContainCaseInsensitiveMatching(node *RegexNode) bool {
	if node.Options&IgnoreCase != 0 {
		return true
	}

	if node.Set != nil {
		chars := node.Set.GetSetChars(maxPrefixes)
		for _, ch := range chars {
			if participatesInCaseConversion(ch) &&
				slices.Contains(chars, unicode.ToLower(ch)) &&
				slices.Contains(chars, unicode.ToUpper(ch)) {
				return true
			}
		}
	}

	for _, child := range node.Children {
		if mayContainCaseInsensitiveMatching(child) {
			return true
		}
	}

	return false
}

// Percent occurrences in source text (100 * char count / total count)
var frequency = []float32{
	0.000 /* '\x00' */, 0.000 /* '\x01' */, 0.000 /* '\x02' */, 0.000 /* '\x03' */, 0.000 /* '\x04' */, 0.000 /* '\x05' */, 0.000 /* '\x06' */, 0.000, /* '\x07' */
	0.000 /* '\x08' */, 0.001 /* '\x09' */, 0.000 /* '\x0A' */, 0.000 /* '\x0B' */, 0.000 /* '\x0C' */, 0.000 /* '\x0D' */, 0.000 /* '\x0E' */, 0.000, /* '\x0F' */
	0.000 /* '\x10' */, 0.000 /* '\x11' */, 0.000 /* '\x12' */, 0.000 /* '\x13' */, 0.003 /* '\x14' */, 0.000 /* '\x15' */, 0.000 /* '\x16' */, 0.000, /* '\x17' */
	0.000 /* '\x18' */, 0.004 /* '\x19' */, 0.000 /* '\x1A' */, 0.000 /* '\x1B' */, 0.006 /* '\x1C' */, 0.006 /* '\x1D' */, 0.000 /* '\x1E' */, 0.000, /* '\x1F' */
	8.952 /* '    ' */, 0.065 /* '   !' */, 0.420 /* '   "' */, 0.010 /* '   #' */, 0.011 /* '   $' */, 0.005 /* '   %' */, 0.070 /* '   &' */, 0.050, /* '   '' */
	3.911 /* '   (' */, 3.910 /* '   )' */, 0.356 /* '   *' */, 2.775 /* '   +' */, 1.411 /* '   ,' */, 0.173 /* '   -' */, 2.054 /* '   .' */, 0.677, /* '   /' */
	1.199 /* '   0' */, 0.870 /* '   1' */, 0.729 /* '   2' */, 0.491 /* '   3' */, 0.335 /* '   4' */, 0.269 /* '   5' */, 0.435 /* '   6' */, 0.240, /* '   7' */
	0.234 /* '   8' */, 0.196 /* '   9' */, 0.144 /* '   :' */, 0.983 /* '   ;' */, 0.357 /* '   <' */, 0.661 /* '   =' */, 0.371 /* '   >' */, 0.088, /* '   ?' */
	0.007 /* '   @' */, 0.763 /* '   A' */, 0.229 /* '   B' */, 0.551 /* '   C' */, 0.306 /* '   D' */, 0.449 /* '   E' */, 0.337 /* '   F' */, 0.162, /* '   G' */
	0.131 /* '   H' */, 0.489 /* '   I' */, 0.031 /* '   J' */, 0.035 /* '   K' */, 0.301 /* '   L' */, 0.205 /* '   M' */, 0.253 /* '   N' */, 0.228, /* '   O' */
	0.288 /* '   P' */, 0.034 /* '   Q' */, 0.380 /* '   R' */, 0.730 /* '   S' */, 0.675 /* '   T' */, 0.265 /* '   U' */, 0.309 /* '   V' */, 0.137, /* '   W' */
	0.084 /* '   X' */, 0.023 /* '   Y' */, 0.023 /* '   Z' */, 0.591 /* '   [' */, 0.085 /* '   \' */, 0.590 /* '   ]' */, 0.013 /* '   ^' */, 0.797, /* '   _' */
	0.001 /* '   `' */, 4.596 /* '   a' */, 1.296 /* '   b' */, 2.081 /* '   c' */, 2.005 /* '   d' */, 6.903 /* '   e' */, 1.494 /* '   f' */, 1.019, /* '   g' */
	1.024 /* '   h' */, 3.750 /* '   i' */, 0.286 /* '   j' */, 0.439 /* '   k' */, 2.913 /* '   l' */, 1.459 /* '   m' */, 3.908 /* '   n' */, 3.230, /* '   o' */
	1.444 /* '   p' */, 0.231 /* '   q' */, 4.220 /* '   r' */, 3.924 /* '   s' */, 5.312 /* '   t' */, 2.112 /* '   u' */, 0.737 /* '   v' */, 0.573, /* '   w' */
	0.992 /* '   x' */, 1.067 /* '   y' */, 0.181 /* '   z' */, 0.391 /* '   {' */, 0.056 /* '   |' */, 0.391 /* '   }' */, 0.002 /* '   ~' */, 0.000, /* '\x7F' */
}

// The above table was generated programmatically with the following.  This can be augmented to incorporate additional data sources,
// though it is only intended to be a rough approximation use when tie-breaking and we'd otherwise be picking randomly, so, it's something.
// The frequencies may be wildly inaccurate when used with data sources different in nature than the training set, in which case we shouldn't
// be much worse off than just picking randomly:
//
// using System.Runtime.InteropServices;
//
// var counts = new Dictionary<byte, long>();
//
// (string, string)[] rootsAndExtensions = new[]
// {
//     (@"d:\repos\runtime\src\", "*.cs"),   // C# files in dotnet/runtime
//     (@"d:\Top25GutenbergBooks", "*.txt"), // Top 25 most popular books on Project Gutenberg
// };
//
// foreach ((string root, string ext) in rootsAndExtensions)
//     foreach (string path in Directory.EnumerateFiles(root, ext, SearchOption.AllDirectories))
//         foreach (string line in File.ReadLines(path))
//             foreach (char c in line.AsSpan().Trim())
//                 CollectionsMarshal.GetValueRefOrAddDefault(counts, (byte)c, out _)++;
//
// long total = counts.Sum(i => i.Value);
//
// Console.WriteLine("/// <summary>Percent occurrences in source text (100 * char count / total count).</summary>");
// Console.WriteLine("private static ReadOnlySpan<float> Frequency =>");
// Console.WriteLine("[");
// int i = 0;
// for (int row = 0; row < 16; row++)
// {
//     Console.Write("   ");
//     for (int col = 0; col < 8; col++)
//     {
//         counts.TryGetValue((byte)i, out long charCount);
//         float frequency = (float)(charCount / (double)total) * 100;
//         Console.Write($" {frequency:N3}f /* '{(i >= 32 && i < 127 ? $"   {(char)i}" : $"\\x{i:X2}")}' */,");
//         i++;
//     }
//     Console.WriteLine();
// }
// Console.WriteLine("];");

// / <summary>
// / Analyzes the pattern for a leading set loop followed by a non-overlapping literal. If such a pattern is found, an implementation
// / can search for the literal and then walk backward through all matches for the loop until the beginning is found.
// / </summary>
func findLiteralFollowingLeadingLoop(node *RegexNode) *LiteralAfterLoop {
	if (node.Options & RightToLeft) != 0 {
		// As a simplification, ignore RightToLeft.
		return nil
	}

	// Find the first concatenation.  We traverse through atomic and capture nodes as they don't effect flow control.  (We don't
	// want to explore loops, even if they have a guaranteed iteration, because we may use information about the node to then
	// skip the node's execution in the matching algorithm, and we would need to special-case only skipping the first iteration.)
	for node.T == NtAtomic || node.T == NtCapture {
		node = node.Children[0]
	}
	if node.T != NtConcatenate {
		return nil
	}

	// Bail if the first node isn't a set loop.  We treat any kind of set loop (Setloop, Setloopatomic, and Setlazy)
	// the same because of two important constraints: the loop must not have an upper bound, and the literal we look
	// for immediately following it must not overlap.  With those constraints, all three of these kinds of loops will
	// end up having the same semantics; in fact, if atomic optimizations are used, we will have converted Setloop
	// into a Setloopatomic (but those optimizations are disabled for NonBacktracking in general). This
	// could also be made to support Oneloopatomic and Notoneloopatomic, but the scenarios for that are rare.
	firstChild := node.Children[0]
	for firstChild.T == NtAtomic || firstChild.T == NtCapture {
		firstChild = firstChild.Children[0]
	}
	if (firstChild.T != NtSetloop && firstChild.T != NtSetloopatomic && firstChild.T != NtSetlazy) ||
		firstChild.N != math.MaxInt32 {
		return nil
	}

	// Get the subsequent node.  An UpdateBumpalong may have been added as an optimization, but it doesn't have an
	// impact on semantics and we can skip it.
	nextChild := node.Children[1]
	if nextChild.T == NtUpdateBumpalong {
		if len(node.Children) == 2 {
			// If the UpdateBumpalong is the last node, nothing meaningful follows the set loop.
			return nil
		}
		nextChild = node.Children[2]
	}
	nextChild = unwrapImmediateLiteralAfterLoopNode(nextChild)
	if nextChild == nil {
		return nil
	}

	// Is the set loop followed by a case-sensitive string we can search for?
	if prefix := findPrefix(nextChild); len(prefix) >= 1 {
		// The literal can be searched for as either a single char or as a string.
		// But we need to make sure that its starting character isn't part of the preceding
		// set, as then we can't know for certain where the set loop ends.
		if firstChild.Set.CharIn(rune(prefix[0])) {
			return nil
		} else if len(prefix) == 1 {
			return &LiteralAfterLoop{
				LoopNode: firstChild,
				Char:     rune(prefix[0]),
			}
		}
		return &LiteralAfterLoop{
			LoopNode: firstChild,
			String:   prefix,
		}
	}

	// Is the set loop followed by an ordinal case-insensitive string we can search for? We could
	// search for a string with at least one char, but if it has only one, we're better off just
	// searching as a set, so we look for strings with at least two chars.
	if ordinalCaseInsensitivePrefix := findPrefixOrdinalCaseInsensitive(nextChild); len(ordinalCaseInsensitivePrefix) >= 2 {
		// The literal can be searched for as a case-insensitive string. As with ordinal above,
		// though, we need to make sure its starting character isn't part of the previous set.
		// If that starting character participates in case conversion, then we need to test out
		// both casings (FindPrefixOrdinalCaseInsensitive will only return strings composed of
		// characters that either are ASCII or that don't participate in case conversion).
		ch, _ := utf8.DecodeRuneInString(ordinalCaseInsensitivePrefix)
		if participatesInCaseConversion(ch) {
			if firstChild.Set.CharIn(ch|0x20) ||
				firstChild.Set.CharIn(ch&^0x20) {
				return nil
			}
		} else if firstChild.Set.CharIn(ch) {
			return nil
		}

		return &LiteralAfterLoop{
			LoopNode:         firstChild,
			String:           ordinalCaseInsensitivePrefix,
			StringIgnoreCase: true,
		}
	}

	// If the resulting node is a set with at least one iteration, we can search for it.
	if nextChild.IsSetFamily() &&
		!nextChild.Set.IsNegated() &&
		(nextChild.T == NtSet || nextChild.M >= 1) {
		// maximum number of chars optimized by IndexOfAny
		chars := nextChild.Set.GetSetChars(5)
		if len(chars) > 0 {
			for _, c := range chars {
				if firstChild.Set.CharIn(c) {
					return nil
				}
			}

			return &LiteralAfterLoop{
				LoopNode: firstChild,
				Chars:    chars,
			}
		}
	}

	// Otherwise, we couldn't find the pattern of an atomic set loop followed by a literal.
	return nil
}

func unwrapImmediateLiteralAfterLoopNode(node *RegexNode) *RegexNode {
	for {
		node = unwrapTransparentNodes(node)
		if node == nil {
			return nil
		}
		if node.T != NtConcatenate {
			return node
		}
		if len(node.Children) == 0 {
			return nil
		}
		node = node.Children[0]
	}
}

func findRequiredLandmarkChain(node *RegexNode) *RequiredLandmarkChain {
	if (node.Options & RightToLeft) != 0 {
		return nil
	}

	node = unwrapTransparentNodes(node)
	if node.T != NtConcatenate || len(node.Children) < 4 {
		return nil
	}

	firstChild := unwrapTransparentNodes(node.Children[0])
	if !isUnboundedSetLoop(firstChild) {
		return nil
	}

	// Collect landmarks that must appear in order later in the match. This is a
	// conservative prefilter: if any child cannot be described as a detectable
	// landmark, we skip that child rather than making it part of the chain.
	landmarks := make([]RequiredLandmark, 0, 2)
	for _, child := range node.Children[1:] {
		if landmark, ok := extractRequiredLandmark(child); ok {
			landmarks = append(landmarks, landmark)
		} else if len(landmarks) == 0 && !isZeroWidthLandmarkGap(child) {
			return nil
		}
	}
	if len(landmarks) < 2 {
		return nil
	}

	return &RequiredLandmarkChain{
		LeadingLoopSet: firstChild.Set,
		Landmarks:      landmarks,
	}
}

func isZeroWidthLandmarkGap(node *RegexNode) bool {
	node = unwrapTransparentNodes(node)
	if node == nil {
		return true
	}
	switch node.T {
	case NtEmpty, NtUpdateBumpalong,
		NtBeginning, NtBol, NtStart, NtEndZ, NtEnd, NtEol,
		NtBoundary, NtNonboundary, NtECMABoundary, NtNonECMABoundary:
		return true
	default:
		return false
	}
}

func unwrapTransparentNodes(node *RegexNode) *RegexNode {
	for node != nil && (node.T == NtAtomic || node.T == NtCapture || node.T == NtGroup) && len(node.Children) == 1 {
		node = node.Children[0]
	}
	return node
}

func isUnboundedSetLoop(node *RegexNode) bool {
	return node != nil &&
		(node.T == NtSetloop || node.T == NtSetloopatomic || node.T == NtSetlazy) &&
		node.Set != nil &&
		node.N == math.MaxInt32
}

func extractRequiredLandmark(node *RegexNode) (RequiredLandmark, bool) {
	node = unwrapTransparentNodes(node)
	if node == nil {
		return RequiredLandmark{}, false
	}

	if node.T != NtAlternate {
		alt, ok := extractRequiredLandmarkAlternative(node)
		if !ok {
			return RequiredLandmark{}, false
		}
		return RequiredLandmark{Alternatives: []RequiredLandmarkAlternative{alt}}, true
	}

	landmark := RequiredLandmark{Alternatives: make([]RequiredLandmarkAlternative, 0, len(node.Children))}
	for _, child := range node.Children {
		alt, ok := extractRequiredLandmarkAlternative(child)
		if !ok {
			return RequiredLandmark{}, false
		}
		landmark.Alternatives = append(landmark.Alternatives, alt)
	}
	return landmark, len(landmark.Alternatives) > 0
}

func extractRequiredLandmarkAlternative(node *RegexNode) (RequiredLandmarkAlternative, bool) {
	node = unwrapTransparentNodes(node)
	if node == nil {
		return RequiredLandmarkAlternative{}, false
	}

	children := []*RegexNode{node}
	if node.T == NtConcatenate {
		children = node.Children
	}

	alt := RequiredLandmarkAlternative{}
	i := 0
	if i < len(children) {
		if whitespaceSet, min, ok := whitespaceLoop(children[i]); ok {
			alt.LeadingWhitespaceSet = whitespaceSet
			alt.RequireWhitespaceBefore = min > 0
			i++
		}
	}
	if i >= len(children) {
		return RequiredLandmarkAlternative{}, false
	}

	// The core of a landmark must be a literal or a bounded, enumerable set
	// repetition. The VM still validates the full regex after this prefilter
	// returns a candidate.
	core := unwrapTransparentNodes(children[i])
	switch core.T {
	case NtOne:
		alt.Literal = []rune{core.Ch}
		alt.MinRepeat = 1
		alt.MaxRepeat = 1
	case NtMulti:
		alt.Literal = core.Str
		alt.MinRepeat = 1
		alt.MaxRepeat = 1
	case NtSet, NtSetloop, NtSetloopatomic, NtSetlazy:
		if core.Set == nil {
			return RequiredLandmarkAlternative{}, false
		}
		if core.T == NtSet {
			alt.MinRepeat = 1
			alt.MaxRepeat = 1
		} else if core.M > 0 && core.N != math.MaxInt32 {
			alt.MinRepeat = core.M
			alt.MaxRepeat = core.N
		} else {
			return RequiredLandmarkAlternative{}, false
		}
		chars := core.Set.GetSetChars(8)
		if len(chars) == 0 || core.Set.IsNegated() {
			return RequiredLandmarkAlternative{}, false
		}
		alt.Set = core.Set
	default:
		return RequiredLandmarkAlternative{}, false
	}
	i++

	if i < len(children) {
		if whitespaceSet, min, ok := whitespaceLoop(children[i]); ok {
			alt.TrailingWhitespaceSet = whitespaceSet
			alt.RequireWhitespaceAfter = min > 0
			i++
		}
	}
	if i != len(children) {
		return RequiredLandmarkAlternative{}, false
	}
	return alt, true
}

func whitespaceLoop(node *RegexNode) (*CharSet, int, bool) {
	node = unwrapTransparentNodes(node)
	if node != nil &&
		(node.T == NtSetloop || node.T == NtSetloopatomic || node.T == NtSetlazy) &&
		node.Set != nil &&
		node.N == math.MaxInt32 &&
		(node.Set.Equals(SpaceClass()) || node.Set.Equals(ECMASpaceClass()) || node.Set.Equals(RE2SpaceClass())) {
		return node.Set, node.M, true
	}
	return nil, 0, false
}

// Returns a leading positive lookahead if found and whether to keep examining subsequent nodes in a concatenation.
func findLeadingPositiveLookahead(node *RegexNode) (*RegexNode, bool) {
	for {
		if node.Options&RightToLeft != 0 {
			return nil, false
		}

		switch node.T {
		case NtPosLook:
			return node, false

		case NtBol, NtEol, NtBeginning, NtStart, NtEndZ, NtEnd,
			NtBoundary, NtECMABoundary, NtNegLook, NtEmpty:
			return nil, true

		case NtAtomic, NtCapture:
			node = node.Children[0]
			continue

		case NtLoop, NtLazyloop:
			if node.M < 1 {
				return nil, false
			}
			lookahead, _ := findLeadingPositiveLookahead(node.Children[0])
			return lookahead, false

		case NtConcatenate:
			for i := 0; i < len(node.Children); i++ {
				lookahead, keepLooking := findLeadingPositiveLookahead(node.Children[i])
				if lookahead != nil || !keepLooking {
					return lookahead, false
				}
			}
			return nil, true

		default:
			return nil, false
		}
	}
}
