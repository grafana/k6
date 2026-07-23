package syntax

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"
)

type FindOptimizations struct {
	rightToLeft  bool
	asciiLookups [][]uint

	FindMode             FindNextStartingPositionMode
	LeadingAnchor        NodeType
	TrailingAnchor       NodeType
	MinRequiredLength    int
	MaxPossibleLength    int
	LeadingPrefix        string
	LeadingPrefixes      []string
	LeadingPrefixesRunes [][]rune
	//LeadingStrings    *helpers.StringSearchValues

	FixedDistanceLiteral FixedDistanceLiteral
	FixedDistanceSets    []FixedDistanceSet
	LiteralAfterLoop     *LiteralAfterLoop
	LandmarkChain        *RequiredLandmarkChain
}

type LiteralAfterLoop struct {
	String           string
	StringIgnoreCase bool
	Char             rune
	Chars            []rune

	LoopNode *RegexNode
}

type FixedDistanceSet struct {
	Set      *CharSet
	Chars    []rune
	Negated  bool
	Range    *SingleRange
	Distance int
}

type FixedDistanceLiteral struct {
	S        string
	C        rune
	Distance int
}

type RequiredLandmarkChain struct {
	// LeadingLoopSet is the unbounded leading set loop that precedes every
	// landmark in the original concatenation. At run time, after the first
	// landmark alternative is found, the scanner walks backward over this set
	// to recover the earliest plausible regex start position.
	LeadingLoopSet *CharSet

	// Landmarks must all be found, in slice order, for the chain prefilter to
	// produce a candidate. Each landmark is satisfied by exactly one matching
	// alternative; alternatives are tried independently by the runner.
	Landmarks []RequiredLandmark
}

type RequiredLandmark struct {
	// Alternatives describes the mutually exclusive shapes that can satisfy
	// this single required landmark. A landmark matches when any one alternative
	// matches at a position in the input.
	Alternatives []RequiredLandmarkAlternative
}

type RequiredLandmarkAlternative struct {
	// Literal is the core token for literal alternatives. When non-empty, it
	// must match exactly at the candidate core position, and Set must be nil.
	Literal []rune

	// Set is the core token for character-class alternatives. When non-nil, it
	// must match between MinRepeat and MaxRepeat runes at the candidate core
	// position, and Literal must be empty. The analyzer only builds set
	// alternatives for non-negated sets that can be cheaply enumerated, but the
	// runner uses Set for membership checks.
	Set *CharSet

	// LeadingWhitespaceSet is the optional or required whitespace immediately
	// before the core token. If RequireWhitespaceBefore is true, at least one
	// rune from this set must precede the core. When this alternative is the
	// first matched landmark, the runner may rewind over additional contiguous
	// leading whitespace from this set before rewinding over LeadingLoopSet.
	LeadingWhitespaceSet *CharSet

	// TrailingWhitespaceSet is the optional or required whitespace immediately
	// after the core token. If RequireWhitespaceAfter is true, at least one rune
	// from this set must follow the core. The runner validates the requirement,
	// but does not consume optional trailing whitespace into the landmark end.
	TrailingWhitespaceSet *CharSet

	// MinRepeat and MaxRepeat describe the core token width. Literal alternatives
	// use 1..1 regardless of literal length because the literal is matched as one
	// fixed core token; Set alternatives use the source set repetition.
	MinRepeat int
	MaxRepeat int

	RequireWhitespaceBefore bool
	RequireWhitespaceAfter  bool
}

type FindNextStartingPositionMode int

