package syntax

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"slices"
)

// Arbitrary number of repetitions of the same character when we'd prefer to represent that as a repeater of that character rather than a string.
const MultiVsRepeaterLimit = 64

type RegexTree struct {
	Root              *RegexNode
	Caps              map[int]int
	Capnumlist        []int
	Captop            int
	Capnames          map[string]int
	Caplist           []string
	Options           RegexOptions
	FindOptimizations *FindOptimizations
}

// It is built into a parsed tree for a regular expression.

// Implementation notes:
//
// Since the node tree is a temporary data structure only used
// during compilation of the regexp to integer codes, it's
// designed for clarity and convenience rather than
// space efficiency.
//
// RegexNodes are built into a tree, linked by the n.children list.
// Each node also has a n.parent and n.ichild member indicating
// its parent and which child # it is in its parent's list.
//
// RegexNodes come in as many types as there are constructs in
// a regular expression, for example, "concatenate", "alternate",
// "one", "rept", "group". There are also node types for basic
// peephole optimizations, e.g., "onerep", "notsetrep", etc.
//
// Because perl 5 allows "lookback" groups that scan backwards,
// each node also gets a "direction". Normally the value of
// boolean n.backward = false.
//
// On the parse stack, each tree has a "role" - basically, the
// nonterminal in the grammar that the parser has currently
// assigned to the tree. That code is stored in n.role.
//
// Finally, some of the different kinds of nodes have data.
// Two integers (for the looping constructs) are stored in
// n.operands, an an object (either a string or a set)
// is stored in n.data
type RegexNode struct {
	T        NodeType
	Children []*RegexNode
	Str      []rune
	Set      *CharSet
	Ch       rune
	M        int
	N        int
	Options  RegexOptions
	Parent   *RegexNode
}

type NodeType int32

const (
	// The following are leaves, and correspond to primitive operations
	NtUnknown NodeType = -1
	//NtOnerep      NodeType = 0  // lef,back char,min,max    a {n}
	//NtNotonerep   NodeType = 1  // lef,back char,min,max    .{n}
	//NtSetrep      NodeType = 2  // lef,back set,min,max     [\d]{n}
	NtOneloop     NodeType = 3  // lef,back char,min,max    a {,n}
	NtNotoneloop  NodeType = 4  // lef,back char,min,max    .{,n}
	NtSetloop     NodeType = 5  // lef,back set,min,max     [\d]{,n}
	NtOnelazy     NodeType = 6  // lef,back char,min,max    a {,n}?
	NtNotonelazy  NodeType = 7  // lef,back char,min,max    .{,n}?
	NtSetlazy     NodeType = 8  // lef,back set,min,max     [\d]{,n}?
	NtOne         NodeType = 9  // lef      char            a
	NtNotone      NodeType = 10 // lef      char            [^a]
	NtSet         NodeType = 11 // lef      set             [a-z\s]  \w \s \d
	NtMulti       NodeType = 12 // lef      string          abcd
	NtRef         NodeType = 13 // lef      group           \#
	NtBol         NodeType = 14 //                          ^
	NtEol         NodeType = 15 //                          $
	NtBoundary    NodeType = 16 //                          \b
	NtNonboundary NodeType = 17 //                          \B
	NtBeginning   NodeType = 18 //                          \A
	NtStart       NodeType = 19 //                          \G
	NtEndZ        NodeType = 20 //                          \Z
	NtEnd         NodeType = 21 //                          \Z

	// Interior nodes do not correspond to primitive operations, but
	// control structures compositing other operations

	// Concat and alternate take n children, and can run forward or backwards

	NtNothing     NodeType = 22 //          []
	NtEmpty       NodeType = 23 //          ()
	NtAlternate   NodeType = 24 //          a|b
	NtConcatenate NodeType = 25 //          ab
	NtLoop        NodeType = 26 // m,x      * + ? {,}
	NtLazyloop    NodeType = 27 // m,x      *? +? ?? {,}?
	NtCapture     NodeType = 28 // n        ()
	NtGroup       NodeType = 29 //          (?:)
	NtPosLook     NodeType = 30 //          (?=) (?<=)
	NtNegLook     NodeType = 31 //          (?!) (?<!)
	NtAtomic      NodeType = 32 //          (?>) (?<)
	NtBackRefCond NodeType = 33 //          (?(n) | )
	NtExprCond    NodeType = 34 //          (?(...) | )

	NtECMABoundary    NodeType = 41 //                          \b
	NtNonECMABoundary NodeType = 42 //                          \B

	// Atomic loop of the specified character.
	// Operand 0 is the character. Operand 1 is the max iteration count.
	NtOneloopatomic NodeType = 43
	// Atomic loop of a single character other than the one specified.
	// Operand 0 is the character. Operand 1 is the max iteration count.
	NtNotoneloopatomic NodeType = 44
	// Atomic loop of a single character matching the specified set
	// Operand 0 is index into the strings table of the character class description. Operand 1 is the repetition count.
	NtSetloopatomic NodeType = 45
	// Updates the bumpalong position to the current position.
	NtUpdateBumpalong NodeType = 46
)

func newRegexNode(t NodeType, opt RegexOptions) *RegexNode {
	return &RegexNode{
		T:       t,
		Options: opt,
	}
}

func newRegexNodeCh(t NodeType, opt RegexOptions, ch rune) *RegexNode {
	return nodeWithCaseConversion(&RegexNode{
		T:       t,
		Options: opt,
		Ch:      ch,
	})
}

func newRegexNodeStr(t NodeType, opt RegexOptions, str []rune) *RegexNode {
	return &RegexNode{
		T:       t,
		Options: opt,
		Str:     str,
	}
}

func newRegexNodeSet(t NodeType, opt RegexOptions, set *CharSet) *RegexNode {
	return nodeWithCaseConversion(&RegexNode{
		T:       t,
		Options: opt,
		Set:     set,
	})
}

func newRegexNodeM(t NodeType, opt RegexOptions, m int) *RegexNode {
	return &RegexNode{
		T:       t,
		Options: opt,
		M:       m,
	}
}
func newRegexNodeMN(t NodeType, opt RegexOptions, m, n int) *RegexNode {
	return &RegexNode{
		T:       t,
		Options: opt,
		M:       m,
		N:       n,
	}
}

func nodeWithCaseConversion(n *RegexNode) *RegexNode {
	// if opts are ignore case and our rune is impacted by casing
	// then we need to switch our type to the set version
	// NtOne = NtSet
	if n.Options&IgnoreCase == 0 {
		return n
	}

	if n.Ch > 0 {
		ch := n.Ch
		if isLow, isUp := unicode.IsLower(ch), unicode.IsUpper(ch); isLow || isUp {
			/*var upper, lower rune
			// it's a capitalizable char
			if isUp {
				upper = ch
				lower = unicode.ToLower(ch)
			} else {
				lower = ch
				upper = unicode.ToUpper(ch)
			}*/
			set := &CharSet{}
			set.addChar(ch)
			set.addCaseEquivalences()
			t := NtSet

			switch n.T {
			case NtOneloop, NtNotoneloop:
				t = NtSetloop
			case NtOnelazy, NtNotonelazy:
				t = NtSetlazy
			}
			set.negate = n.IsNotoneFamily()

			return &RegexNode{
				T:       t,
				Options: n.Options & ^IgnoreCase,
				Set:     set,
			}
		}
	} else if n.Set != nil {
		// just to be safe we don't modify the original set pointer
		// just in case it's used in a case-sensitive area
		s := n.Set.Copy()
		s.addCaseEquivalences()
		n.Set = &s
		n.Options &= ^IgnoreCase
	}

	return n
}

func (n *RegexNode) IsSetFamily() bool {
	return n.T == NtSet || n.T == NtSetloop || n.T == NtSetlazy || n.T == NtSetloopatomic
}
func (n *RegexNode) IsOneFamily() bool {
	return n.T == NtOne || n.T == NtOneloop || n.T == NtOnelazy || n.T == NtOneloopatomic
}
func (n *RegexNode) IsNotoneFamily() bool {
	return n.T == NtNotone || n.T == NtNotoneloop || n.T == NtNotonelazy || n.T == NtNotoneloopatomic
}

func (n *RegexNode) IsSetloopFamily() bool {
	return n.T == NtSetloop || n.T == NtSetlazy || n.T == NtSetloopatomic
}
func (n *RegexNode) IsOneloopFamily() bool {
	return n.T == NtOneloop || n.T == NtOnelazy || n.T == NtOneloopatomic
}
func (n *RegexNode) IsNotoneloopFamily() bool {
	return n.T == NtNotoneloop || n.T == NtNotonelazy || n.T == NtNotoneloopatomic
}

func (n *RegexNode) IsAtomicloopFamily() bool {
	return n.T == NtOneloopatomic || n.T == NtNotoneloopatomic || n.T == NtSetloopatomic
}

func (n *RegexNode) writeStrToBuf(buf *bytes.Buffer) {
	for i := 0; i < len(n.Str); i++ {
		buf.WriteRune(n.Str[i])
	}
}

func (n *RegexNode) addChild(child *RegexNode) {
	child.Parent = n
	reduced := child.reduce()
	reduced.Parent = n
	n.Children = append(n.Children, reduced)
	reduced.Parent = n
}

func (n *RegexNode) insertChildren(afterIndex int, nodes []*RegexNode) {
	for _, c := range nodes {
		c.Parent = n
	}
	n.Children = slices.Insert(n.Children, afterIndex, nodes...)
	//newChildren := make([]*RegexNode, 0, len(n.Children)+len(nodes))
	//n.Children = append(append(append(newChildren, n.Children[:afterIndex]...), nodes...), n.Children[afterIndex:]...)
}

// removes children including the start but not the end index
func (n *RegexNode) removeChildren(startIndex, endIndex int) {
	n.Children = append(n.Children[:startIndex], n.Children[endIndex:]...)
}

func (n *RegexNode) ReplaceChild(index int, newChild *RegexNode) {
	newChild.Parent = n // so that the child can see its parent while being reduced
	newChild = newChild.reduce()
	newChild.Parent = n // in case Reduce returns a different node that needs to be reparented

	n.Children[index] = newChild
}

// Pass type as OneLazy or OneLoop
func (n *RegexNode) makeRep(t NodeType, min, max int) {
	n.T += (t - NtOne)
	n.M = min
	n.N = max
}

