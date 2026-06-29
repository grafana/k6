package syntax

import (
	"bytes"
	"fmt"
	"math"
)

func Write(tree *RegexTree) (*Code, error) {
	w := writer{
		intStack:   make([]int, 0, 32),
		emitted:    make([]int, 2),
		stringhash: make(map[string]int),
		sethash:    make(map[string]int),
	}

	code, err := w.codeFromTree(tree)

	return code, err
}

type writer struct {
	emitted []int

	intStack    []int
	curpos      int
	stringhash  map[string]int
	stringtable [][]rune
	sethash     map[string]int
	settable    []*CharSet
	counting    bool
	count       int
	trackcount  int
	caps        map[int]int
}

const (
	BeforeChild NodeType = 64
	AfterChild  NodeType = 128
	//MaxPrefixSize is the largest number of runes we'll use for a BoyerMoyer prefix
	MaxPrefixSize = 50
)

// The top level RegexCode generator. It does a depth-first walk
// through the tree and calls EmitFragment to emits code before
// and after each child of an interior node, and at each leaf.
//
// It runs two passes, first to count the size of the generated
// code, and second to generate the code.
//
// We should time it against the alternative, which is
// to just generate the code and grow the array as we go.
func (w *writer) codeFromTree(tree *RegexTree) (*Code, error) {
	var (
		curNode  *RegexNode
		curChild int
		capsize  int
	)
	// construct sparse capnum mapping if some numbers are unused

	if tree.Capnumlist == nil || tree.Captop == len(tree.Capnumlist) {
		capsize = tree.Captop
		w.caps = nil
	} else {
		capsize = len(tree.Capnumlist)
		w.caps = tree.Caps
		for i := 0; i < len(tree.Capnumlist); i++ {
			w.caps[tree.Capnumlist[i]] = i
		}
	}

	w.counting = true

	for {
		if !w.counting {
			w.emitted = make([]int, w.count)
		}

		curNode = tree.Root
		curChild = 0

		w.emit1(Lazybranch, 0)

		for {
			if len(curNode.Children) == 0 {
				if err := w.emitFragment(curNode.T, curNode, 0); err != nil {
					return nil, err
				}
			} else if curChild < len(curNode.Children) {
				if err := w.emitFragment(curNode.T|BeforeChild, curNode, curChild); err != nil {
					return nil, err
				}

				curNode = curNode.Children[curChild]

				w.pushInt(curChild)
				curChild = 0
				continue
			}

			if w.emptyStack() {
				break
			}

			curChild = w.popInt()
			curNode = curNode.Parent

			if err := w.emitFragment(curNode.T|AfterChild, curNode, curChild); err != nil {
				return nil, err
			}
			curChild++
		}

		w.patchJump(0, w.curPos())
		w.emit(Stop)

		if !w.counting {
			break
		}

		w.counting = false
	}

	fcPrefix := getFirstCharsPrefix(tree)
	prefix := getPrefix(tree)
	rtl := (tree.Options & RightToLeft) != 0

	var bmPrefix *BmPrefix
	//TODO: benchmark string prefixes
	if prefix != nil && len(prefix.PrefixStr) > 0 && MaxPrefixSize > 0 {
		if len(prefix.PrefixStr) > MaxPrefixSize {
			// limit prefix changes to 10k
			prefix.PrefixStr = prefix.PrefixStr[:MaxPrefixSize]
		}
		bmPrefix = newBmPrefix(prefix.PrefixStr, prefix.CaseInsensitive, rtl)
	} else {
		bmPrefix = nil
	}

	return &Code{
		Codes:             w.emitted,
		Strings:           w.stringtable,
		Sets:              w.settable,
		TrackCount:        w.trackcount,
		Caps:              w.caps,
		Capsize:           capsize,
		FcPrefix:          fcPrefix,
		BmPrefix:          bmPrefix,
		Anchors:           getAnchors(tree),
		RightToLeft:       rtl,
		FindOptimizations: tree.FindOptimizations,
	}, nil
}