const (
	NoSearch FindNextStartingPositionMode = iota
	// A "beginning" anchor at the beginning of the pattern.
	LeadingAnchor_LeftToRight_Beginning
	// A "start" anchor at the beginning of the pattern.
	LeadingAnchor_LeftToRight_Start
	// An "endz" anchor at the beginning of the pattern.  This is rare.
	LeadingAnchor_LeftToRight_EndZ
	// An "end" anchor at the beginning of the pattern.  This is rare.
	LeadingAnchor_LeftToRight_End
	// A "beginning" anchor at the beginning of the right-to-left pattern.
	LeadingAnchor_RightToLeft_Beginning
	// A "start" anchor at the beginning of the right-to-left pattern.
	LeadingAnchor_RightToLeft_Start
	// An "endz" anchor at the beginning of the right-to-left pattern.  This is rare.
	LeadingAnchor_RightToLeft_EndZ
	// An "end" anchor at the beginning of the right-to-left pattern.  This is rare.
	LeadingAnchor_RightToLeft_End
	// An "end" anchor at the end of the pattern, with the pattern always matching a fixed-length expression.
	TrailingAnchor_FixedLength_LeftToRight_End
	// An "endz" anchor at the end of the pattern, with the pattern always matching a fixed-length expression.
	TrailingAnchor_FixedLength_LeftToRight_EndZ
	// A multi-character substring at the beginning of the pattern.
	LeadingString_LeftToRight
	// A multi-character substring at the beginning of the right-to-left pattern.
	LeadingString_RightToLeft
	// A multi-character ordinal case-insensitive substring at the beginning of the pattern.
	LeadingString_OrdinalIgnoreCase_LeftToRight
	// Multiple leading prefix strings
	LeadingStrings_LeftToRight
	// Multiple leading ordinal case-insensitive prefix strings
	LeadingStrings_OrdinalIgnoreCase_LeftToRight

	// A set starting the pattern.
	LeadingSet_LeftToRight
	// A set starting the right-to-left pattern.
	LeadingSet_RightToLeft

	// A single character at the start of the right-to-left pattern.
	LeadingChar_RightToLeft

	// A single character at a fixed distance from the start of the pattern.
	FixedDistanceChar_LeftToRight
	// A multi-character case-sensitive string at a fixed distance from the start of the pattern.
	FixedDistanceString_LeftToRight

	// One or more sets at a fixed distance from the start of the pattern.
	FixedDistanceSets_LeftToRight

	// A literal (single character, multi-char string, or set with small number of characters) after a non-overlapping set loop at the start of the pattern.
	LiteralAfterLoop_LeftToRight

	// A sequence of required landmarks after a leading loop.
	RequiredLandmarkChain_LeftToRight
)

func (m FindNextStartingPositionMode) String() string {
	switch m {
	case NoSearch:
		return "NoSearch"
	case LeadingAnchor_LeftToRight_Beginning:
		return "LeadingAnchor_LeftToRight_Beginning"
	case LeadingAnchor_LeftToRight_Start:
		return "LeadingAnchor_LeftToRight_Start"
	case LeadingAnchor_LeftToRight_EndZ:
		return "LeadingAnchor_LeftToRight_EndZ"
	case LeadingAnchor_LeftToRight_End:
		return "LeadingAnchor_LeftToRight_End"
	case LeadingAnchor_RightToLeft_Beginning:
		return "LeadingAnchor_RightToLeft_Beginning"
	case LeadingAnchor_RightToLeft_Start:
		return "LeadingAnchor_RightToLeft_Start"
	case LeadingAnchor_RightToLeft_EndZ:
		return "LeadingAnchor_RightToLeft_EndZ"
	case LeadingAnchor_RightToLeft_End:
		return "LeadingAnchor_RightToLeft_End"
	case TrailingAnchor_FixedLength_LeftToRight_End:
		return "TrailingAnchor_FixedLength_LeftToRight_End"
	case TrailingAnchor_FixedLength_LeftToRight_EndZ:
		return "TrailingAnchor_FixedLength_LeftToRight_EndZ"
	case LeadingString_LeftToRight:
		return "LeadingString_LeftToRight"
	case LeadingString_RightToLeft:
		return "LeadingString_RightToLeft"
	case LeadingString_OrdinalIgnoreCase_LeftToRight:
		return "LeadingString_OrdinalIgnoreCase_LeftToRight"
	case LeadingStrings_LeftToRight:
		return "LeadingStrings_LeftToRight"
	case LeadingStrings_OrdinalIgnoreCase_LeftToRight:
		return "LeadingStrings_OrdinalIgnoreCase_LeftToRight"
	case LeadingSet_LeftToRight:
		return "LeadingSet_LeftToRight"
	case LeadingSet_RightToLeft:
		return "LeadingSet_RightToLeft"
	case LeadingChar_RightToLeft:
		return "LeadingChar_RightToLeft"
	case FixedDistanceChar_LeftToRight:
		return "FixedDistanceChar_LeftToRight"
	case FixedDistanceString_LeftToRight:
		return "FixedDistanceString_LeftToRight"
	case FixedDistanceSets_LeftToRight:
		return "FixedDistanceSets_LeftToRight"
	case LiteralAfterLoop_LeftToRight:
		return "LiteralAfterLoop_LeftToRight"
	case RequiredLandmarkChain_LeftToRight:
		return "RequiredLandmarkChain_LeftToRight"
	default:
		return fmt.Sprintf("FindNextStartingPositionMode(%d)", int(m))
	}
}