// Performs additional optimizations on an entire tree prior to being used.
//
// Some optimizations are performed by the parser while parsing, and others are performed
// as nodes are being added to the tree.  The optimizations here expect the tree to be fully
// formed, as they inspect relationships between nodes that may not have been in place as
// individual nodes were being processed/added to the tree.
func (n *RegexNode) finalOptimize() *RegexNode {
	rootNode := n

	// Only apply optimization when LTR to avoid needing additional code for the much rarer RTL case.
	// Also only apply these optimizations when not using NonBacktracking, as these optimizations are
	// all about avoiding things that are impactful for the backtracking engines but nops for non-backtracking.
	if n.Options&RightToLeft == 0 {
		// Optimization: eliminate backtracking for loops.
		// For any single-character loop (Oneloop, Notoneloop, Setloop), see if we can automatically convert
		// that into its atomic counterpart (Oneloopatomic, Notoneloopatomic, Setloopatomic) based on what
		// comes after it in the expression tree.
		rootNode.findAndMakeLoopsAtomic()

		// Optimization: backtracking removal at expression end.
		// If we find backtracking construct at the end of the regex, we can instead make it non-backtracking,
		// since nothing would ever backtrack into it anyway.  Doing this then makes the construct available
		// to implementations that don't support backtracking.
		rootNode.eliminateEndingBacktracking()

		// Optimization: unnecessary re-processing of starting loops.
		// If an expression is guaranteed to begin with a single-character unbounded loop that isn't part of an alternation (in which case it
		// wouldn't be guaranteed to be at the beginning) or a capture (in which case a back reference could be influenced by its length), then we
		// can update the tree with a temporary node to indicate that the implementation should use that node's ending position in the input text
		// as the next starting position at which to start the next match. This avoids redoing matches we've already performed, e.g. matching
		// "\w+@dot.net" against "is this a valid address@dot.net", the \w+ will initially match the "is" and then will fail to match the "@".
		// Rather than bumping the scan loop by 1 and trying again to match at the "s", we can instead start at the " ".  For functional correctness
		// we can only consider unbounded loops, as to be able to start at the end of the loop we need the loop to have consumed all possible matches;
		// otherwise, you could end up with a pattern like "a{1,3}b" matching against "aaaabc", which should match, but if we pre-emptively stop consuming
		// after the first three a's and re-start from that position, we'll end up failing the match even though it should have succeeded.  We can also
		// apply this optimization to non-atomic loops: even though backtracking could be necessary, such backtracking would be handled within the processing
		// of a single starting position.  Lazy loops similarly benefit, as a failed match will result in exploring the exact same search space as with
		// a greedy loop, just in the opposite order (and a successful match will overwrite the bumpalong position); we need to avoid atomic lazy loops,
		// however, as they will only end up as a repeater for the minimum length and thus will effectively end up with a non-infinite upper bound, which
		// we've already outlined is problematic.
		node := rootNode.Children[0] // skip implicit root capture node
		atomicByAncestry := true     // the root is implicitly atomic because nothing comes after it (same for the implicit root capture)
		for {
			if node.T == NtAtomic {
				node = node.Children[0]
				continue
			} else if node.T == NtConcatenate {
				atomicByAncestry = false
				node = node.Children[0]
				continue
			} else if node.N == math.MaxInt32 &&
				((node.T == NtOneloop || node.T == NtOneloopatomic || node.T == NtNotoneloop || node.T == NtNotoneloopatomic || node.T == NtSetloop || node.T == NtSetloopatomic) ||
					((node.T == NtOnelazy || node.T == NtNotonelazy || node.T == NtSetlazy) && !atomicByAncestry)) {

				if node.Parent != nil && node.Parent.T == NtConcatenate {
					node.Parent.Children = slices.Insert(node.Parent.Children, 1, &RegexNode{T: NtUpdateBumpalong, Options: node.Options, Parent: node.Parent})
				}
			}

			break
		}

	}

	//debug helper
	//rootNode.ValidateFinalTreeInvariants()

	// Done optimizing.  Return the final tree.
	return rootNode
}

// Finds {one/notone/set}loop nodes in the concatenation that can be automatically upgraded
// to {one/notone/set}loopatomic nodes.  Such changes avoid potential useless backtracking.
// e.g. A*B (where sets A and B don't overlap) => (?>A*)B.
func (n *RegexNode) findAndMakeLoopsAtomic() {
	if n.Options&RightToLeft != 0 {
		// RTL is so rare, we don't need to spend additional time/code optimizing for it.
		return
	}

	// For all node types that have children, recur into each of those children.
	for i := 0; i < len(n.Children); i++ {
		n.Children[i].findAndMakeLoopsAtomic()
	}

	// If this isn't a concatenation, nothing more to do.
	if n.T != NtConcatenate {
		return
	}

	// This is a concatenation.  Iterate through each pair of nodes in the concatenation seeing whether we can
	// make the first node (or its right-most child) atomic based on the second node (or its left-most child).
	for i := 0; i < len(n.Children)-1; i++ {
		n.Children[i].processNode(n.Children[i+1])
	}
}

func (node *RegexNode) processNode(subsequent *RegexNode) {
	// Skip down the node past irrelevant nodes.
	for {
		// We can always recur into captures and into the last node of concatenations.
		if node.T == NtCapture || node.T == NtConcatenate {
			node = node.Children[len(node.Children)-1]
			continue
		}

		// For loops with at least one guaranteed iteration, we can recur into them, but
		// we need to be careful not to just always do so; the ending node of a loop can only
		// be made atomic if what comes after the loop but also the beginning of the loop are
		// compatible for the optimization.
		if node.T == NtLoop {
			loopDescendent := node.FindLastExpressionInLoopForAutoAtomic()
			if loopDescendent != nil {
				node = loopDescendent
				continue
			}
		}

		// Can't skip any further.
		break
	}

	// If the node can be changed to atomic based on what comes after it, do so.
	switch node.T {
	case NtOneloop, NtNotoneloop, NtSetloop:
		if node.canBeMadeAtomic(subsequent, true, false) {
			// The greedy loop doesn't overlap with what comes after it, which means giving anything it matches back will not
			// help the overall match to succeed, which means it can simply become atomic to match as much as possible. The call
			// to CanBeMadeAtomic passes iterateNullableSubsequent=true because, in a pattern like a*b*c*, when analyzing a*, we
			// want to examine the b* and the c* rather than just giving up after seeing that b* is nullable; in order to make
			// the a* atomic, we need to know that anything that could possibly come after the loop doesn't overlap.
			node.makeLoopAtomic()
		}

	case NtOnelazy, NtNotonelazy, NtSetlazy:
		if node.canBeMadeAtomic(subsequent, false, true) {
			// The lazy loop doesn't overlap with what comes after it, which means it needs to match as much as its allowed
			// to match in order for there to be a possibility that what comes next matches (if it doesn't match as much
			// as it's allowed and there was still more it could match, then what comes next is guaranteed to not match,
			// since it doesn't match any of the same things the loop matches).  We don't want to just make the lazy loop
			// atomic, as an atomic lazy loop matches as little as possible, not as much as possible.  Instead, we want to
			// make the lazy loop into an atomic greedy loop.  Note that when we check CanBeMadeAtomic, we need to set
			// "iterateNullableSubsequent" to false so that we only inspect non-nullable subsequent nodes.  For example,
			// given a pattern like a*?b, we want to upgrade that loop to being greedy atomic, e.g. (?>a*)b.  But given a
			// pattern like a*?b*, the subsequent node is nullable, which means it doesn't have to be part of a match, which
			// means the a*? could match by itself, in which case as it's lazy it needs to match as few a's as possible, e.g.
			// a+?b* against the input "aaaab" should match "a", not "aaaa" nor "aaaab". (Technically for lazy, we only need to prevent
			// walking off the end of the pattern, but it's not currently worth complicating the implementation for that case.)
			// allowLazy is set to true so that the implementation will analyze rather than ignore this node; generally lazy nodes
			// are ignored due to making them atomic not generally being a sound change, but here we're explicitly choosing to
			// given the circumstances.
			node.T -= NtOnelazy - NtOneloop // lazy to greedy
			node.makeLoopAtomic()
		}

	case NtAlternate, NtBackRefCond, NtExprCond:
		// In the case of alternation, we can't change the alternation node itself
		// based on what comes after it (at least not with more complicated analysis
		// that factors in all branches together), but we can look at each individual
		// branch, and analyze ending loops in each branch individually to see if they
		// can be made atomic.  Then if we do end up backtracking into the alternation,
		// we at least won't need to backtrack into that loop.  The same is true for
		// conditionals, though we don't want to process the condition expression
		// itself, as it's already considered atomic and handled as part of ReduceExpressionConditional.
		b := 0
		if node.T == NtExprCond {
			b = 1
		}
		for ; b < len(node.Children); b++ {
			node.Children[b].processNode(subsequent)
		}

	}
}

func (n *RegexNode) reduce() *RegexNode {
	// Remove IgnoreCase option from everything except a Backreference
	if n.T != NtRef {
		n.Options &= ^IgnoreCase
	}
	switch n.T {
	case NtAlternate:
		return n.reduceAlternation()
	case NtAtomic:
		return n.reduceAtomic()
	case NtConcatenate:
		return n.reduceConcatenation()
	case NtGroup:
		return n.reduceGroup()
	case NtLoop, NtLazyloop:
		return n.reduceRep()
	case NtPosLook, NtNegLook:
		return n.reduceLookaround()
	case NtSet, NtSetloop, NtSetlazy, NtSetloopatomic:
		return n.reduceSet()
	case NtExprCond:
		return n.reduceExpressionConditional()
	case NtBackRefCond:
		return n.reduceBackreferenceConditional()
	default:
		return n
	}
}

// / <summary>Optimizations for positive and negative lookaheads/behinds.</summary>
func (n *RegexNode) reduceLookaround() *RegexNode {
	// A lookaround is a zero-width atomic assertion.
	// As it's atomic, nothing will backtrack into it, and we can
	// eliminate any ending backtracking from it.
	n.eliminateEndingBacktracking()

	// A positive lookaround wrapped around an empty is a nop, and we can reduce it
	// to simply Empty.  A developer typically doesn't write this, but rather it evolves
	// due to optimizations resulting in empty.

	// A negative lookaround wrapped around an empty child, i.e. (?!), is
	// sometimes used as a way to insert a guaranteed no-match into the expression,
	// often as part of a conditional. We can reduce it to simply Nothing.

	if n.Children[0].T == NtEmpty {
		if n.T == NtPosLook {
			n.T = NtEmpty
		} else {
			n.T = NtNothing
		}
		n.Children = nil
	}

	return n
}

// Optimizations for backreference conditionals.
func (n *RegexNode) reduceBackreferenceConditional() *RegexNode {
	// This isn't so much an optimization as it is changing the tree for consistency. We want
	// all engines to be able to trust that every backreference conditional will have two children,
	// even though it's optional in the syntax.  If it's missing a "not matched" branch,
	// we add one that will match empty.
	if len(n.Children) == 1 {
		n.addChild(&RegexNode{T: NtEmpty, Options: n.Options})
	}

	return n
}

