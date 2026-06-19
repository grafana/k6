package syntax

import (
	"bytes"
	"fmt"
	"strconv"
	"unicode"
	"unicode/utf8"
)

type Prefix struct {
	PrefixStr       []rune
	PrefixSet       CharSet
	CaseInsensitive bool
}

// It takes a RegexTree and computes the set of chars that can start it.
func getFirstCharsPrefix(tree *RegexTree) *Prefix {
	s := regexFcd{
		fcStack:  make([]regexFc, 32),
		intStack: make([]int, 32),
	}
	fc := s.regexFCFromRegexTree(tree)

	if fc == nil || fc.nullable || fc.cc.IsEmpty() {
		return nil
	}
	fcSet := fc.getFirstChars()
	return &Prefix{PrefixSet: fcSet, CaseInsensitive: fc.caseInsensitive}
}

type regexFcd struct {
	intStack        []int
	intDepth        int
	fcStack         []regexFc
	fcDepth         int
	skipAllChildren bool // don't process any more children at the current level
	skipchild       bool // don't process the current child.
	failed          bool
}

/*
 * The main FC computation. It does a shortcutted depth-first walk
 * through the tree and calls CalculateFC to emits code before
 * and after each child of an interior node, and at each leaf.
 */
func (s *regexFcd) regexFCFromRegexTree(tree *RegexTree) *regexFc {
	curNode := tree.Root
	curChild := 0

	for {
		if len(curNode.Children) == 0 {
			// This is a leaf node
			s.calculateFC(curNode.T, curNode, 0)
		} else if curChild < len(curNode.Children) && !s.skipAllChildren {
			// This is an interior node, and we have more children to analyze
			s.calculateFC(curNode.T|BeforeChild, curNode, curChild)

			if !s.skipchild {
				curNode = curNode.Children[curChild]
				// this stack is how we get a depth first walk of the tree.
				s.pushInt(curChild)
				curChild = 0
			} else {
				curChild++
				s.skipchild = false
			}
			continue
		}

		// This is an interior node where we've finished analyzing all the children, or
		// the end of a leaf node.
		s.skipAllChildren = false

		if s.intIsEmpty() {
			break
		}

		curChild = s.popInt()
		curNode = curNode.Parent

		s.calculateFC(curNode.T|AfterChild, curNode, curChild)
		if s.failed {
			return nil
		}

		curChild++
	}

	if s.fcIsEmpty() {
		return nil
	}

	return s.popFC()
}

// To avoid recursion, we use a simple integer stack.
// This is the push.
func (s *regexFcd) pushInt(I int) {
	if s.intDepth >= len(s.intStack) {
		expanded := make([]int, s.intDepth*2)
		copy(expanded, s.intStack)
		s.intStack = expanded
	}

	s.intStack[s.intDepth] = I
	s.intDepth++
}

// True if the stack is empty.
func (s *regexFcd) intIsEmpty() bool {
	return s.intDepth == 0
}

// This is the pop.
func (s *regexFcd) popInt() int {
	s.intDepth--
	return s.intStack[s.intDepth]
}

// We also use a stack of RegexFC objects.
// This is the push.
func (s *regexFcd) pushFC(fc regexFc) {
	if s.fcDepth >= len(s.fcStack) {
		expanded := make([]regexFc, s.fcDepth*2)
		copy(expanded, s.fcStack)
		s.fcStack = expanded
	}

	s.fcStack[s.fcDepth] = fc
	s.fcDepth++
}

// True if the stack is empty.
func (s *regexFcd) fcIsEmpty() bool {
	return s.fcDepth == 0
}

// This is the pop.
func (s *regexFcd) popFC() *regexFc {
	s.fcDepth--
	return &s.fcStack[s.fcDepth]
}

// This is the top.
func (s *regexFcd) topFC() *regexFc {
	return &s.fcStack[s.fcDepth-1]
}

// Called in Beforechild to prevent further processing of the current child
func (s *regexFcd) skipChild() {
	s.skipchild = true
}