func (f *FindOptimizations) Dump() string {
	buf := &bytes.Buffer{}
	if f == nil {
		fmt.Fprintln(buf, "Find mode:  n/a")
		return buf.String()
	}

	fmt.Fprintf(buf, "Find mode:  %s\n", f.FindMode)
	fmt.Fprintf(buf, "Min length: %d\n", f.MinRequiredLength)
	if f.MaxPossibleLength > 0 {
		fmt.Fprintf(buf, "Max length: %d\n", f.MaxPossibleLength)
	}

	switch f.FindMode {
	case LeadingAnchor_LeftToRight_Beginning, LeadingAnchor_LeftToRight_Start,
		LeadingAnchor_LeftToRight_EndZ, LeadingAnchor_LeftToRight_End,
		LeadingAnchor_RightToLeft_Beginning, LeadingAnchor_RightToLeft_Start,
		LeadingAnchor_RightToLeft_EndZ, LeadingAnchor_RightToLeft_End:
		fmt.Fprintf(buf, "Anchor:     %v\n", f.LeadingAnchor)

	case TrailingAnchor_FixedLength_LeftToRight_End, TrailingAnchor_FixedLength_LeftToRight_EndZ:
		fmt.Fprintf(buf, "Anchor:     %v\n", f.TrailingAnchor)

	case LeadingString_LeftToRight, LeadingString_RightToLeft, LeadingString_OrdinalIgnoreCase_LeftToRight:
		fmt.Fprintf(buf, "Prefix:     %q\n", f.LeadingPrefix)

	case LeadingStrings_LeftToRight, LeadingStrings_OrdinalIgnoreCase_LeftToRight:
		fmt.Fprintf(buf, "Prefixes:   %q\n", f.LeadingPrefixes)

	case FixedDistanceChar_LeftToRight:
		fmt.Fprintf(buf, "Literal:    %q at distance %d\n", f.FixedDistanceLiteral.C, f.FixedDistanceLiteral.Distance)

	case FixedDistanceString_LeftToRight:
		fmt.Fprintf(buf, "Literal:    %q at distance %d\n", f.FixedDistanceLiteral.S, f.FixedDistanceLiteral.Distance)

	case FixedDistanceSets_LeftToRight:
		for i, set := range f.FixedDistanceSets {
			fmt.Fprintf(buf, "Set[%d]:     distance=%d %s\n", i, set.Distance, fixedDistanceSetDescription(set))
		}

	case LiteralAfterLoop_LeftToRight:
		fmt.Fprintf(buf, "Literal:    %s\n", literalAfterLoopDescription(f.LiteralAfterLoop))

	case RequiredLandmarkChain_LeftToRight:
		fmt.Fprintf(buf, "Landmarks:  %s\n", landmarkChainDescription(f.LandmarkChain))
	}

	return buf.String()
}

func fixedDistanceSetDescription(set FixedDistanceSet) string {
	switch {
	case len(set.Chars) > 0:
		return fmt.Sprintf("chars=%q negated=%t", string(set.Chars), set.Negated)
	case set.Range != nil:
		return fmt.Sprintf("range=%q-%q negated=%t", set.Range.First, set.Range.Last, set.Negated)
	case set.Set != nil:
		return fmt.Sprintf("set=%s", set.Set.String())
	default:
		return "set=<nil>"
	}
}