// / <summary>Optimizations for expression conditionals.</summary>
func (n *RegexNode) reduceExpressionConditional() *RegexNode {
	// This isn't so much an optimization as it is changing the tree for consistency. We want
	// all engines to be able to trust that every expression conditional will have three children,
	// even though it's optional in the syntax.  If it's missing a "not matched" branch,
	// we add one that will match empty.
	if len(n.Children) == 2 {
		n.addChild(&RegexNode{T: NtEmpty, Options: n.Options})
	}

	// It's common for the condition to be an explicit positive lookahead, as specifying
	// that eliminates any ambiguity in syntax as to whether the expression is to be matched
	// as an expression or to be a reference to a capture group.  After parsing, however,
	// there's no ambiguity, and we can remove an extra level of positive lookahead, as the
	// engines need to treat the condition as a zero-width positive, atomic assertion regardless.
	condition := n.Children[0]
	if condition.T == NtPosLook && (condition.Options&RightToLeft) == 0 {
		n.ReplaceChild(0, condition.Children[0])
	}

	// We can also eliminate any ending backtracking in the condition, as the condition
	// is considered to be a positive lookahead, which is an atomic zero-width assertion.
	condition = n.Children[0]
	condition.eliminateEndingBacktracking()

	return n
}

// Remove unnecessary atomic nodes, and make appropriate descendents of the atomic node themselves atomic.
// e.g. (?>(?>(?>a*))) => (?>a*)
// e.g. (?>(abc*)*) => (?>(abc(?>c*))*)
func (n *RegexNode) reduceAtomic() *RegexNode {
	atomic := n
	child := n.Children[0]
	for child.T == NtAtomic {
		atomic = child
		child = atomic.Children[0]
	}

	switch child.T {
	// If the child is empty/nothing, there's nothing to be made atomic so the Atomic
	// node can simply be removed.
	case NtEmpty, NtNothing:
		return child

	// If the child is already atomic, we can just remove the atomic node.
	case NtOneloopatomic, NtNotoneloopatomic, NtSetloopatomic:
		return child

	// If an atomic subexpression contains only a {one/notone/set}{loop/lazy},
	// change it to be an {one/notone/set}loopatomic and remove the atomic node.
	case NtOneloop, NtNotoneloop, NtSetloop, NtOnelazy, NtNotonelazy, NtSetlazy:
		child.makeLoopAtomic()
		return child

	// Alternations have a variety of possible optimizations that can be applied
	// iff they're atomic.
	case NtAlternate:
		if (n.Options & RightToLeft) == 0 {
			branches := child.Children

			// If an alternation is atomic and its first branch is Empty, the whole thing
			// is a nop, as Empty will match everything trivially, and no backtracking
			// into the node will be performed, making the remaining branches irrelevant.
			if branches[0].T == NtEmpty {
				return &RegexNode{T: NtEmpty, Options: child.Options}
			}

			// Similarly, we can trim off any branches after an Empty, as they'll never be used.
			// An Empty will match anything, and thus branches after that would only be used
			// if we backtracked into it and advanced passed the Empty after trying the Empty...
			// but if the alternation is atomic, such backtracking won't happen.
			for i := 1; i < len(branches)-1; i++ {
				if branches[i].T == NtEmpty {
					branches = slices.Delete(branches, i+1, len(branches))
					break
				}
			}

			// If an alternation is atomic, we won't ever backtrack back into it, which
			// means order matters but not repetition.  With backtracking, it would be incorrect
			// to convert an expression like "hi|there|hello" into "hi|hello|there", as doing
			// so could then change the order of results if we matched "hi" and then failed
			// based on what came after it, and both "hello" and "there" could be successful
			// with what came later.  But without backtracking, we can reorder "hi|there|hello"
			// to instead be "hi|hello|there", as "hello" and "there" can't match the same text,
			// and once this atomic alternation has matched, we won't try another branch. This
			// reordering is valuable as it then enables further optimizations, e.g.
			// "hi|there|hello" => "hi|hello|there" => "h(?:i|ello)|there", which means we only
			// need to check the 'h' once in case it's not an 'h', and it's easier to employ different
			// code gen that, for example, switches on first character of the branches, enabling faster
			// choice of branch without always having to walk through each.
			reordered := false
			for start := 0; start < len(branches); start++ {
				// Get the node that may start our range.  If it's a one, multi, or concat of those, proceed.
				startNode := branches[start]
				if startNode.findBranchOneOrMultiStart() == nil {
					continue
				}

				// Find the contiguous range of nodes from this point that are similarly one, multi, or concat of those.
				endExclusive := start + 1
				for endExclusive < len(branches) && branches[endExclusive].findBranchOneOrMultiStart() != nil {
					endExclusive++
				}

				// If there's at least 3, there may be something to reorder (we won't reorder anything
				// before the starting position, and so only 2 items is considered ordered).
				if endExclusive-start >= 3 {
					compare := start
					for compare < endExclusive {
						// Get the starting character
						c := branches[compare].findBranchOneOrMultiStart().FirstCharOfOneOrMulti()

						// Move compare to point to the last branch that has the same starting value.
						for compare < endExclusive && branches[compare].findBranchOneOrMultiStart().FirstCharOfOneOrMulti() == c {
							compare++
						}

						// Compare now points to the first node that doesn't match the starting node.
						// If we've walked off our range, there's nothing left to reorder.
						if compare < endExclusive {
							// There may be something to reorder.  See if there are any other nodes that begin with the same character.
							for next := compare + 1; next < endExclusive; next++ {
								nextChild := branches[next]
								if nextChild.findBranchOneOrMultiStart().FirstCharOfOneOrMulti() == c {
									branches = slices.Delete(branches, next, next+1)
									branches = slices.Insert(branches, compare, nextChild)
									compare++
									reordered = true
								}
							}
						}
					}
				}

				// Move to the end of the range we've now explored. endExclusive is not a viable
				// starting position either, and the start++ for the loop will thus take us to
				// the next potential place to start a range.
				start = endExclusive
			}
			child.Children = branches
			// If anything was reordered, there may be new optimization opportunities inside
			// of the alternation, so reduce it again.
			if reordered {
				atomic.ReplaceChild(0, child)
				child = atomic.Children[0]
			}
		}
		fallthrough

	// For everything else, try to reduce ending backtracking of the last contained expression.
	default:
		child.eliminateEndingBacktracking()
		return atomic
	}
}

func (n *RegexNode) makeLoopAtomic() {

	switch n.T {
	case NtOneloop, NtNotoneloop, NtSetloop:
		// For loops, we simply change the Type to the atomic variant.
		// Atomic greedy loops should consume as many values as they can.
		n.T += NtOneloopatomic - NtOneloop

	case NtOnelazy, NtNotonelazy, NtSetlazy:
		// For lazy, we not only change the Type, we also lower the max number of iterations
		// to the minimum number of iterations, creating a repeater, as they should end up
		// matching as little as possible.
		n.T += NtOneloopatomic - NtOnelazy
		n.N = n.M
		if n.N == 0 {
			// If moving the max to be the same as the min dropped it to 0, there's no
			// work to be done for this node, and we can make it Empty.
			n.T = NtEmpty
			n.Str = nil
			n.Ch = 0x0
		} else if n.T == NtOneloopatomic && n.N >= 2 && n.N <= MultiVsRepeaterLimit {
			// If this is now a One repeater with a small enough length,
			// make it a Multi instead, as they're better optimized down the line.
			n.T = NtMulti
			n.Str = []rune(strings.Repeat(string(n.Ch), n.N))
			n.Ch = 0x0
			n.M = 0
			n.N = 0
		}
	}
}

// Converts nodes at the end of the node tree to be atomic.
// The correctness of this optimization depends on nothing being able to backtrack into
// the provided node.  That means it must be at the root of the overall expression, or
// it must be an Atomic node that nothing will backtrack into by the very nature of Atomic.
func (n *RegexNode) eliminateEndingBacktracking() {
	// Walk the tree starting from the current node.
	node := n
	for {
		switch node.T {
		// {One/Notone/Set}loops can be upgraded to {One/Notone/Set}loopatomic nodes, e.g. [abc]* => (?>[abc]*).
		// And {One/Notone/Set}lazys can similarly be upgraded to be atomic, which really makes them into repeaters
		// or even empty nodes.
		case NtOneloop, NtNotoneloop, NtSetloop, NtOnelazy, NtNotonelazy, NtSetlazy:
			node.makeLoopAtomic()

		// Just because a particular node is atomic doesn't mean all its descendants are.
		// Process them as well. Lookarounds are implicitly atomic.
		case NtAtomic, NtPosLook, NtNegLook:
			node = node.Children[0]
			continue

		case NtCapture, NtConcatenate:
			// For Capture and Concatenate, we just recur into their last child (only child in the case
			// of Capture).  However, if the child is an alternation or loop, we can also make the
			// node itself atomic by wrapping it in an Atomic node. Since we later check to see whether a
			// node is atomic based on its parent or grandparent, we don't bother wrapping such a node in
			// an Atomic one if its grandparent is already Atomic.
			// e.g. [xyz](?:abc|def) => [xyz](?>abc|def)

			// validate grandparent isn't atomic
			existingChild := node.Children[len(node.Children)-1]
			if (existingChild.T == NtAlternate || existingChild.T == NtBackRefCond ||
				existingChild.T == NtExprCond || existingChild.T == NtLoop ||
				existingChild.T == NtLazyloop) &&
				(node.Parent == nil || node.Parent.T != NtAtomic) {

				atomic := &RegexNode{T: NtAtomic, Options: existingChild.Options}
				atomic.addChild(existingChild)
				node.ReplaceChild(len(node.Children)-1, atomic)
			}
			node = existingChild
			continue

		// For alternate, we can recur into each branch separately.  We use this iteration for the first branch.
		// Conditionals are just like alternations in this regard.
		// e.g. abc*|def* => ab(?>c*)|de(?>f*)
		case NtAlternate, NtBackRefCond, NtExprCond:

			branches := len(node.Children)
			for i := 1; i < branches; i++ {
				node.Children[i].eliminateEndingBacktracking()
			}

			// ReduceExpressionConditional will have already applied ending backtracking removal
			if node.T != NtExprCond {
				node = node.Children[0]
				continue
			}

		// For {Lazy}Loop, we search to see if there's a viable last expression, and iff there
		// is we recur into processing it.  Also, as with the single-char lazy loops, LazyLoop
		// can have its max iteration count dropped to its min iteration count, as there's no
		// reason for it to match more than the minimal at the end; that in turn makes it a
		// repeater, which results in better code generation.
		// e.g. (?:abc*)* => (?:ab(?>c*))*
		// e.g. (abc*?)+? => (ab){1}
		case NtLazyloop:
			node.N = node.M
			fallthrough
		case NtLoop:
			if node.N == 1 {
				// If the loop has a max iteration count of 1 (e.g. it's an optional node),
				// there's no possibility for conflict between multiple iterations, so
				// we can process it.
				node = node.Children[0]
				continue
			}

			loopDescendent := node.FindLastExpressionInLoopForAutoAtomic()
			if loopDescendent != nil {
				node = loopDescendent
				continue // loop around to process node
			}

		}

		break
	}
}