// The main RegexCode generator. It does a depth-first walk
// through the tree and calls EmitFragment to emits code before
// and after each child of an interior node, and at each leaf.
func (w *writer) emitFragment(nodetype NodeType, node *RegexNode, curIndex int) error {
	bits := InstOp(0)

	if (node.Options & RightToLeft) != 0 {
		bits |= Rtl
	}
	if (node.Options & IgnoreCase) != 0 {
		bits |= Ci
	}

	ntBits := NodeType(bits)

	switch nodetype {
	case NtConcatenate | BeforeChild, NtConcatenate | AfterChild, NtEmpty:

	case NtAlternate | BeforeChild:
		if curIndex < len(node.Children)-1 {
			w.pushInt(w.curPos())
			w.emit1(Lazybranch, 0)
		}

	case NtAlternate | AfterChild:
		if curIndex < len(node.Children)-1 {
			lbPos := w.popInt()
			w.pushInt(w.curPos())
			w.emit1(Goto, 0)
			w.patchJump(lbPos, w.curPos())
		} else {
			for i := 0; i < curIndex; i++ {
				w.patchJump(w.popInt(), w.curPos())
			}
		}

	case NtBackRefCond | BeforeChild:
		if curIndex == 0 {
			w.emit(Setjump)
			w.pushInt(w.curPos())
			w.emit1(Lazybranch, 0)
			w.emit1(Testref, w.mapCapnum(node.M))
			w.emit(Forejump)
		}

	case NtBackRefCond | AfterChild:
		switch curIndex {
		case 0:
			branchpos := w.popInt()
			w.pushInt(w.curPos())
			w.emit1(Goto, 0)
			w.patchJump(branchpos, w.curPos())
			w.emit(Forejump)
			if len(node.Children) <= 1 {
				w.patchJump(w.popInt(), w.curPos())
			}
		case 1:
			w.patchJump(w.popInt(), w.curPos())
		}

	case NtExprCond | BeforeChild:
		if curIndex == 0 {
			w.emit(Setjump)
			w.emit(Setmark)
			w.pushInt(w.curPos())
			w.emit1(Lazybranch, 0)
		}

	case NtExprCond | AfterChild:
		switch curIndex {
		case 0:
			w.emit(Getmark)
			w.emit(Forejump)
		case 1:
			branchpos := w.popInt()
			w.pushInt(w.curPos())
			w.emit1(Goto, 0)
			w.patchJump(branchpos, w.curPos())
			w.emit(Getmark)
			w.emit(Forejump)
			if len(node.Children) <= 2 {
				w.patchJump(w.popInt(), w.curPos())
			}
		case 2:
			w.patchJump(w.popInt(), w.curPos())
		}

	case NtLoop | BeforeChild, NtLazyloop | BeforeChild:

		if node.N < math.MaxInt32 || node.M > 1 {
			if node.M == 0 {
				w.emit1(Nullcount, 0)
			} else {
				w.emit1(Setcount, 1-node.M)
			}
		} else if node.M == 0 {
			w.emit(Nullmark)
		} else {
			w.emit(Setmark)
		}

		if node.M == 0 {
			w.pushInt(w.curPos())
			w.emit1(Goto, 0)
		}
		w.pushInt(w.curPos())

	case NtLoop | AfterChild, NtLazyloop | AfterChild:

		startJumpPos := w.curPos()
		lazy := InstOp(nodetype - (NtLoop | AfterChild))

		if node.N < math.MaxInt32 || node.M > 1 {
			if node.N == math.MaxInt32 {
				w.emit2(Branchcount+lazy, w.popInt(), math.MaxInt32)
			} else {
				w.emit2(Branchcount+lazy, w.popInt(), node.N-node.M)
			}
		} else {
			w.emit1(Branchmark+lazy, w.popInt())
		}

		if node.M == 0 {
			w.patchJump(w.popInt(), startJumpPos)
		}

	case NtGroup | BeforeChild, NtGroup | AfterChild:

	case NtCapture | BeforeChild:
		w.emit(Setmark)

	case NtCapture | AfterChild:
		w.emit2(Capturemark, w.mapCapnum(node.M), w.mapCapnum(node.N))

	case NtPosLook | BeforeChild:
		// NOTE: the following line causes lookahead/lookbehind to be
		// NON-BACKTRACKING. It can be commented out with (*)
		w.emit(Setjump)

		w.emit(Setmark)

	case NtPosLook | AfterChild:
		w.emit(Getmark)

		// NOTE: the following line causes lookahead/lookbehind to be
		// NON-BACKTRACKING. It can be commented out with (*)
		w.emit(Forejump)

	case NtNegLook | BeforeChild:
		w.emit(Setjump)
		w.pushInt(w.curPos())
		w.emit1(Lazybranch, 0)

	case NtNegLook | AfterChild:
		w.emit(Backjump)
		w.patchJump(w.popInt(), w.curPos())
		w.emit(Forejump)

	case NtAtomic | BeforeChild:
		w.emit(Setjump)

	case NtAtomic | AfterChild:
		w.emit(Forejump)

	case NtOne, NtNotone:
		w.emit1(InstOp(node.T|ntBits), int(node.Ch))

	case NtNotoneloop, NtNotoneloopatomic, NtNotonelazy, NtOneloop, NtOneloopatomic, NtOnelazy:
		if node.M > 0 {
			if node.T == NtOneloop || node.T == NtOnelazy || node.T == NtOneloopatomic {
				w.emit2(Onerep|bits, int(node.Ch), node.M)
			} else {
				w.emit2(Notonerep|bits, int(node.Ch), node.M)
			}
		}
		if node.N > node.M {
			if node.N == math.MaxInt32 {
				w.emit2(InstOp(node.T|ntBits), int(node.Ch), math.MaxInt32)
			} else {
				w.emit2(InstOp(node.T|ntBits), int(node.Ch), node.N-node.M)
			}
		}

	case NtSetloop, NtSetlazy, NtSetloopatomic:
		if node.M > 0 {
			w.emit2(Setrep|bits, w.setCode(node.Set), node.M)
		}
		if node.N > node.M {
			if node.N == math.MaxInt32 {
				w.emit2(InstOp(node.T|ntBits), w.setCode(node.Set), math.MaxInt32)
			} else {
				w.emit2(InstOp(node.T|ntBits), w.setCode(node.Set), node.N-node.M)
			}
		}

	case NtMulti:
		w.emit1(InstOp(node.T|ntBits), w.stringCode(node.Str))

	case NtSet:
		w.emit1(InstOp(node.T|ntBits), w.setCode(node.Set))

	case NtRef:
		w.emit1(InstOp(node.T|ntBits), w.mapCapnum(node.M))

	case NtNothing, NtBol, NtEol, NtBoundary, NtNonboundary, NtECMABoundary, NtNonECMABoundary, NtBeginning, NtStart, NtEndZ, NtEnd, NtUpdateBumpalong:
		w.emit(InstOp(node.T))

	default:
		return fmt.Errorf("unexpected opcode in regular expression generation: %v", nodetype)
	}

	return nil
}