func literalAfterLoopDescription(literal *LiteralAfterLoop) string {
	if literal == nil {
		return "<nil>"
	}
	loopSet := "<nil>"
	if literal.LoopNode != nil && literal.LoopNode.Set != nil {
		loopSet = literal.LoopNode.Set.String()
	}
	switch {
	case literal.String != "":
		return fmt.Sprintf("string=%q ignoreCase=%t after loop set %s", literal.String, literal.StringIgnoreCase, loopSet)
	case len(literal.Chars) > 0:
		return fmt.Sprintf("chars=%q after loop set %s", string(literal.Chars), loopSet)
	default:
		return fmt.Sprintf("char=%q after loop set %s", literal.Char, loopSet)
	}
}

func landmarkChainDescription(chain *RequiredLandmarkChain) string {
	if chain == nil {
		return "<nil>"
	}
	loopSet := "<nil>"
	if chain.LeadingLoopSet != nil {
		loopSet = chain.LeadingLoopSet.String()
	}
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, "leadingLoop=%s", loopSet)
	for i, landmark := range chain.Landmarks {
		fmt.Fprintf(buf, " landmark[%d]=", i)
		for j, alt := range landmark.Alternatives {
			if j > 0 {
				buf.WriteString("|")
			}
			buf.WriteString(requiredLandmarkAlternativeDescription(alt))
		}
	}
	return buf.String()
}

func requiredLandmarkAlternativeDescription(alt RequiredLandmarkAlternative) string {
	core := ""
	switch {
	case len(alt.Literal) > 0:
		core = fmt.Sprintf("literal=%q", string(alt.Literal))
	case alt.Set != nil:
		core = fmt.Sprintf("set=%s repeat=%d..%d", alt.Set.String(), alt.MinRepeat, alt.MaxRepeat)
	default:
		core = "<empty>"
	}
	if alt.LeadingWhitespaceSet != nil {
		core += fmt.Sprintf(" leadingWhitespace=%s required=%t", alt.LeadingWhitespaceSet.String(), alt.RequireWhitespaceBefore)
	}
	if alt.TrailingWhitespaceSet != nil {
		core += fmt.Sprintf(" trailingWhitespace=%s required=%t", alt.TrailingWhitespaceSet.String(), alt.RequireWhitespaceAfter)
	}
	return core
}

func newFindOptimizations(tree *RegexTree, opt ParseOptions) *FindOptimizations {
	f := newFindOptimizationsForNode(tree.Root, opt, false)

	if !f.rightToLeft && !f.isUseful() {
		if positiveLookahead, _ := findLeadingPositiveLookahead(tree.Root); positiveLookahead != nil {
			positiveLookaheadOpts := newFindOptimizationsForNode(positiveLookahead.Children[0], opt, true)

			// Lookaheads don't currently factor into the whole-pattern length analysis,
			// as they can overlap with the rest of the expression. Keep the whole-pattern
			// minimum if it's larger, and preserve any max from the original expression.
			positiveLookaheadOpts.MinRequiredLength = max(f.MinRequiredLength, positiveLookaheadOpts.MinRequiredLength)
			positiveLookaheadOpts.MaxPossibleLength = f.MaxPossibleLength
			f = positiveLookaheadOpts
		}
	}

	return f
}