// Recurs into the last expression of a loop node, looking to see if it can find a node
// that could be made atomic _assuming_ the conditions exist for it with the loop's ancestors.
// Returns The found node that should be explored further for auto-atomicity; null if it doesn't exist.
func (n *RegexNode) FindLastExpressionInLoopForAutoAtomic() *RegexNode {
	node := n

	// Start by looking at the loop's sole child.
	node = node.Children[0]

	// Skip past captures.
	for node.T == NtCapture {
		node = node.Children[0]
	}

	// If the loop's body is a concatenate, we can skip to its last child iff that
	// last child doesn't conflict with the first child, since this whole concatenation
	// could be repeated, such that the first node ends up following the last.  For
	// example, in the expression (a+[def])*, the last child is [def] and the first is
	// a+, which can't possibly overlap with [def].  In contrast, if we had (a+[ade])*,
	// [ade] could potentially match the starting 'a'.
	if node.T == NtConcatenate {
		concatCount := len(node.Children)
		lastConcatChild := node.Children[concatCount-1]
		if lastConcatChild.canBeMadeAtomic(node.Children[0], false, false) {
			return lastConcatChild
		}
	}

	// Otherwise, the loop has nothing that can participate in auto-atomicity.
	return nil
}

// Determines whether a node can be switched to an atomic loop.
//
// The node following is subsequent, used to determine whether it overlaps.
// iterateNullableSubsequent is whether to allow examining nodes beyond subsequent.
// allowLazy is whether lazy loops in addition to greedy loops should be considered for atomicity.
func (n *RegexNode) canBeMadeAtomic(subsequent *RegexNode, iterateNullableSubsequent, allowLazy bool) bool {
	// In most case, we'll simply check the node against whatever subsequent is.  However, in case
	// subsequent ends up being a loop with a min bound of 0, we'll also need to evaluate the node
	// against whatever comes after subsequent.  In that case, we'll walk the tree to find the
	// next subsequent, and we'll loop around against to perform the comparison again.
	for {
		// Skip the successor down to the closest node that's guaranteed to follow it.
		childCount := len(subsequent.Children)
		for ; childCount > 0; childCount = len(subsequent.Children) {
			if subsequent.T == NtConcatenate || subsequent.T == NtCapture ||
				subsequent.T == NtAtomic ||
				(subsequent.T == NtPosLook && subsequent.Options&RightToLeft == 0) ||
				((subsequent.T == NtLoop || subsequent.T == NtLazyloop) && subsequent.M > 0) {
				subsequent = subsequent.Children[0]
				continue
			}

			break
		}

		// If the current node's options don't match the subsequent node, then we cannot make it atomic.
		// This applies to RightToLeft for lookbehinds, as well as patterns that enable/disable global flags in the middle of the pattern.
		if n.Options != subsequent.Options {
			return false
		}

		// If the successor is an alternation, all of its children need to be evaluated, since any of them
		// could come after this node.  If any of them fail the optimization, then the whole node fails.
		// This applies to expression conditionals as well, as long as they have both a yes and a no branch (if there's
		// only a yes branch, we'd need to also check whatever comes after the conditional).  It doesn't apply to
		// backreference conditionals, as the condition itself is unknown statically and could overlap with the
		// loop being considered for atomicity.
		if subsequent.T == NtAlternate || (subsequent.T == NtExprCond && childCount == 3) {
			// condition, yes, and no branch
			for i := 0; i < childCount; i++ {
				if !n.canBeMadeAtomic(subsequent.Children[i], iterateNullableSubsequent, false) {
					return false
				}
			}
			return true
		}

		// If this node is a {one/notone/set}loop, see if it overlaps with its successor in the concatenation.
		// If it doesn't, then we can upgrade it to being a {one/notone/set}loopatomic.
		// Doing so avoids unnecessary backtracking.
		if n.T == NtOneloop || (n.T == NtOnelazy && allowLazy) {

			if (subsequent.T == NtOne && n.Ch != subsequent.Ch) ||
				(subsequent.T == NtNotone && n.Ch == subsequent.Ch) ||
				(subsequent.T == NtSet && !subsequent.Set.CharIn(n.Ch)) ||
				(subsequent.IsOneFamily() && subsequent.M > 0 && n.Ch != subsequent.Ch) ||
				(subsequent.IsNotoneFamily() && subsequent.M > 0 && n.Ch == subsequent.Ch) ||
				(subsequent.IsSetFamily() && subsequent.M > 0 && !subsequent.Set.CharIn(n.Ch)) ||
				(subsequent.T == NtMulti && n.Ch != subsequent.Str[0]) ||
				(subsequent.T == NtEnd) ||
				(subsequent.T == NtEndZ && n.Ch != '\n') ||
				(subsequent.T == NtEol && n.Ch != '\n') {
				return true
			}

			// The loop can be made atomic based on this subsequent node, but we'll need to evaluate the next one as well.
			if (subsequent.IsOneloopFamily() && subsequent.M == 0 && n.Ch != subsequent.Ch) ||
				(subsequent.IsNotoneloopFamily() && subsequent.M == 0 && n.Ch == subsequent.Ch) ||
				(subsequent.IsSetloopFamily() && subsequent.M == 0 && !subsequent.Set.CharIn(n.Ch)) ||
				(subsequent.T == NtBoundary && n.M > 0 && IsWordChar(n.Ch)) ||
				(subsequent.T == NtNonboundary && n.M > 0 && !IsWordChar(n.Ch)) ||
				(subsequent.T == NtECMABoundary && n.M > 0 && IsECMAWordChar(n.Ch)) ||
				(subsequent.T == NtNonECMABoundary && n.M > 0 && !IsECMAWordChar(n.Ch)) {
				goto end
			}

			return false

		} else if n.T == NtNotoneloop || (n.T == NtNotonelazy && allowLazy) {
			if (subsequent.T == NtOne && n.Ch == subsequent.Ch) ||
				(subsequent.IsOneFamily() && subsequent.M > 0 && n.Ch == subsequent.Ch) ||
				(subsequent.T == NtMulti && n.Ch == subsequent.Str[0]) ||
				(subsequent.T == NtEnd) {
				return true
			}

			// The loop can be made atomic based on this subsequent node, but we'll need to evaluate the next one as well.
			if subsequent.IsOneloopFamily() && subsequent.M == 0 && n.Ch == subsequent.Ch {
				goto end
			}

			return false
		} else if n.T == NtSetloop || (n.T == NtSetlazy && allowLazy) {
			if (subsequent.T == NtOne && !n.Set.CharIn(subsequent.Ch)) ||
				(subsequent.T == NtSet && !n.Set.MayOverlap(subsequent.Set)) ||
				(subsequent.IsOneloopFamily() && subsequent.M > 0 && !n.Set.CharIn(subsequent.Ch)) ||
				(subsequent.IsSetloopFamily() && subsequent.M > 0 && !n.Set.MayOverlap(subsequent.Set)) ||
				(subsequent.T == NtMulti && !n.Set.CharIn(subsequent.Str[0])) ||
				(subsequent.T == NtEnd) ||
				(subsequent.T == NtEndZ && !n.Set.CharIn('\n')) ||
				(subsequent.T == NtEol && !n.Set.CharIn('\n')) {
				return true
			}

			if (subsequent.IsOneloopFamily() && subsequent.M == 0 && !n.Set.CharIn(subsequent.Ch)) ||
				(subsequent.IsSetloopFamily() && subsequent.M == 0 && subsequent.Set.MayOverlap(n.Set)) ||
				(subsequent.T == NtBoundary && n.M > 0 && (n.Set.Equals(WordClass()) || n.Set.Equals(DigitClass()))) ||
				(subsequent.T == NtNonboundary && n.M > 0 && (n.Set.Equals(NotWordClass()) || n.Set.Equals(NotDigitClass()))) ||
				(subsequent.T == NtECMABoundary && n.M > 0 && (n.Set.Equals(ECMAWordClass()) || n.Set.Equals(ECMADigitClass()))) ||
				(subsequent.T == NtNonECMABoundary && n.M > 0 && (n.Set.Equals(NotECMAWordClass()) || n.Set.Equals(NotDigitClass()))) {
				// The loop can be made atomic based on this subsequent node, but we'll need to evaluate the next one as well.
				goto end
			}
			return false
		} else {
			return false
		}

	end:
		// We only get here if the node could be made atomic based on subsequent but subsequent has a lower bound of zero
		// and thus we need to move subsequent to be the next node in sequence and loop around to try again.
		if !iterateNullableSubsequent {
			return false
		}

		// To be conservative, we only walk up through a very limited set of constructs (even though we may have walked
		// down through more, like loops), looking for the next concatenation that we're not at the end of, at
		// which point subsequent becomes whatever node is next in that concatenation.
		for {
			parent := subsequent.Parent
			if parent == nil {
				// If we hit the root, we're at the end of the expression, at which point nothing could backtrack
				// in and we can declare success.
				return true
			}

			switch parent.T {
			case NtAtomic, NtAlternate, NtCapture:
				subsequent = parent
				continue

			case NtConcatenate:
				peers := parent.Children
				currentIndex := slices.Index(peers, subsequent)

				if currentIndex+1 == len(peers) {
					subsequent = parent
					continue
				} else {
					subsequent = peers[currentIndex+1]
				}

			default:
				// Anything else, we don't know what to do, so we have to assume it could conflict with the loop.
				return false
			}

			break
		}
	}
}

// Basic optimization. Single-letter alternations can be replaced
// by faster set specifications, and nested alternations with no
// intervening operators can be flattened:
//
// a|b|c|def|g|h -> [a-c]|def|[gh]
// apple|(?:orange|pear)|grape -> apple|orange|pear|grape
func (n *RegexNode) reduceAlternation() *RegexNode {
	if len(n.Children) == 0 {
		return newRegexNode(NtNothing, n.Options)
	}
	if len(n.Children) == 1 {
		return n.Children[0]
	}
	n.reduceSingleLetterAndNestedAlternations()

	node := n.replaceNodeIfUnnecessary()
	if node.T == NtAlternate {
		node = node.extractCommonPrefixText()
		if node.T == NtAlternate {
			node = node.extractCommonPrefixOneNotoneSet()
			if node.T == NtAlternate {
				node = node.removeRedundantEmptiesAndNothings()
			}
		}
	}
	return node
}