// FC computation and shortcut cases for each node type
func (s *regexFcd) calculateFC(nt NodeType, node *RegexNode, CurIndex int) {
	//fmt.Printf("NodeType: %v, CurIndex: %v, Desc: %v\n", nt, CurIndex, node.description())
	ci := false
	rtl := false

	if nt <= NtRef {
		if (node.Options & IgnoreCase) != 0 {
			ci = true
		}
		if (node.Options & RightToLeft) != 0 {
			rtl = true
		}
	}

	switch nt {
	case NtConcatenate | BeforeChild, NtAlternate | BeforeChild, NtBackRefCond | BeforeChild, NtLoop | BeforeChild, NtLazyloop | BeforeChild:

	case NtExprCond | BeforeChild:
		if CurIndex == 0 {
			s.skipChild()
		}

	case NtEmpty:
		s.pushFC(regexFc{nullable: true})

	case NtConcatenate | AfterChild:
		if CurIndex != 0 {
			child := s.popFC()
			cumul := s.topFC()

			s.failed = !cumul.addFC(*child, true)
		}

		fc := s.topFC()
		if !fc.nullable {
			s.skipAllChildren = true
		}

	case NtExprCond | AfterChild:
		if CurIndex > 1 {
			child := s.popFC()
			cumul := s.topFC()

			s.failed = !cumul.addFC(*child, false)
		}

	case NtAlternate | AfterChild, NtBackRefCond | AfterChild:
		if CurIndex != 0 {
			child := s.popFC()
			cumul := s.topFC()

			s.failed = !cumul.addFC(*child, false)
		}

	case NtLoop | AfterChild, NtLazyloop | AfterChild:
		if node.M == 0 {
			fc := s.topFC()
			fc.nullable = true
		}

	case NtGroup | BeforeChild, NtGroup | AfterChild, NtCapture | BeforeChild, NtCapture | AfterChild, NtAtomic | BeforeChild, NtAtomic | AfterChild:

	case NtPosLook | BeforeChild, NtNegLook | BeforeChild:
		s.skipChild()
		s.pushFC(regexFc{nullable: true})

	case NtPosLook | AfterChild, NtNegLook | AfterChild:

	case NtOne, NtNotone:
		s.pushFC(newRegexFc(node.Ch, nt == NtNotone, false, ci))

	case NtOneloop, NtOnelazy, NtOneloopatomic:
		s.pushFC(newRegexFc(node.Ch, false, node.M == 0, ci))

	case NtNotoneloop, NtNotonelazy, NtNotoneloopatomic:
		s.pushFC(newRegexFc(node.Ch, true, node.M == 0, ci))

	case NtMulti:
		if len(node.Str) == 0 {
			s.pushFC(regexFc{nullable: true})
		} else if !rtl {
			s.pushFC(newRegexFc(node.Str[0], false, false, ci))
		} else {
			s.pushFC(newRegexFc(node.Str[len(node.Str)-1], false, false, ci))
		}

	case NtSet:
		s.pushFC(regexFc{cc: node.Set.Copy(), nullable: false, caseInsensitive: ci})

	case NtSetloop, NtSetlazy, NtSetloopatomic:
		s.pushFC(regexFc{cc: node.Set.Copy(), nullable: node.M == 0, caseInsensitive: ci})

	case NtRef:
		s.pushFC(regexFc{cc: *AnyClass(), nullable: true, caseInsensitive: false})

	case NtNothing, NtBol, NtEol, NtBoundary, NtNonboundary, NtECMABoundary, NtNonECMABoundary, NtBeginning, NtStart, NtEndZ, NtEnd, NtUpdateBumpalong:
		s.pushFC(regexFc{nullable: true})

	default:
		panic(fmt.Sprintf("unexpected op code: %v", nt))
	}
}

type regexFc struct {
	cc              CharSet
	nullable        bool
	caseInsensitive bool
}

func newRegexFc(ch rune, not, nullable, caseInsensitive bool) regexFc {
	r := regexFc{
		caseInsensitive: caseInsensitive,
		nullable:        nullable,
	}
	if not {
		if ch > 0 {
			r.cc.addRange('\x00', ch-1)
		}
		if ch < 0xFFFF {
			r.cc.addRange(ch+1, utf8.MaxRune)
		}
	} else {
		r.cc.addRange(ch, ch)
	}
	return r
}

func (r *regexFc) getFirstChars() CharSet {
	if r.caseInsensitive {
		r.cc.addLowercase()
	}

	return r.cc
}