func newFindOptimizationsForNode(root *RegexNode, opt ParseOptions, isLeadingPartial bool) *FindOptimizations {
	f := &FindOptimizations{
		rightToLeft:       opt.RegexOptions&RightToLeft != 0,
		MinRequiredLength: root.ComputeMinLength(),
		LeadingAnchor:     findLeadingOrTrailingAnchor(root, true),
		MaxPossibleLength: -1,
	}

	if f.rightToLeft && f.LeadingAnchor == NtBol {
		// Filter out Bol for RightToLeft, as we don't currently optimize for it.
		f.LeadingAnchor = NtUnknown
	}

	f.FindMode = getFindMode(f.rightToLeft, f.LeadingAnchor)
	if f.FindMode != NoSearch {
		return f
	}

	// Compute any anchor trailing the expression.  If there is one, and we can also compute a fixed length
	// for the whole expression, we can use that to quickly jump to the right location in the input.
	if !f.rightToLeft && !isLeadingPartial {
		f.TrailingAnchor = findLeadingOrTrailingAnchor(root, false)
		if f.TrailingAnchor == NtEnd || f.TrailingAnchor == NtEndZ {
			f.MaxPossibleLength = root.computeMaxLength()
			if f.MinRequiredLength == f.MaxPossibleLength {
				if f.TrailingAnchor == NtEnd {
					f.FindMode = TrailingAnchor_FixedLength_LeftToRight_End
				} else {
					f.FindMode = TrailingAnchor_FixedLength_LeftToRight_EndZ
				}
				return f
			}
		}
	}

	// If there's a leading substring, just use IndexOf and inherit all of its optimizations.
	prefix := findPrefix(root)
	if len(prefix) > 1 {
		f.LeadingPrefix = prefix
		if f.rightToLeft {
			f.FindMode = LeadingString_RightToLeft
		} else {
			f.FindMode = LeadingString_LeftToRight
		}
		return f
	}

	// At this point there are no fast-searchable anchors or case-sensitive prefixes. We can now analyze the
	// pattern for sets and then use any found sets to determine what kind of search to perform.

	// If we're generating code, then the code generation process already handles sets that reduce to a single literal,
	// so we can simplify and just always go for the sets.
	dfa := false                   //(opt&NonBacktracking) != 0
	codeGen := opt.CodeGen && !dfa // for now, we never generate code for NonBacktracking, so treat it as non-codegen
	interpreter := !codeGen && !dfa
	//usesRfoTryFind := !codeGen

	// For interpreter, we want to employ optimizations, but we don't want to make construction significantly
	// more expensive. regexp2cg can opt into more expensive analysis with OptionIsCodeGen. So for the
	// interpreter we focus only on creating a set for the first character. Same for right-to-left, which
	// is used very rarely and thus we don't need to invest in special-casing it.
	if f.rightToLeft {
		// Determine a set for anything that can possibly start the expression.
		set := findFirstCharClass(root)
		if set != nil {
			var chars []rune
			if !set.IsNegated() {
				// See if the set is limited to holding only a few characters.
				chars = set.GetSetChars(5)
			}

			if !codeGen && len(chars) == 1 {
				// The set contains one and only one character, meaning every match starts
				// with the same literal value (potentially case-insensitive). Search for that.
				f.FixedDistanceLiteral.C = chars[0]
				f.FindMode = LeadingChar_RightToLeft
			} else {
				// The set may match multiple characters.  Search for that.
				f.FixedDistanceSets = []FixedDistanceSet{{
					Chars:    chars,
					Set:      set,
					Distance: 0,
				}}
				f.FindMode = LeadingSet_RightToLeft
				f.asciiLookups = make([][]uint, 1)
			}
		}
		return f
	}

	// We're now left-to-right only.

	prefix = findPrefixOrdinalCaseInsensitive(root)
	if len(prefix) > 1 {
		f.LeadingPrefix = prefix
		f.FindMode = LeadingString_OrdinalIgnoreCase_LeftToRight
		return f
	}

	// We're now left-to-right only and looking for multiple prefixes and/or sets.

	// If there are multiple leading strings, we can search for any of them.
	// this works in the interpreter, but we avoid it due to additional cost during construction

	if !interpreter {
		ciPrefixes := findPrefixes(root, true)
		if len(ciPrefixes) > 1 {
			f.LeadingPrefixes = ciPrefixes
			f.LeadingPrefixesRunes = toRunePrefixes(ciPrefixes)
			f.FindMode = LeadingStrings_OrdinalIgnoreCase_LeftToRight
			/*SYSTEM_TEXT_REGULAREXPRESSIONS
			if usesRfoTryFind {
						f.LeadingStrings = helpers.NewSearchValues(f.LeadingPrefixes, true)
			}*/
			return f
		}
	}

	// Build up a list of all of the sets that are a fixed distance from the start of the expression.
	fixedDistanceSets := findFixedDistanceSets(root, !interpreter)

	// See if we can make a string of at least two characters long out of those sets.  We should have already caught
	// one at the beginning of the pattern, but there may be one hiding at a non-zero fixed distance into the pattern.
	if len(fixedDistanceSets) > 0 {
		bestFixedDistanceString := findFixedDistanceString(fixedDistanceSets)
		if bestFixedDistanceString != nil {
			f.FindMode = FixedDistanceString_LeftToRight
			f.FixedDistanceLiteral = *bestFixedDistanceString
			return f
		}
	}

	// A landmark chain is more selective than a single leading set for shapes like
	// /name-separator host-separator domain/. Prefer it before falling back to
	// one-position or one-literal candidate searches.
	if landmarkChain := findRequiredLandmarkChain(root); landmarkChain != nil {
		f.FindMode = RequiredLandmarkChain_LeftToRight
		f.LandmarkChain = landmarkChain
		return f
	}

	// As a backup, see if we can find a literal after a leading atomic loop.  That might be better than whatever sets we find, so
	// we want to know whether we have one in our pocket before deciding whether to use a leading set (we'll prefer a leading
	// set if it's something for which we can search efficiently).
	literalAfterLoop := findLiteralFollowingLeadingLoop(root)

	// If we got such sets, we'll likely use them.  However, if the best of them is something that doesn't support an efficient
	// search and we did successfully find a literal after an atomic loop we could search instead, we prefer the efficient search.
	// For example, if we have a negated set, we will still prefer the literal-after-an-atomic-loop because negated sets typically
	// contain _many_ characters (e.g. [^a] is everything but 'a') and are thus more likely to very quickly match, which means any
	// vectorization employed is less likely to kick in and be worth the startup overhead.
	if len(fixedDistanceSets) > 0 {
		// Sort the sets by "quality", such that whatever set is first is the one deemed most efficient to use.
		// In some searches, we may use multiple sets, so we want the subsequent ones to also be the efficiency runners-up.
		slices.SortFunc(fixedDistanceSets, compareFixedDistanceSetsByQuality)

		// If the best fixed-distance set is composed of high-frequency characters, IndexOfAny on
		// those characters is likely to match too many positions. Prefer a case-sensitive
		// multi-prefix search when one is available.
		if !interpreter && !mayContainCaseInsensitiveMatching(root) && hasHighFrequencyChars(fixedDistanceSets[0]) {
			caseSensitivePrefixes := findPrefixes(root, false)
			if len(caseSensitivePrefixes) > 1 {
				f.LeadingPrefixes = caseSensitivePrefixes
				f.LeadingPrefixesRunes = toRunePrefixes(caseSensitivePrefixes)
				f.FindMode = LeadingStrings_LeftToRight
				return f
			}
		}

		// If there is no literal after the loop, use whatever set we got.
		// If there is a literal after the loop, consider it to be better than a negated set and better than a set with many characters.
		if literalAfterLoop == nil || (len(fixedDistanceSets[0].Chars) > 0 && !fixedDistanceSets[0].Negated) {
			// Determine whether to do searching based on one or more sets or on a single literal. Code-generated engines
			// don't need to special-case literals as they already do codegen to create the optimal lookup based on
			// the set's characteristics.
			if !codeGen && len(fixedDistanceSets) == 1 && len(fixedDistanceSets[0].Chars) == 1 &&
				!fixedDistanceSets[0].Negated {
				f.FixedDistanceLiteral = FixedDistanceLiteral{
					C:        fixedDistanceSets[0].Chars[0],
					Distance: fixedDistanceSets[0].Distance,
				}
				f.FindMode = FixedDistanceChar_LeftToRight
			} else {
				// Limit how many sets we use to avoid doing lots of unnecessary work.  The list was already
				// sorted from best to worst, so just keep the first ones up to our limit.
				const MaxSetsToUse = 3 // arbitrary tuned limit
				if len(fixedDistanceSets) > MaxSetsToUse {
					fixedDistanceSets = fixedDistanceSets[:MaxSetsToUse]
				}

				// Store the sets, and compute which mode to use.
				f.FixedDistanceSets = fixedDistanceSets
				if len(fixedDistanceSets) == 1 && fixedDistanceSets[0].Distance == 0 {
					f.FindMode = LeadingSet_LeftToRight
				} else {
					f.FindMode = FixedDistanceSets_LeftToRight
				}
				f.asciiLookups = make([][]uint, len(fixedDistanceSets))
			}
			return f
		}
	}

	// If we found a literal we can search for after a leading set loop, use it.
	if literalAfterLoop != nil {
		f.FindMode = LiteralAfterLoop_LeftToRight
		f.LiteralAfterLoop = literalAfterLoop
		f.asciiLookups = make([][]uint, 1)
	}

	return f
}