// Analyzes all the branches of the alternation for text that's identical at the beginning
// of every branch.  That text is then pulled out into its own one or multi node in a
// concatenation with the alternation (whose branches are updated to remove that prefix).
// This is valuable for a few reasons.  One, it exposes potentially more text to the
// expression prefix analyzer used to influence FindFirstChar.  Second, it exposes more
// potential alternation optimizations, e.g. if the same prefix is followed in two branches
// by sets that can be merged.  Third, it reduces the amount of duplicated comparisons required
// if we end up backtracking into subsequent branches.
// e.g. abc|ade => a(?bc|de)
func (n *RegexNode) extractCommonPrefixText() *RegexNode {
	// To keep things relatively simple, we currently only handle:
	// - Left to right (e.g. we don't process alternations in lookbehinds)
	// - Branches that are one or multi nodes, or that are concatenations beginning with one or multi nodes.
	// - All branches having the same options.

	// Only extract left-to-right prefixes.
	if (n.Options & RightToLeft) != 0 {
		return n
	}

	for startingIndex := 0; startingIndex < len(n.Children)-1; startingIndex++ {
		// Process the first branch to get the maximum possible common string.
		startingNode := n.Children[startingIndex].findBranchOneOrMultiStart()
		if startingNode == nil {
			return n
		}

		startingNodeOptions := startingNode.Options
		startingSpan := startingNode.Str
		if startingNode.T == NtOne {
			startingSpan = []rune{startingNode.Ch}
		}

		// Now compare the rest of the branches against it.
		endingIndex := startingIndex + 1
		for ; endingIndex < len(n.Children); endingIndex++ {
			// Get the starting node of the next branch.
			startingNode = n.Children[endingIndex].findBranchOneOrMultiStart()
			if startingNode == nil || startingNode.Options != startingNodeOptions {
				break
			}

			// See if the new branch's prefix has a shared prefix with the current one.
			// If it does, shorten to that; if it doesn't, bail.
			if startingNode.T == NtOne {
				if startingSpan[0] != startingNode.Ch {
					break
				}

				if len(startingSpan) != 1 {
					startingSpan = startingSpan[0:1]
				}
			} else {
				minLength := len(startingSpan)
				if len(startingNode.Str) < minLength {
					minLength = len(startingNode.Str)
				}
				c := 0
				for c < minLength && startingSpan[c] == startingNode.Str[c] {
					c++
				}

				if c == 0 {
					break
				}

				startingSpan = startingSpan[0:c]
			}
		}

		// When we get here, we have a starting string prefix shared by all branches
		// in the range [startingIndex, endingIndex).
		if endingIndex-startingIndex <= 1 {
			// There's nothing to consolidate for this starting node.
			continue
		}

		// We should be able to consolidate something for the nodes in the range [startingIndex, endingIndex).

		// Create a new node of the form:
		//     Concatenation(prefix, Alternation(each | node | with | prefix | removed))
		// that replaces all these branches in this alternation.
		var prefix *RegexNode
		if len(startingSpan) == 1 {
			prefix = &RegexNode{T: NtOne, Options: startingNodeOptions, Ch: startingSpan[0]}
		} else {
			prefix = &RegexNode{T: NtMulti, Options: startingNodeOptions, Str: slices.Clone(startingSpan)}
		}

		newAlternate := &RegexNode{T: NtAlternate, Options: startingNodeOptions}
		for i := startingIndex; i < endingIndex; i++ {
			branch := n.Children[i]
			if branch.T == NtConcatenate {
				branch.Children[0].processOneOrMulti(startingSpan)
			} else {
				branch.processOneOrMulti(startingSpan)
			}
			branch = branch.reduce()
			newAlternate.addChild(branch)
		}

		if n.Parent != nil && n.Parent.T == NtAtomic {
			var atomic = &RegexNode{T: NtAtomic, Options: startingNodeOptions}
			atomic.addChild(newAlternate)
			newAlternate = atomic
		}

		newConcat := &RegexNode{T: NtConcatenate, Options: startingNodeOptions}
		newConcat.addChild(prefix)
		newConcat.addChild(newAlternate)
		n.ReplaceChild(startingIndex, newConcat)
		n.Children = slices.Delete(n.Children, startingIndex+1, endingIndex)
	}

	if len(n.Children) == 1 {
		return n.Children[0]
	}

	return n
}

// This function optimizes out prefix nodes from alternation branches that are
// the same across multiple contiguous branches.
// e.g. \w12|\d34|\d56|\w78|\w90 => \w12|\d(?:34|56)|\w(?:78|90)
func (n *RegexNode) extractCommonPrefixOneNotoneSet() *RegexNode {
	// Only process left-to-right prefixes.
	if (n.Options & RightToLeft) != 0 {
		return n
	}

	// Only handle the case where each branch is a concatenation
	for _, child := range n.Children {
		if child.T != NtConcatenate || len(child.Children) < 2 {
			return n
		}
	}

	for startingIndex := 0; startingIndex < len(n.Children)-1; startingIndex++ {
		// Only handle the case where each branch begins with the same One, Notone, or Set (individual or loop).
		// Note that while we can do this for individual characters, fixed length loops, and atomic loops, doing
		// it for non-atomic variable length loops could change behavior as each branch could otherwise have a
		// different number of characters consumed by the loop based on what's after it.
		required := n.Children[startingIndex].Children[0]

		if (!required.IsOneFamily() && !required.IsNotoneFamily() && !required.IsSetFamily()) ||
			required.M != required.N {
			// skip if it's not one of these scenarios
			continue
		}

		// Only handle the case where each branch begins with the exact same node value
		endingIndex := startingIndex + 1
		for ; endingIndex < len(n.Children); endingIndex++ {
			other := n.Children[endingIndex].Children[0]
			if required.T != other.T ||
				required.Options != other.Options ||
				required.M != other.M ||
				required.N != other.N ||
				required.Ch != other.Ch ||
				!slices.Equal(required.Str, other.Str) ||
				!required.Set.Equals(other.Set) {
				break
			}
		}

		if endingIndex-startingIndex <= 1 {
			// Nothing to extract from this starting index.
			continue
		}

		// Remove the prefix node from every branch, adding it to a new alternation
		newAlternate := &RegexNode{T: NtAlternate, Options: n.Options}
		for i := startingIndex; i < endingIndex; i++ {
			n.Children[i].Children = slices.Delete(n.Children[i].Children, 0, 1)
			newAlternate.addChild(n.Children[i])
		}

		// If this alternation is wrapped as atomic, we need to do the same for the new alternation.
		if n.Parent != nil && n.Parent.T == NtAtomic {
			atomic := &RegexNode{T: NtAtomic, Options: n.Options}
			atomic.addChild(newAlternate)
			newAlternate = atomic
		}

		// Now create a concatenation of the prefix node with the new alternation for the combined
		// branches, and replace all of the branches in this alternation with that new concatenation.
		newConcat := &RegexNode{T: NtConcatenate, Options: n.Options}
		newConcat.addChild(required)
		newConcat.addChild(newAlternate)
		n.ReplaceChild(startingIndex, newConcat)
		n.Children = slices.Delete(n.Children, startingIndex+1, endingIndex)
	}

	return n.replaceNodeIfUnnecessary()
}

// Removes unnecessary Empty and Nothing nodes from the alternation. A Nothing will never
// match, so it can be removed entirely, and an Empty can be removed if there's a previous
// Empty in the alternation: it's an extreme case of just having a repeated branch in an
// alternation, and while we don't check for all duplicates, checking for empty is easy.
func (n *RegexNode) removeRedundantEmptiesAndNothings() *RegexNode {
	children := n.Children

	i, j := 0, 0
	seenEmpty := false
	for i < len(children) {
		child := children[i]
		if child.T == NtNothing || (child.T == NtEmpty && seenEmpty) {
			i++
			continue
		}
		if child.T == NtEmpty {
			seenEmpty = true
		}
		children[j] = children[i]
		i++
		j++
	}

	n.Children = slices.Delete(children, j, len(children))
	return n.replaceNodeIfUnnecessary()
}

// Remove the starting text from the one or multi node.  This may end up changing
// the type of the node to be Empty if the starting text matches the node's full value.
func (n *RegexNode) processOneOrMulti(startingSpan []rune) {
	if n.T == NtOne {
		n.T = NtEmpty
		n.Ch = 0x0
	} else {
		if len(n.Str) == len(startingSpan) {
			n.T = NtEmpty
			n.Str = nil
		} else if len(n.Str)-1 == len(startingSpan) {
			n.T = NtOne
			n.Ch = n.Str[len(n.Str)-1]
			n.Str = nil
		} else {
			n.Str = n.Str[len(startingSpan):]
		}
	}
}

// Finds the starting one or multi of the branch, if it has one; otherwise, returns null.
// For simplicity, this only considers branches that are One or Multi, or a Concatenation
// beginning with a One or Multi.  We don't traverse more than one level to avoid the
// complication of then having to later update that hierarchy when removing the prefix,
// but it could be done in the future if proven beneficial enough.
func (n *RegexNode) findBranchOneOrMultiStart() *RegexNode {
	branch := n
	if n.T == NtConcatenate {
		branch = n.Children[0]
	}
	if branch.T == NtOne || branch.T == NtMulti {
		return branch
	}
	return nil
}

func (n *RegexNode) reduceSingleLetterAndNestedAlternations() {

	wasLastSet := false
	lastNodeCannotMerge := false
	var optionsLast RegexOptions
	var i, j int

	for i, j = 0, 0; i < len(n.Children); i, j = i+1, j+1 {
		at := n.Children[i]

		if j < i {
			n.Children[j] = at
		}

		for {
			if at.T == NtAlternate {
				n.insertChildren(i+1, at.Children)
				j--
			} else if at.T == NtSet || at.T == NtOne {
				// Cannot merge sets if L or I options differ, or if either are negated.
				optionsAt := at.Options & (RightToLeft | IgnoreCase)

				if at.T == NtSet {
					if !wasLastSet || optionsLast != optionsAt || lastNodeCannotMerge || !at.Set.IsMergeable() {
						wasLastSet = true
						lastNodeCannotMerge = !at.Set.IsMergeable()
						optionsLast = optionsAt
						break
					}
				} else if !wasLastSet || optionsLast != optionsAt || lastNodeCannotMerge {
					wasLastSet = true
					lastNodeCannotMerge = false
					optionsLast = optionsAt
					break
				}

				// The last node was a Set or a One, we're a Set or One and our options are the same.
				// Merge the two nodes.
				j--
				prev := n.Children[j]

				var prevCharClass *CharSet
				if prev.T == NtOne {
					prevCharClass = &CharSet{}
					prevCharClass.addChar(prev.Ch)
				} else {
					prevCharClass = prev.Set
				}

				if at.T == NtOne {
					prevCharClass.addChar(at.Ch)
				} else {
					prevCharClass.addSet(*at.Set)
				}

				prev.T = NtSet
				prev.Set = prevCharClass
				if prev.Options&IgnoreCase != 0 {
					prev.Options &= ^IgnoreCase
				}
			} else if at.T == NtNothing {
				j--
			} else {
				wasLastSet = false
				lastNodeCannotMerge = false
			}
			break
		}
	}

	if j < i {
		n.removeChildren(j, i)
	}
}