func (r *regexFc) addFC(fc regexFc, concatenate bool) bool {
	if !r.cc.IsMergeable() || !fc.cc.IsMergeable() {
		return false
	}

	if concatenate {
		if !r.nullable {
			return true
		}

		if !fc.nullable {
			r.nullable = false
		}
	} else {
		if fc.nullable {
			r.nullable = true
		}
	}

	r.caseInsensitive = r.caseInsensitive || fc.caseInsensitive
	r.cc.addSet(fc.cc)

	return true
}

// This is a related computation: it takes a RegexTree and computes the
// leading substring if it sees one. It's quite trivial and gives up easily.
func getPrefix(tree *RegexTree) *Prefix {
	var concatNode *RegexNode
	nextChild := 0

	curNode := tree.Root

	for {
		switch curNode.T {
		case NtConcatenate:
			if len(curNode.Children) > 0 {
				concatNode = curNode
				nextChild = 0
			}

		case NtAtomic, NtCapture:
			curNode = curNode.Children[0]
			concatNode = nil
			continue

		case NtOneloop, NtOnelazy:
			if curNode.M > 0 {
				return &Prefix{
					PrefixStr:       repeat(curNode.Ch, curNode.M),
					CaseInsensitive: (curNode.Options & IgnoreCase) != 0,
				}
			}
			return nil

		case NtOne:
			return &Prefix{
				PrefixStr:       []rune{curNode.Ch},
				CaseInsensitive: (curNode.Options & IgnoreCase) != 0,
			}

		case NtMulti:
			return &Prefix{
				PrefixStr:       curNode.Str,
				CaseInsensitive: (curNode.Options & IgnoreCase) != 0,
			}

		case NtBol, NtEol, NtBoundary, NtECMABoundary, NtBeginning, NtStart,
			NtEndZ, NtEnd, NtEmpty, NtPosLook, NtNegLook:

		default:
			return nil
		}

		if concatNode == nil || nextChild >= len(concatNode.Children) {
			return nil
		}

		curNode = concatNode.Children[nextChild]
		nextChild++
	}
}

// repeat the rune r, c times... up to the max of MaxPrefixSize
func repeat(r rune, c int) []rune {
	if c > MaxPrefixSize {
		c = MaxPrefixSize
	}

	ret := make([]rune, c)

	// binary growth using copy for speed
	ret[0] = r
	bp := 1
	for bp < len(ret) {
		copy(ret[bp:], ret[:bp])
		bp *= 2
	}

	return ret
}

// BmPrefix precomputes the Boyer-Moore
// tables for fast string scanning. These tables allow
// you to scan for the first occurrence of a string within
// a large body of text without examining every character.
// The performance of the heuristic depends on the actual
// string and the text being searched, but usually, the longer
// the string that is being searched for, the fewer characters
// need to be examined.
type BmPrefix struct {
	positive        []int
	negativeASCII   []int
	negativeUnicode [][]int
	pattern         []rune
	lowASCII        rune
	highASCII       rune
	rightToLeft     bool
	caseInsensitive bool
}