func (f *FindOptimizations) isUseful() bool {
	return f.FindMode != NoSearch || f.LeadingAnchor == NtBol
}

func toRunePrefixes(prefixes []string) [][]rune {
	if len(prefixes) == 0 {
		return nil
	}
	runes := make([][]rune, len(prefixes))
	for i, prefix := range prefixes {
		runes[i] = []rune(prefix)
	}
	return runes
}

func getFindMode(rtl bool, t NodeType) FindNextStartingPositionMode {
	if rtl {
		switch t {
		case NtBeginning:
			return LeadingAnchor_RightToLeft_Beginning
		case NtStart:
			return LeadingAnchor_RightToLeft_Start
		case NtEnd:
			return LeadingAnchor_RightToLeft_End
		case NtEndZ:
			return LeadingAnchor_RightToLeft_EndZ
		}
	} else {
		switch t {
		case NtBeginning:
			return LeadingAnchor_LeftToRight_Beginning
		case NtStart:
			return LeadingAnchor_LeftToRight_Start
		case NtEnd:
			return LeadingAnchor_LeftToRight_End
		case NtEndZ:
			return LeadingAnchor_LeftToRight_EndZ
		}
	}

	return NoSearch
}

// Analyzes a list of fixed-distance sets to extract a case-sensitive string at a fixed distance.</summary>
func findFixedDistanceString(fixedDistanceSets []FixedDistanceSet) *FixedDistanceLiteral {
	var best *FixedDistanceLiteral

	// A result string must be at least two characters in length; therefore we require at least that many sets.
	if len(fixedDistanceSets) >= 2 {
		// We're walking the sets from beginning to end, so we need them sorted by distance.
		slices.SortFunc(fixedDistanceSets, func(s1, s2 FixedDistanceSet) int {
			return cmp.Compare(s1.Distance, s2.Distance)
		})

		vsb := &bytes.Buffer{}

		// Looking for strings of length >= 2
		start := -1
		for i := 0; i < len(fixedDistanceSets)+1; i++ {
			chars := []rune(nil)
			if i < len(fixedDistanceSets) {
				chars = fixedDistanceSets[i].Chars
			}
			invalidChars := len(chars) != 1 || fixedDistanceSets[i].Negated

			// If the current set ends a sequence (or we've walked off the end), see whether
			// what we've gathered constitues a valid string, and if it's better than the
			// best we've already seen, store it.  Regardless, reset the sequence in order
			// to continue analyzing.
			if invalidChars || (i > 0 && fixedDistanceSets[i].Distance != fixedDistanceSets[i-1].Distance+1) {
				bestLen := 2
				if best != nil {
					bestLen = len(best.S)
				}
				if start != -1 && i-start >= bestLen {
					best = &FixedDistanceLiteral{
						S:        vsb.String(),
						Distance: fixedDistanceSets[start].Distance,
					}
				}

				vsb.Reset()
				start = -1
				if invalidChars {
					continue
				}
			}

			if start == -1 {
				start = i
			}

			vsb.WriteRune(chars[0])
		}
	}

	return best
}