func (n *RegexNode) reduceConcatenation() *RegexNode {
	// Eliminate empties and concat adjacent strings/chars

	if len(n.Children) == 0 {
		return newRegexNode(NtEmpty, n.Options)
	}
	// remove concat
	if len(n.Children) == 1 {
		return n.Children[0]
	}

	// If any node in the concatenation is a Nothing, the concatenation itself is a Nothing.
	for i := 0; i < len(n.Children); i++ {
		child := n.Children[i]
		if child.T == NtNothing {
			return child
		}
	}

	// Coalesce adjacent loops.  This helps to minimize work done by the interpreter, minimize code gen,
	// and also help to reduce catastrophic backtracking.
	n.reduceConcatenationWithAdjacentLoops()

	// Coalesce adjacent characters/strings.  This is done after the adjacent loop coalescing so that
	// a One adjacent to both a Multi and a Loop prefers being folded into the Loop rather than into
	// the Multi.  Doing so helps with auto-atomicity when it's later applied.
	n.reduceConcatenationWithAdjacentStrings()

	// If the concatenation is now empty, return an empty node, or if it's got a single child, return that child.
	// Otherwise, return this.
	return n.replaceNodeIfUnnecessary()
}

func addMinLength(x, y int) int {
	const maxMinLength = math.MaxInt32 - 1
	if x >= maxMinLength || y >= maxMinLength || x > maxMinLength-y {
		return maxMinLength
	}
	return x + y
}

func multiplyMinLength(x, y int) int {
	const maxMinLength = math.MaxInt32 - 1
	if x == 0 || y == 0 {
		return 0
	}
	if x >= maxMinLength || y >= maxMinLength || x > maxMinLength/y {
		return maxMinLength
	}
	return x * y
}

func addMaxLength(x, y int) int {
	if x < 0 || y < 0 || x >= math.MaxInt32 || y >= math.MaxInt32 || x > (math.MaxInt32-1)-y {
		return -1
	}
	return x + y
}

func multiplyMaxLength(x, y int) int {
	if x < 0 || y < 0 {
		return -1
	}
	if x == 0 || y == 0 {
		return 0
	}
	if x >= math.MaxInt32 || y >= math.MaxInt32 || x > (math.MaxInt32-1)/y {
		return -1
	}
	return x * y
}

func maxLessThanTwiceMin(max, min int) bool {
	if min <= math.MaxInt32/2 {
		return max < min*2
	}
	return max != math.MaxInt32
}

func canCombineCounts(nodeMin, nodeMax, nextMin, nextMax int) bool {
	// We shouldn't have an infinite minimum; bail if we find one. Also check for the
	// degenerate case where we'd make the min overflow or go infinite when it wasn't already.
	if nodeMin == math.MaxInt32 ||
		nextMin == math.MaxInt32 ||
		addMaxLength(nodeMin, nextMin) < 0 {
		return false
	}

	// Similar overflow / go infinite check for max (which can be infinite).
	if nodeMax != math.MaxInt32 &&
		nextMax != math.MaxInt32 &&
		addMaxLength(nodeMax, nextMax) < 0 {
		return false
	}

	return true
}

// Combine adjacent loops.
// e.g. a*a*a* => a*
// e.g. a+ab => a{2,}b
func (n *RegexNode) reduceConcatenationWithAdjacentLoops() {
	current, next, nextSave := 0, 1, 1

	for next < len(n.Children) {
		currentNode := n.Children[current]
		nextNode := n.Children[next]

		if currentNode.Options == nextNode.Options {
			// Coalescing a loop with its same type
			if ((currentNode.IsOneloopFamily() || currentNode.IsNotoneloopFamily()) && nextNode.T == currentNode.T && currentNode.Ch == nextNode.Ch) ||
				(currentNode.IsSetloopFamily() && currentNode.T == nextNode.T && currentNode.Set.Equals(nextNode.Set)) {
				if nextNode.M > 0 && currentNode.IsAtomicloopFamily() {
					// Atomic loops can only be combined if the second loop has no lower bound, as if it has a lower bound,
					// combining them changes behavior. Uncombined, the first loop can consume all matching items;
					// the second loop might then not be able to meet its minimum and fail.  But if they're combined, the combined
					// minimum of the sole loop could now be met, introducing matches where there shouldn't have been any.
					goto End
				}

				if !canCombineCounts(currentNode.M, currentNode.N, nextNode.M, nextNode.N) {
					goto End
				}
				currentNode.M += nextNode.M
				if currentNode.N != math.MaxInt32 {
					if nextNode.N == math.MaxInt32 {
						currentNode.N = math.MaxInt32
					} else {
						currentNode.N += nextNode.N
					}
				}
				next++
				continue

			} else if ((currentNode.T == NtOneloop || currentNode.T == NtOnelazy) && nextNode.T == NtOne && currentNode.Ch == nextNode.Ch) ||
				((currentNode.T == NtNotoneloop || currentNode.T == NtNotonelazy) && nextNode.T == NtNotone && currentNode.Ch == nextNode.Ch) ||
				((currentNode.T == NtSetloop || currentNode.T == NtSetlazy) && nextNode.T == NtSet && currentNode.Set.Equals(nextNode.Set)) {
				// Coalescing a loop with an additional item of the same type
				if canCombineCounts(currentNode.M, currentNode.N, 1, 1) {
					currentNode.M++
					if currentNode.N != math.MaxInt32 {
						currentNode.N++
					}
					next++
					continue
				}
			} else if (currentNode.T == NtOneloop || currentNode.T == NtOnelazy) && nextNode.T == NtMulti && currentNode.Ch == nextNode.Str[0] {
				// Coalescing a loop with a subsequent string
				// Determine how many of the multi's characters can be combined.
				// We already checked for the first, so we know it's at least one.
				matchingCharsInMulti := 1
				for matchingCharsInMulti < len(nextNode.Str) && currentNode.Ch == nextNode.Str[matchingCharsInMulti] {
					matchingCharsInMulti++
				}

				if canCombineCounts(currentNode.M, currentNode.N, matchingCharsInMulti, matchingCharsInMulti) {
					// Update the loop's bounds to include those characters from the multi
					currentNode.M += matchingCharsInMulti
					if currentNode.N != math.MaxInt32 {
						currentNode.N += matchingCharsInMulti
					}

					// If it was the full multi, skip/remove the multi and continue processing this loop.
					if len(nextNode.Str) == matchingCharsInMulti {
						next++
						continue
					}

					// Otherwise, trim the characters from the multiple that were absorbed into the loop.
					// If it now only has a single character, it becomes a One.
					if len(nextNode.Str)-matchingCharsInMulti == 1 {
						nextNode.T = NtOne
						nextNode.Ch = nextNode.Str[len(nextNode.Str)-1]
						nextNode.Str = nil
					} else {
						nextNode.Str = nextNode.Str[matchingCharsInMulti:]
					}
				}

				// NOTE: We could add support for coalescing a string with a subsequent loop, but the benefits of that
				// are limited. Pulling a subsequent string's prefix back into the loop helps with making the loop atomic,
				// but if the loop is after the string, pulling the suffix of the string forward into the loop may actually
				// be a deoptimization as those characters could end up matching more slowly as part of loop matching.

			} else if (currentNode.T == NtOne && nextNode.IsOneloopFamily() && currentNode.Ch == nextNode.Ch) ||
				(currentNode.T == NtNotone && nextNode.IsNotoneloopFamily() && currentNode.Ch == nextNode.Ch) ||
				(currentNode.T == NtSet && nextNode.IsSetloopFamily() && currentNode.Set.Equals(nextNode.Set)) {
				// Coalescing an individual item with a loop.
				if canCombineCounts(1, 1, nextNode.M, nextNode.N) {
					currentNode.T = nextNode.T
					currentNode.M = nextNode.M + 1
					if nextNode.N == math.MaxInt32 {
						currentNode.N = math.MaxInt32
					} else {
						currentNode.N = nextNode.N + 1
					}
					next++
					continue
				}
			} else if (currentNode.T == NtNotone && nextNode.T == NtNotone && currentNode.Ch == nextNode.Ch) ||
				(currentNode.T == NtSet && nextNode.T == NtSet && currentNode.Set.Equals(nextNode.Set)) {
				// Coalescing an individual item with another individual item.
				// We don't coalesce adjacent One nodes into a Oneloop as we'd rather they be joined into a Multi.
				currentNode.makeRep(NtOneloop, 2, 2)
				next++
				continue
			}

		End:
		}

		n.Children[nextSave] = n.Children[next]
		nextSave++
		current = next
		next++
	}

	if nextSave < len(n.Children) {
		n.Children = slices.Delete(n.Children, nextSave, len(n.Children))
	}
}

// Basic optimization. Adjacent strings can be concatenated.
//
// (?:abc)(?:def) -> abcdef
func (n *RegexNode) reduceConcatenationWithAdjacentStrings() {
	var optionsLast RegexOptions
	var optionsAt RegexOptions
	var i, j int

	wasLastString := false

	for i, j = 0, 0; i < len(n.Children); i, j = i+1, j+1 {
		var at, prev *RegexNode

		at = n.Children[i]

		if j < i {
			n.Children[j] = at
		}

		if at.T == NtConcatenate &&
			((at.Options & RightToLeft) == (n.Options & RightToLeft)) {
			for k := 0; k < len(at.Children); k++ {
				at.Children[k].Parent = n
			}

			//insert at.children at i+1 index in n.children
			n.insertChildren(i+1, at.Children)

			j--
		} else if at.T == NtMulti || at.T == NtOne {
			// Cannot merge strings if L or I options differ
			optionsAt = at.Options & (RightToLeft | IgnoreCase)

			if !wasLastString || optionsLast != optionsAt {
				wasLastString = true
				optionsLast = optionsAt
				continue
			}

			j--
			prev = n.Children[j]

			if prev.T == NtOne {
				prev.T = NtMulti
				prev.Str = []rune{prev.Ch}
			}

			if (optionsAt & RightToLeft) == 0 {
				if at.T == NtOne {
					prev.Str = append(prev.Str, at.Ch)
				} else {
					prev.Str = append(prev.Str, at.Str...)
				}
			} else {
				if at.T == NtOne {
					// insert at the front by expanding our slice, copying the data over, and then setting the value
					prev.Str = append(prev.Str, 0)
					copy(prev.Str[1:], prev.Str)
					prev.Str[0] = at.Ch
				} else {
					//insert at the front...this one we'll make a new slice and copy both into it
					merge := make([]rune, len(prev.Str)+len(at.Str))
					copy(merge, at.Str)
					copy(merge[len(at.Str):], prev.Str)
					prev.Str = merge
				}
			}
		} else if at.T == NtEmpty {
			j--
		} else {
			wasLastString = false
		}
	}

	if j < i {
		// remove indices j through i from the children
		n.removeChildren(j, i)
	}
}