// To avoid recursion, we use a simple integer stack.
// This is the push.
func (w *writer) pushInt(i int) {
	w.intStack = append(w.intStack, i)
}

// Returns true if the stack is empty.
func (w *writer) emptyStack() bool {
	return len(w.intStack) == 0
}

// This is the pop.
func (w *writer) popInt() int {
	//get our item
	idx := len(w.intStack) - 1
	i := w.intStack[idx]
	//trim our slice
	w.intStack = w.intStack[:idx]
	return i
}

// Returns the current position in the emitted code.
func (w *writer) curPos() int {
	return w.curpos
}

// Fixes up a jump instruction at the specified offset
// so that it jumps to the specified jumpDest.
func (w *writer) patchJump(offset, jumpDest int) {
	w.emitted[offset+1] = jumpDest
}

// Returns an index in the set table for a charset
// uses a map to eliminate duplicates.
func (w *writer) setCode(set *CharSet) int {
	if w.counting {
		return 0
	}

	buf := &bytes.Buffer{}

	set.mapHashFill(buf)
	hash := buf.String()
	i, ok := w.sethash[hash]
	if !ok {
		i = len(w.sethash)
		w.sethash[hash] = i
		w.settable = append(w.settable, set)
	}
	return i
}

// Returns an index in the string table for a string.
// uses a map to eliminate duplicates.
func (w *writer) stringCode(str []rune) int {
	if w.counting {
		return 0
	}

	hash := string(str)
	i, ok := w.stringhash[hash]
	if !ok {
		i = len(w.stringhash)
		w.stringhash[hash] = i
		w.stringtable = append(w.stringtable, str)
	}

	return i
}

// When generating code on a regex that uses a sparse set
// of capture slots, we hash them to a dense set of indices
// for an array of capture slots. Instead of doing the hash
// at match time, it's done at compile time, here.
func (w *writer) mapCapnum(capnum int) int {
	if capnum == -1 {
		return -1
	}

	if w.caps != nil {
		return w.caps[capnum]
	}

	return capnum
}

// Emits a zero-argument operation. Note that the emit
// functions all run in two modes: they can emit code, or
// they can just count the size of the code.
func (w *writer) emit(op InstOp) {
	if w.counting {
		w.count++
		if opcodeBacktracks(op) {
			w.trackcount++
		}
		return
	}
	w.emitted[w.curpos] = int(op)
	w.curpos++
}

// Emits a one-argument operation.
func (w *writer) emit1(op InstOp, opd1 int) {
	if w.counting {
		w.count += 2
		if opcodeBacktracks(op) {
			w.trackcount++
		}
		return
	}
	w.emitted[w.curpos] = int(op)
	w.curpos++
	w.emitted[w.curpos] = opd1
	w.curpos++
}

// Emits a two-argument operation.
func (w *writer) emit2(op InstOp, opd1, opd2 int) {
	if w.counting {
		w.count += 3
		if opcodeBacktracks(op) {
			w.trackcount++
		}
		return
	}
	w.emitted[w.curpos] = int(op)
	w.curpos++
	w.emitted[w.curpos] = opd1
	w.curpos++
	w.emitted[w.curpos] = opd2
	w.curpos++
}