func newBmPrefix(pattern []rune, caseInsensitive, rightToLeft bool) *BmPrefix {

	b := &BmPrefix{
		rightToLeft:     rightToLeft,
		caseInsensitive: caseInsensitive,
		pattern:         pattern,
	}

	if caseInsensitive {
		for i := 0; i < len(b.pattern); i++ {
			// We do the ToLower character by character for consistency.  With surrogate chars, doing
			// a ToLower on the entire string could actually change the surrogate pair.  This is more correct
			// linguistically, but since Regex doesn't support surrogates, it's more important to be
			// consistent.

			b.pattern[i] = unicode.ToLower(b.pattern[i])
		}
	}

	var beforefirst, last, bump int
	var scan, match int

	if !rightToLeft {
		beforefirst = -1
		last = len(b.pattern) - 1
		bump = 1
	} else {
		beforefirst = len(b.pattern)
		last = 0
		bump = -1
	}

	// PART I - the good-suffix shift table
	//
	// compute the positive requirement:
	// if char "i" is the first one from the right that doesn't match,
	// then we know the matcher can advance by _positive[i].
	//
	// This algorithm is a simplified variant of the standard
	// Boyer-Moore good suffix calculation.

	b.positive = make([]int, len(b.pattern))

	examine := last
	ch := b.pattern[examine]
	b.positive[examine] = bump
	examine -= bump

Outerloop:
	for {
		// find an internal char (examine) that matches the tail

		for {
			if examine == beforefirst {
				break Outerloop
			}
			if b.pattern[examine] == ch {
				break
			}
			examine -= bump
		}

		match = last
		scan = examine

		// find the length of the match
		for {
			if scan == beforefirst || b.pattern[match] != b.pattern[scan] {
				// at the end of the match, note the difference in _positive
				// this is not the length of the match, but the distance from the internal match
				// to the tail suffix.
				if b.positive[match] == 0 {
					b.positive[match] = match - scan
				}

				// System.Diagnostics.Debug.WriteLine("Set positive[" + match + "] to " + (match - scan));

				break
			}

			scan -= bump
			match -= bump
		}

		examine -= bump
	}

	match = last - bump

	// scan for the chars for which there are no shifts that yield a different candidate

	// The inside of the if statement used to say
	// "_positive[match] = last - beforefirst;"
	// This is slightly less aggressive in how much we skip, but at worst it
	// should mean a little more work rather than skipping a potential match.
	for match != beforefirst {
		if b.positive[match] == 0 {
			b.positive[match] = bump
		}

		match -= bump
	}

	// PART II - the bad-character shift table
	//
	// compute the negative requirement:
	// if char "ch" is the reject character when testing position "i",
	// we can slide up by _negative[ch];
	// (_negative[ch] = str.Length - 1 - str.LastIndexOf(ch))
	//
	// the lookup table is divided into ASCII and Unicode portions;
	// only those parts of the Unicode 16-bit code set that actually
	// appear in the string are in the table. (Maximum size with
	// Unicode is 65K; ASCII only case is 512 bytes.)

	b.negativeASCII = make([]int, 128)

	for i := 0; i < len(b.negativeASCII); i++ {
		b.negativeASCII[i] = last - beforefirst
	}

	b.lowASCII = 127
	b.highASCII = 0

	for examine = last; examine != beforefirst; examine -= bump {
		ch = b.pattern[examine]

		switch {
		case ch < 128:
			if b.lowASCII > ch {
				b.lowASCII = ch
			}

			if b.highASCII < ch {
				b.highASCII = ch
			}

			if b.negativeASCII[ch] == last-beforefirst {
				b.negativeASCII[ch] = last - examine
			}
		case ch <= 0xffff:
			i, j := ch>>8, ch&0xFF

			if b.negativeUnicode == nil {
				b.negativeUnicode = make([][]int, 256)
			}

			if b.negativeUnicode[i] == nil {
				newarray := make([]int, 256)

				for k := 0; k < len(newarray); k++ {
					newarray[k] = last - beforefirst
				}

				if i == 0 {
					copy(newarray, b.negativeASCII)
					//TODO: this line needed?
					b.negativeASCII = newarray
				}

				b.negativeUnicode[i] = newarray
			}

			if b.negativeUnicode[i][j] == last-beforefirst {
				b.negativeUnicode[i][j] = last - examine
			}
		default:
			// we can't do the filter because this algo doesn't support
			// unicode chars >0xffff
			return nil
		}
	}

	return b
}

func (b *BmPrefix) String() string {
	return string(b.pattern)
}

// Dump returns the contents of the filter as a human readable string
func (b *BmPrefix) Dump(indent string) string {
	buf := &bytes.Buffer{}

	fmt.Fprintf(buf, "%sBM Pattern: %s\n%sPositive: ", indent, string(b.pattern), indent)
	for i := 0; i < len(b.positive); i++ {
		buf.WriteString(strconv.Itoa(b.positive[i]))
		buf.WriteRune(' ')
	}
	buf.WriteRune('\n')

	if b.negativeASCII != nil {
		buf.WriteString(indent)
		buf.WriteString("Negative table\n")
		for i := 0; i < len(b.negativeASCII); i++ {
			if b.negativeASCII[i] != len(b.pattern) {
				fmt.Fprintf(buf, "%s  %s %s\n", indent, Escape(string(rune(i))), strconv.Itoa(b.negativeASCII[i]))
			}
		}
	}

	return buf.String()
}