// Nested repeaters just get multiplied with each other if they're not
// too lumpy
func (n *RegexNode) reduceRep() *RegexNode {

	u := n
	t := n.T
	min := n.M
	max := n.N

	for len(u.Children) > 0 {
		child := u.Children[0]

		// multiply reps of the same type only
		if child.T != t {
			valid := false
			if t == NtLoop {
				switch child.T {
				case NtOneloop, NtOneloopatomic, NtNotoneloop,
					NtNotoneloopatomic, NtSetloop, NtSetloopatomic:
					valid = true
				}
			} else {
				switch child.T {
				case NtOnelazy, NtNotonelazy, NtSetlazy:
					valid = true
				}
			}
			if !valid {
				break
			}
		}

		// child can be too lumpy to blur, e.g., (a {100,105}) {3} or (a {2,})?
		// [but things like (a {2,})+ are not too lumpy...]
		if u.M == 0 && child.M > 1 || maxLessThanTwiceMin(child.N, child.M) {
			break
		}

		u = child
		if u.M > 0 {
			if (math.MaxInt32-1)/u.M < min {
				u.M = math.MaxInt32
			} else {
				u.M *= min
			}
		}
		if u.N > 0 {
			if (math.MaxInt32-1)/u.N < max {
				u.N = math.MaxInt32
			} else {
				u.N *= max
			}
		}
	}

	if min == math.MaxInt32 {
		return newRegexNode(NtNothing, n.Options)
	}

	// If the Loop or Lazyloop now only has one child node and its a Set, One, or Notone,
	// reduce to just Setloop/lazy, Oneloop/lazy, or Notoneloop/lazy.  The parser will
	// generally have only produced the latter, but other reductions could have exposed
	// this.
	if len(u.Children) == 1 {
		child := u.Children[0]
		switch child.T {
		case NtOne, NtNotone, NtSet:
			if u.T == NtLazyloop {
				child.makeRep(NtOnelazy, u.M, u.N)
			} else {
				child.makeRep(NtOneloop, u.M, u.N)
			}
			u = child
		}
	}

	return u

}

// Simple optimization. If a concatenation or alternation has only
// one child strip out the intermediate node. If it has zero children,
// turn it into an empty.
func (n *RegexNode) replaceNodeIfUnnecessary() *RegexNode {
	switch len(n.Children) {
	case 0:
		emptyType := NtEmpty
		if n.T == NtAlternate {
			emptyType = NtNothing
		}
		return newRegexNode(emptyType, n.Options)
	case 1:
		return n.Children[0]
	default:
		return n
	}
}

func (n *RegexNode) reduceGroup() *RegexNode {
	u := n

	for u.T == NtGroup {
		u = u.Children[0]
	}

	return u
}

// Simple optimization. If a set is a singleton, an inverse singleton,
// or empty, it's transformed accordingly.
func (n *RegexNode) reduceSet() *RegexNode {
	// Extract empty-set, one and not-one case as special

	if n.Set == nil {
		n.T = NtNothing
	} else if n.Set.IsSingleton() {
		n.Ch = n.Set.SingletonChar()
		n.Set = nil
		n.T += (NtOne - NtSet)
	} else if n.Set.IsSingletonInverse() {
		n.Ch = n.Set.SingletonChar()
		n.Set = nil
		n.T += (NtNotone - NtSet)
	}

	return n
}

func (n *RegexNode) reverseLeft() *RegexNode {
	if n.Options&RightToLeft != 0 && n.T == NtConcatenate && len(n.Children) > 0 {
		//reverse children order
		for left, right := 0, len(n.Children)-1; left < right; left, right = left+1, right-1 {
			n.Children[left], n.Children[right] = n.Children[right], n.Children[left]
		}
	}

	return n
}

func (n *RegexNode) makeQuantifier(lazy bool, min, max int) *RegexNode {
	// Certain cases of repeaters (min == max) can be handled specially
	if min == 0 && max == 0 {
		// The node is repeated 0 times, so it's actually empty.
		return newRegexNode(NtEmpty, n.Options)
	}

	if min == 1 && max == 1 {
		// The node is repeated 1 time, so it's not actually a repeater.
		return n
	}

	if min == max && max <= MultiVsRepeaterLimit && n.T == NtOne {
		// The same character is repeated a fixed number of times, so it's actually a multi.
		// While this could remain a repeater, multis are more readily optimized later in
		// processing. The counts used here in real-world expressions are invariably small (e.g. 4),
		// but we set an upper bound just to avoid creating really large strings.
		n.T = NtMulti
		n.Str = []rune(strings.Repeat(string(n.Ch), max))
		n.Ch = 0
		return n
	}

	switch n.T {
	case NtOne, NtNotone, NtSet:
		if lazy {
			n.makeRep(NtOnelazy, min, max)
		} else {
			n.makeRep(NtOneloop, min, max)
		}
		return n

	default:
		var t NodeType
		if lazy {
			t = NtLazyloop
		} else {
			t = NtLoop
		}
		result := newRegexNodeMN(t, n.Options, min, max)
		result.addChild(n)
		return result
	}
}

// Computes a min bound on the required length of any string that could possibly match.
// If the result is 0, there is no minimum we can enforce.
func (n *RegexNode) ComputeMinLength() int {
	switch n.T {
	case NtOne, NtNotone, NtSet:
		// single char
		return 1
	case NtMulti:
		// Every character in the string needs to match.
		return len(n.Str)
	case NtNotonelazy, NtNotoneloop, NtNotoneloopatomic,
		NtOnelazy, NtOneloop, NtOneloopatomic, NtSetlazy, NtSetloop, NtSetloopatomic:
		// One character repeated at least M times.
		return n.M
	case NtLazyloop, NtLoop:
		// A node graph repeated at least M times.
		return multiplyMinLength(n.M, n.Children[0].ComputeMinLength())
	case NtAlternate:
		// The minimum required length for any of the alternation's branches.
		childCount := len(n.Children)
		min := n.Children[0].ComputeMinLength()
		for i := 1; i < childCount && min > 0; i++ {
			newMin := n.Children[i].ComputeMinLength()
			if newMin < min {
				min = newMin
			}
		}
		return min
	case NtBackRefCond:
		// Minimum of its yes and no branches.  The backreference doesn't add to the length.
		b1 := n.Children[0].ComputeMinLength()
		if len(n.Children) == 1 {
			return b1
		}
		b2 := n.Children[1].ComputeMinLength()
		if b1 < b2 {
			return b1
		}
		return b2
	case NtExprCond:
		// Minimum of its yes and no branches.  The condition is a zero-width assertion.
		if len(n.Children) == 2 {
			return n.Children[1].ComputeMinLength()
		}
		b1 := n.Children[1].ComputeMinLength()
		b2 := n.Children[2].ComputeMinLength()
		if b1 < b2 {
			return b1
		}
		return b2
	case NtConcatenate:
		// The sum of all of the concatenation's children.
		sum := 0
		for i := 0; i < len(n.Children); i++ {
			sum = addMinLength(sum, n.Children[i].ComputeMinLength())
		}
		return sum
	case NtAtomic, NtCapture, NtGroup:
		// For groups, we just delegate to the sole child.
		return n.Children[0].ComputeMinLength()
	case NtEmpty, NtNothing,
		NtBeginning, NtBol, NtBoundary, NtECMABoundary, NtEnd, NtEndZ, NtEol,
		NtNonboundary, NtNonECMABoundary, NtStart, NtNegLook, NtPosLook, NtRef:
		// Nothing to match. In the future, we could potentially use Nothing to say that the min length
		// is infinite, but that would require a different structure, as that would only apply if the
		// Nothing match is required in all cases (rather than, say, as one branch of an alternation).
	}
	return 0
}

// Computes a maximum length of any string that could possibly match.
// or -1 if the length may not always be the same.
func (n *RegexNode) computeMaxLength() int {
	switch n.T {
	case NtOne, NtNotone, NtSet:
		return 1
	case NtMulti:
		return len(n.Str)
	case NtNotonelazy, NtNotoneloop, NtNotoneloopatomic,
		NtOnelazy, NtOneloop, NtOneloopatomic,
		NtSetlazy, NtSetloop, NtSetloopatomic:
		// Return the max number of iterations if there's an upper bound, or null if it's infinite
		if n.N == math.MaxInt32 {
			return -1
		}
		return n.N
	case NtLazyloop, NtLoop:
		if n.N == math.MaxInt32 {
			return -1
		}
		// A node graph repeated a fixed number of times
		if c := n.Children[0].computeMaxLength(); c >= 0 {
			return multiplyMaxLength(n.N, c)
		}
	case NtAlternate:
		// The maximum length of any child branch, as long as they all have one.
		c := n.Children[0].computeMaxLength()

		if c < 0 {
			return -1
		}
		for i := 1; i < len(n.Children); i++ {
			c2 := n.Children[i].computeMaxLength()
			if c2 < 0 {
				return -1
			}

			c = max(c, c2)
		}
		return c
	case NtBackRefCond:
		// The maximum length of either child branch, as long as they both have one.
		b1 := n.Children[0].computeMaxLength()
		if b1 < 0 {
			return -1
		}
		b2 := n.Children[1].computeMaxLength()
		if b2 < 0 {
			return -1
		}
		return max(b1, b2)

	case NtExprCond:
		// The condition for an expression conditional is a zero-width assertion.
		b1 := n.Children[1].computeMaxLength()
		if b1 < 0 {
			return -1
		}
		b2 := n.Children[2].computeMaxLength()
		if b2 < 0 {
			return -1
		}
		return max(b1, b2)

	case NtConcatenate:
		// The sum of all of the concatenation's children's max lengths, as long as they all have one.
		sum := 0
		for i := 0; i < len(n.Children); i++ {
			c := n.Children[i].computeMaxLength()
			if c < 0 {
				return -1
			}
			sum = addMaxLength(sum, c)
			if sum < 0 {
				return -1
			}
		}
		return sum

	case NtAtomic, NtCapture:
		// For groups, we just delegate to the sole child.
		return n.Children[0].computeMaxLength()
	case NtEmpty, NtNothing, NtUpdateBumpalong,
		NtBeginning, NtBol, NtBoundary, NtECMABoundary, NtEnd, NtEndZ, NtEol,
		NtNonboundary, NtNonECMABoundary, NtStart, NtNegLook, NtPosLook:
		//zero-width
		return 0

	case NtRef:
		// Requires matching data available only at run-time.  In the future, we could choose to find
		// and follow the capture group this aligns with, while being careful not to end up in an
		// infinite cycle.
		return -1
	}

	return -1
}

// debug functions

var typeStr = []string{
	"Onerep", "Notonerep", "Setrep",
	"Oneloop", "Notoneloop", "Setloop",
	"Onelazy", "Notonelazy", "Setlazy",
	"One", "Notone", "Set",
	"Multi", "Ref",
	"Bol", "Eol", "Boundary", "Nonboundary",
	"Beginning", "Start", "EndZ", "End",
	"Nothing", "Empty",
	"Alternate", "Concatenate",
	"Loop", "Lazyloop",
	"Capture", "Group", "PosLook", "NegLook", "Atomic",
	"BackRefCond", "ExprCond",
	"Unknown", "Unknown", "Unknown",
	"Unknown", "Unknown", "Unknown",
	"ECMABoundary", "NonECMABoundary",
	"OneloopAtomic", "NotoneloopAtomic", "SetloopAtomic",
	"UpdateBumpalong",
}