// Scan uses the Boyer-Moore algorithm to find the first occurrence
// of the specified string within text, beginning at index, and
// constrained within beglimit and endlimit.
//
// The direction and case-sensitivity of the match is determined
// by the arguments to the RegexBoyerMoore constructor.
func (b *BmPrefix) Scan(text []rune, index, beglimit, endlimit int) int {
	var (
		defadv, test, test2         int
		match, startmatch, endmatch int
		bump, advance               int
		chTest                      rune
		unicodeLookup               []int
	)

	if !b.rightToLeft {
		defadv = len(b.pattern)
		startmatch = len(b.pattern) - 1
		endmatch = 0
		test = index + defadv - 1
		bump = 1
	} else {
		defadv = -len(b.pattern)
		startmatch = 0
		endmatch = -defadv - 1
		test = index + defadv
		bump = -1
	}

	chMatch := b.pattern[startmatch]

	for {
		if test >= endlimit || test < beglimit {
			return -1
		}

		chTest = text[test]

		if b.caseInsensitive {
			chTest = unicode.ToLower(chTest)
		}

		if chTest != chMatch {
			if chTest < 128 {
				advance = b.negativeASCII[chTest]
			} else if chTest < 0xffff && len(b.negativeUnicode) > 0 {
				unicodeLookup = b.negativeUnicode[chTest>>8]
				if len(unicodeLookup) > 0 {
					advance = unicodeLookup[chTest&0xFF]
				} else {
					advance = defadv
				}
			} else {
				advance = defadv
			}

			test += advance
		} else { // if (chTest == chMatch)
			test2 = test
			match = startmatch

			for {
				if match == endmatch {
					if b.rightToLeft {
						return test2 + 1
					} else {
						return test2
					}
				}

				match -= bump
				test2 -= bump

				chTest = text[test2]

				if b.caseInsensitive {
					chTest = unicode.ToLower(chTest)
				}

				if chTest != b.pattern[match] {
					advance = b.positive[match]
					if chTest < 128 {
						test2 = (match - startmatch) + b.negativeASCII[chTest]
					} else if chTest < 0xffff && len(b.negativeUnicode) > 0 {
						unicodeLookup = b.negativeUnicode[chTest>>8]
						if len(unicodeLookup) > 0 {
							test2 = (match - startmatch) + unicodeLookup[chTest&0xFF]
						} else {
							test += advance
							break
						}
					} else {
						test += advance
						break
					}

					if b.rightToLeft {
						if test2 < advance {
							advance = test2
						}
					} else if test2 > advance {
						advance = test2
					}

					test += advance
					break
				}
			}
		}
	}
}

// When a regex is anchored, we can do a quick IsMatch test instead of a Scan
func (b *BmPrefix) IsMatch(text []rune, index, beglimit, endlimit int) bool {
	if !b.rightToLeft {
		if index < beglimit || endlimit-index < len(b.pattern) {
			return false
		}

		return b.matchPattern(text, index)
	} else {
		if index > endlimit || index-beglimit < len(b.pattern) {
			return false
		}

		return b.matchPattern(text, index-len(b.pattern))
	}
}

func (b *BmPrefix) matchPattern(text []rune, index int) bool {
	if len(text)-index < len(b.pattern) {
		return false
	}

	if b.caseInsensitive {
		for i := 0; i < len(b.pattern); i++ {
			//Debug.Assert(textinfo.ToLower(_pattern[i]) == _pattern[i], "pattern should be converted to lower case in constructor!");
			if unicode.ToLower(text[index+i]) != b.pattern[i] {
				return false
			}
		}
		return true
	} else {
		for i := 0; i < len(b.pattern); i++ {
			if text[index+i] != b.pattern[i] {
				return false
			}
		}
		return true
	}
}

type AnchorLoc int16

// where the regex can be pegged
const (
	AnchorBeginning    AnchorLoc = 0x0001
	AnchorBol          AnchorLoc = 0x0002
	AnchorStart        AnchorLoc = 0x0004
	AnchorEol          AnchorLoc = 0x0008
	AnchorEndZ         AnchorLoc = 0x0010
	AnchorEnd          AnchorLoc = 0x0020
	AnchorBoundary     AnchorLoc = 0x0040
	AnchorECMABoundary AnchorLoc = 0x0080
)

func getAnchors(tree *RegexTree) AnchorLoc {

	var concatNode *RegexNode
	nextChild, result := 0, AnchorLoc(0)

	curNode := tree.Root

	for {
		switch curNode.T {
		case NtConcatenate:
			if len(curNode.Children) > 0 {
				concatNode = curNode
				nextChild = 0
			}

		case NtAtomic, NtCapture:
			curNode = curNode.Children[0]
			concatNode = nil
			continue

		case NtBol, NtEol, NtBoundary, NtECMABoundary, NtBeginning,
			NtStart, NtEndZ, NtEnd:
			return result | anchorFromType(curNode.T)

		case NtEmpty, NtPosLook, NtNegLook:

		default:
			return result
		}

		if concatNode == nil || nextChild >= len(concatNode.Children) {
			return result
		}

		curNode = concatNode.Children[nextChild]
		nextChild++
	}
}

func anchorFromType(t NodeType) AnchorLoc {
	switch t {
	case NtBol:
		return AnchorBol
	case NtEol:
		return AnchorEol
	case NtBoundary:
		return AnchorBoundary
	case NtECMABoundary:
		return AnchorECMABoundary
	case NtBeginning:
		return AnchorBeginning
	case NtStart:
		return AnchorStart
	case NtEndZ:
		return AnchorEndZ
	case NtEnd:
		return AnchorEnd
	default:
		return 0
	}
}

// anchorDescription returns a human-readable description of the anchors
func (anchors AnchorLoc) String() string {
	buf := &bytes.Buffer{}

	if (anchors & AnchorBeginning) != 0 {
		buf.WriteString(", Beginning")
	}
	if (anchors & AnchorStart) != 0 {
		buf.WriteString(", Start")
	}
	if (anchors & AnchorBol) != 0 {
		buf.WriteString(", Bol")
	}
	if (anchors & AnchorBoundary) != 0 {
		buf.WriteString(", Boundary")
	}
	if (anchors & AnchorECMABoundary) != 0 {
		buf.WriteString(", ECMABoundary")
	}
	if (anchors & AnchorEol) != 0 {
		buf.WriteString(", Eol")
	}
	if (anchors & AnchorEnd) != 0 {
		buf.WriteString(", End")
	}
	if (anchors & AnchorEndZ) != 0 {
		buf.WriteString(", EndZ")
	}

	// trim off comma
	if buf.Len() >= 2 {
		return buf.String()[2:]
	}
	return "None"
}

func findLeadingOrTrailingAnchor(node *RegexNode, leading bool) NodeType {
	for {
		switch node.T {
		case NtBol, NtEol, NtBeginning, NtStart, NtEndZ, NtEnd, NtBoundary, NtECMABoundary:
			// anchor found
			return node.T
		case NtAtomic, NtCapture:
			// For groups, continue exploring the sole child.
			node = node.Children[0]
			continue
		case NtConcatenate:
			// For concatenations, we expect primarily to explore its first (for leading) or last (for trailing) child,
			// but we can also skip over certain kinds of nodes (e.g. Empty), and thus iterate through its children backward
			// looking for the last we shouldn't skip.
			var child *RegexNode

			if leading {
				for i := 0; i < len(node.Children); i++ {
					t := node.Children[i].T
					if t != NtEmpty && t != NtPosLook && t != NtNegLook {
						child = node.Children[i]
						break
					}
				}
			} else {
				for i := len(node.Children) - 1; i >= 0; i-- {
					t := node.Children[i].T
					if t != NtEmpty && t != NtPosLook && t != NtNegLook {
						child = node.Children[i]
						break
					}
				}
			}
			if child != nil {
				node = child
				continue
			}

		case NtAlternate:
			// For alternations, every branch needs to lead or trail with the same anchor.

			// Get the leading/trailing anchor of the first branch.  If there isn't one, bail.
			anchor := findLeadingOrTrailingAnchor(node.Children[0], leading)
			if anchor == NtUnknown {
				return NtUnknown
			}

			// Look at each subsequent branch and validate it has the same leading or trailing
			// anchor.  If any doesn't, bail.
			for i := 1; i < len(node.Children); i++ {
				if findLeadingOrTrailingAnchor(node.Children[i], leading) != anchor {
					return NtUnknown
				}
			}

			// All branches have the same leading/trailing anchor.  Return it.
			return anchor

		}

		// no anchor
		return NtUnknown
	}
}