func (n *RegexNode) Description() string {
	buf := &bytes.Buffer{}

	buf.WriteString(typeStr[n.T])

	if (n.Options & ExplicitCapture) != 0 {
		buf.WriteString("-C")
	}
	if (n.Options & IgnoreCase) != 0 {
		buf.WriteString("-I")
	}
	if (n.Options & RightToLeft) != 0 {
		buf.WriteString("-L")
	}
	if (n.Options & Multiline) != 0 {
		buf.WriteString("-M")
	}
	if (n.Options & Singleline) != 0 {
		buf.WriteString("-S")
	}
	if (n.Options & IgnorePatternWhitespace) != 0 {
		buf.WriteString("-X")
	}
	if (n.Options & ECMAScript) != 0 {
		buf.WriteString("-E")
	}

	switch n.T {
	case NtOneloop, NtOneloopatomic, NtNotoneloop, NtOnelazy, NtNotonelazy, NtOne, NtNotone, NtNotoneloopatomic:
		buf.WriteString("(Ch = " + CharDescription(n.Ch) + ")")
	case NtCapture:
		buf.WriteString("(index = " + strconv.Itoa(n.M) + ", unindex = " + strconv.Itoa(n.N) + ")")
	case NtRef, NtBackRefCond:
		buf.WriteString("(index = " + strconv.Itoa(n.M) + ")")
	case NtMulti:
		fmt.Fprintf(buf, "(String = %#v)", string(n.Str))
	case NtSet, NtSetloop, NtSetlazy, NtSetloopatomic:
		buf.WriteString("(Set = " + n.Set.String() + ")")
	}

	switch n.T {
	case NtOneloop, NtNotoneloop, NtOnelazy, NtNotonelazy, NtSetloop, NtSetlazy, NtLoop, NtLazyloop,
		NtOneloopatomic, NtNotoneloopatomic, NtSetloopatomic:

		buf.WriteString("(Min = ")
		buf.WriteString(strconv.Itoa(n.M))
		buf.WriteString(", Max = ")
		if n.N == math.MaxInt32 {
			buf.WriteString("inf")
		} else {
			buf.WriteString(strconv.Itoa(n.N))
		}
		buf.WriteString(")")
	}

	return buf.String()
}

var padSpace = []byte("                                ")

func (t *RegexTree) Dump() string {
	return t.Root.dump()
}

func (n *RegexNode) dump() string {
	var stack []int
	CurNode := n
	CurChild := 0

	buf := bytes.NewBufferString(CurNode.Description())
	buf.WriteRune('\n')

	for {
		if CurNode.Children != nil && CurChild < len(CurNode.Children) {
			stack = append(stack, CurChild+1)
			CurNode = CurNode.Children[CurChild]
			CurChild = 0

			Depth := len(stack)
			if Depth > 32 {
				Depth = 32
			}
			buf.Write(padSpace[:Depth])
			buf.WriteString(CurNode.Description())
			buf.WriteRune('\n')
		} else {
			if len(stack) == 0 {
				break
			}

			CurChild = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			CurNode = CurNode.Parent
		}
	}
	return buf.String()
}

// Determines whether the specified child index of a concatenation begins a sequence whose values
// should be used to perform an ordinal case-insensitive comparison.
//
// When consumeZeroWidthNodes is false, the consumer needs the semantics of matching the produced string to fully represent
// the semantics of all the consumed nodes, which means nodes can be consumed iff they produce text that's represented
// by the resulting string. When true, the resulting string needs to fully represent all valid matches at that position,
// but it can have false positives, which means the resulting string doesn't need to fully represent all zero-width nodes
// consumed. true is only valid when used as part of a search to determine where to try a full match, not as part of
// actual matching logic.
// consumeZeroWidthNodes = false
func (n *RegexNode) TryGetOrdinalCaseInsensitiveString(childIndex int, exclusiveChildBound int, consumeZeroWidthNodes bool) (success bool, nodesConsumed int, caseInsensitiveString string) {
	vsb := &strings.Builder{}

	// We're looking in particular for sets of ASCII characters, so we focus only on sets with two characters in them, e.g. [Aa].
	//twoChars := make([]rune, 0, 2)

	// Iterate from the child index to the exclusive upper bound.
	var i int
	for i = childIndex; i < exclusiveChildBound; i++ {
		child := n.Children[i]

		if child.T == NtOne {
			// We only want to include ASCII characters, and only if they don't participate in case conversion
			// such that they only case to themselves and nothing other cases to them.  Otherwise, including
			// them would potentially cause us to match against things not allowed by the pattern.
			if child.Ch >= unicode.MaxASCII || participatesInCaseConversion(child.Ch) {
				break
			}

			vsb.WriteRune(child.Ch)
		} else if child.T == NtMulti {
			// As with NtOne, the string needs to be composed solely of ASCII characters that
			// don't participate in case conversion.
			hasNonAscii := slices.ContainsFunc(child.Str, func(ch rune) bool { return ch > unicode.MaxASCII })
			if hasNonAscii || anyParticipatesInCaseConversion(string(child.Str)) {
				break
			}

			vsb.WriteString(string(child.Str))
		} else if child.T == NtSet ||
			((child.T == NtSetloop || child.T == NtSetlazy || child.T == NtSetloopatomic) && child.M == child.N) {
			// In particular we want to look for sets that contain only the upper and lowercase variant
			// of the same ASCII letter.
			ok, twoChars := child.Set.containsAsciiIgnoreCaseCharacter()
			if !ok {
				break
			}

			count := child.M
			if child.T == NtSet {
				count = 1
			}
			vsb.WriteString(strings.Repeat(string(twoChars[0]|0x20), count))
		} else if child.T == NtEmpty {
			// Skip over empty nodes, as they're pure nops. They would ideally have been optimized away,
			// but can still remain in some situations.
		} else if consumeZeroWidthNodes &&
			// anchors
			(child.T == NtBeginning || child.T == NtBol || child.T == NtStart ||
				// boundaries
				child.T == NtBoundary || child.T == NtECMABoundary || child.T == NtNonboundary || child.T == NtNonECMABoundary ||
				// lookarounds
				child.T == NtNegLook || child.T == NtPosLook ||
				// logic
				child.T == NtUpdateBumpalong) {
			// Skip over zero-width nodes that might be reasonable at the beginning of or within a substring.
			// We can only do these if consumeZeroWidthNodes is true, as otherwise we'd be producing a string that
			// may not fully represent the semantics of this portion of the pattern.
		} else {
			break
		}
	}

	// If we found at least two characters, consider it a sequence found.  It's possible
	// they all came from the same node, so this could be a sequence of just one node.
	if vsb.Len() >= 2 {
		return true, i - childIndex, vsb.String()
	}

	// No sequence found.
	return false, 0, ""
}

func (child *RegexNode) canJoinLengthCheck() bool {
	if child.T == NtOne || child.T == NtNotone || child.T == NtSet || child.T == NtMulti {
		return true
	}
	if (child.IsSetloopFamily() || child.IsNotoneloopFamily() || child.IsOneloopFamily()) &&
		child.M == child.N {
		return true
	}

	return false
}

// Determine whether the specified child node is the beginning of a sequence that can
// trivially have length checks combined in order to avoid bounds checks.
// requiredLength is The sum of all the fixed lengths for the nodes in the sequence.</param>
// exclusiveEnd is The index of the node just after the last one in the sequence.</param>
// returns true if more than one node can have their length checks combined; otherwise, false.</returns>
//
// There are additional node types for which we can prove a fixed length, e.g. examining all branches
// of an alternation and returning true if all their lengths are equal.  However, the primary purpose
// of this method is to avoid bounds checks by consolidating length checks that guard accesses to
// strings/spans for which the JIT can see a fixed index within bounds, and alternations employ
// patterns that defeat that (e.g. reassigning the span in question).  As such, the implementation
// remains focused on only a core subset of nodes that are a) likely to be used in concatenations and
// b) employ simple patterns of checks.
func (n *RegexNode) TryGetJoinableLengthCheckChildRange(childIndex int, requiredLength *int, exclusiveEnd *int) bool {

	child := n.Children[childIndex]
	if child.canJoinLengthCheck() {
		*requiredLength = child.ComputeMinLength()

		for *exclusiveEnd = childIndex + 1; *exclusiveEnd < len(n.Children); *exclusiveEnd++ {
			child = n.Children[*exclusiveEnd]
			if !child.canJoinLengthCheck() {
				break
			}

			*requiredLength += child.ComputeMinLength()
		}

		if *exclusiveEnd-childIndex > 1 {
			return true
		}
	}

	*requiredLength = 0
	*exclusiveEnd = 0
	return false
}

type StartingLiteral struct {
	Range    SingleRange
	String   []rune
	SetChars []rune
	Negated  bool
}

func (n *RegexNode) FindStartingLiteral() *StartingLiteral {
	node := n.FindStartingLiteralNode(true)
	if node == nil {
		return nil
	}

	if node.IsOneFamily() {
		return &StartingLiteral{Range: SingleRange{node.Ch, node.Ch}, Negated: false}
	}
	if node.IsNotoneFamily() {
		return &StartingLiteral{Range: SingleRange{node.Ch, node.Ch}, Negated: true}
	}
	if node.IsSetFamily() {
		ranges := node.Set.GetIfNRanges(1)
		if len(ranges) == 1 && ranges[0].Last-ranges[0].First > 1 {
			return &StartingLiteral{Range: ranges[0], Negated: node.Set.IsNegated()}
		}
		setChars := node.Set.GetSetChars(128)
		if len(setChars) > 0 {
			return &StartingLiteral{SetChars: setChars, Negated: node.Set.IsNegated()}
		}
	}
	if node.T == NtMulti {
		return &StartingLiteral{String: node.Str}
	}

	return nil
}

// Finds the guaranteed beginning literal(s) of the node, or null if none exists.
// allowZeroWidth = true
func (n *RegexNode) FindStartingLiteralNode(allowZeroWidth bool) *RegexNode {
	node := n
	for {
		if node != nil && node.Options&RightToLeft == 0 {
			switch node.T {
			case NtOne, NtNotone, NtMulti, NtSet:
				return node

			case NtOneloop, NtOneloopatomic, NtOnelazy,
				NtNotoneloop, NtNotoneloopatomic, NtNotonelazy,
				NtSetloop, NtSetloopatomic, NtSetlazy:
				if node.M > 0 {
					return node
				}

			case NtAtomic, NtConcatenate, NtCapture, NtGroup:
				node = node.Children[0]
				continue
			case NtLoop, NtLazyloop:
				node = node.Children[0]
				continue
			case NtPosLook:
				if allowZeroWidth {
					node = node.Children[0]
					continue
				}
			}
		}

		return nil
	}
}

// Gets the character that begins a One or Multi.
func (n *RegexNode) FirstCharOfOneOrMulti() rune {
	if n.IsOneFamily() {
		return n.Ch
	}
	return n.Str[0]
}
