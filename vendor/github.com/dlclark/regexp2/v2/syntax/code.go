package syntax

import (
	"bytes"
	"fmt"
	"math"
)

// similar to prog.go in the go regex package...also with comment 'may not belong in this package'

// File provides operator constants for use by the Builder and the Machine.

// Implementation notes:
//
// Regexps are built into RegexCodes, which contain an operation array,
// a string table, and some constants.
//
// Each operation is one of the codes below, followed by the integer
// operands specified for each op.
//
// Strings and sets are indices into a string table.

type InstOp int

const (
	// 					    lef/back operands        description

	Onerep    InstOp = 0 // lef,back char,min,max    a {n}
	Notonerep InstOp = 1 // lef,back char,min,max    .{n}
	Setrep    InstOp = 2 // lef,back set,min,max     [\d]{n}

	Oneloop    InstOp = 3 // lef,back char,min,max    a {,n}
	Notoneloop InstOp = 4 // lef,back char,min,max    .{,n}
	Setloop    InstOp = 5 // lef,back set,min,max     [\d]{,n}

	Onelazy    InstOp = 6 // lef,back char,min,max    a {,n}?
	Notonelazy InstOp = 7 // lef,back char,min,max    .{,n}?
	Setlazy    InstOp = 8 // lef,back set,min,max     [\d]{,n}?

	One    InstOp = 9  // lef      char            a
	Notone InstOp = 10 // lef      char            [^a]
	Set    InstOp = 11 // lef      set             [a-z\s]  \w \s \d

	Multi InstOp = 12 // lef      string          abcd
	Ref   InstOp = 13 // lef      group           \#

	Bol         InstOp = 14 //                          ^
	Eol         InstOp = 15 //                          $
	Boundary    InstOp = 16 //                          \b
	Nonboundary InstOp = 17 //                          \B
	Beginning   InstOp = 18 //                          \A
	Start       InstOp = 19 //                          \G
	EndZ        InstOp = 20 //                          \Z
	End         InstOp = 21 //                          \Z

	Nothing InstOp = 22 //                          Reject!

	// Primitive control structures

	Lazybranch      InstOp = 23 // back     jump            straight first
	Branchmark      InstOp = 24 // back     jump            branch first for loop
	Lazybranchmark  InstOp = 25 // back     jump            straight first for loop
	Nullcount       InstOp = 26 // back     val             set counter, null mark
	Setcount        InstOp = 27 // back     val             set counter, make mark
	Branchcount     InstOp = 28 // back     jump,limit      branch++ if zero<=c<limit
	Lazybranchcount InstOp = 29 // back     jump,limit      same, but straight first
	Nullmark        InstOp = 30 // back                     save position
	Setmark         InstOp = 31 // back                     save position
	Capturemark     InstOp = 32 // back     group           define group
	Getmark         InstOp = 33 // back                     recall position
	Setjump         InstOp = 34 // back                     save backtrack state
	Backjump        InstOp = 35 //                          zap back to saved state
	Forejump        InstOp = 36 //                          zap backtracking state
	Testref         InstOp = 37 //                          backtrack if ref undefined
	Goto            InstOp = 38 //          jump            just go

	Prune InstOp = 39 //                          prune it baby
	Stop  InstOp = 40 //                          done!

	ECMABoundary    InstOp = 41 //                          \b
	NonECMABoundary InstOp = 42 //                          \B

	// Atomic loop of the specified character.
	// Operand 0 is the character. Operand 1 is the max iteration count.
	Oneloopatomic InstOp = 43
	// Atomic loop of a single character other than the one specified.
	// Operand 0 is the character. Operand 1 is the max iteration count.
	Notoneloopatomic InstOp = 44
	// Atomic loop of a single character matching the specified set
	// Operand 0 is index into the strings table of the character class description. Operand 1 is the repetition count.
	Setloopatomic InstOp = 45
	// Updates the bumpalong position to the current position.
	UpdateBumpalong InstOp = 46

	// Modifiers for alternate modes

	Mask  InstOp = 63  // Mask to get unmodified ordinary operator
	Rtl   InstOp = 64  // bit to indicate that we're reverse scanning.
	Back  InstOp = 128 // bit to indicate that we're backtracking.
	Back2 InstOp = 256 // bit to indicate that we're backtracking on a second branch.
	Ci    InstOp = 512 // bit to indicate that we're case-insensitive.
)

type Code struct {
	Codes             []int              // the code
	Strings           [][]rune           // string table
	Sets              []*CharSet         //character set table
	TrackCount        int                // how many instructions use backtracking
	Caps              map[int]int        // mapping of user group numbers -> impl group slots
	Capsize           int                // number of impl group slots
	FcPrefix          *Prefix            // the set of candidate first characters (may be null)
	BmPrefix          *BmPrefix          // the fixed prefix string as a Boyer-Moore machine (may be null)
	Anchors           AnchorLoc          // the set of zero-length start anchors (RegexFCD.Bol, etc)
	RightToLeft       bool               // true if right to left
	FindOptimizations *FindOptimizations // analyzed candidate search strategy
	QuickCodes        []int              // bool-only code with unobservable captures removed
	CaptureSlotInUse  []bool             // capture slots observable by the pattern itself during quick matches
}

// captureSlotsInUse returns the capture slots whose values can affect matching.
// Group 0 is always retained as the success marker. Ordinary captures that are
// never referenced by the pattern may be omitted by bool-only matching APIs.
func captureSlotsInUse(codes []int, capsize int) []bool {
	inUse := make([]bool, capsize)
	if capsize > 0 {
		inUse[0] = true
	}
	for pos := 0; pos < len(codes); {
		op := InstOp(codes[pos]) & Mask
		switch op {
		case Ref, Testref:
			capnum := codes[pos+1]
			if capnum >= 0 && capnum < len(inUse) {
				inUse[capnum] = true
			}
		case Capturemark:
			// Balancing groups both observe and mutate capture state. Keep both
			// sides live even if no later backreference refers to them.
			if codes[pos+2] != -1 {
				for _, capnum := range codes[pos+1 : pos+3] {
					if capnum >= 0 && capnum < len(inUse) {
						inUse[capnum] = true
					}
				}
			}
		}
		pos += opcodeSize(op)
	}
	return inUse
}

// PrepareCharSetASCIIBitmaps builds bounded ASCII lookup tables for compiled
// character classes before the regexp is shared across goroutines.
func (c *Code) PrepareCharSetASCIIBitmaps() {
	if c == nil {
		return
	}
	for _, set := range c.Sets {
		set.prepareASCIIBitmap()
	}
	if c.FcPrefix != nil {
		c.FcPrefix.PrefixSet.prepareASCIIBitmap()
	}
	if c.FindOptimizations != nil {
		for _, set := range c.FindOptimizations.FixedDistanceSets {
			set.Set.prepareASCIIBitmap()
		}
		if c.FindOptimizations.LiteralAfterLoop != nil && c.FindOptimizations.LiteralAfterLoop.LoopNode != nil {
			c.FindOptimizations.LiteralAfterLoop.LoopNode.Set.prepareASCIIBitmap()
		}
	}
}

func opcodeBacktracks(op InstOp) bool {
	op &= Mask

	switch op {
	case Oneloop, Notoneloop, Setloop, Onelazy, Notonelazy, Setlazy, Lazybranch, Branchmark, Lazybranchmark,
		Nullcount, Setcount, Branchcount, Lazybranchcount, Setmark, Capturemark, Getmark, Setjump, Backjump,
		Forejump, Goto:
		return true

	default:
		return false
	}
}

func opcodeSize(op InstOp) int {
	op &= Mask

	switch op {
	case Nothing, Bol, Eol, Boundary, Nonboundary, ECMABoundary, NonECMABoundary, Beginning, Start, EndZ,
		End, Nullmark, Setmark, Getmark, Setjump, Backjump, Forejump, Stop, UpdateBumpalong:
		return 1

	case One, Notone, Multi, Ref, Testref, Goto, Nullcount, Setcount, Lazybranch, Branchmark, Lazybranchmark,
		Prune, Set:
		return 2

	case Capturemark, Branchcount, Lazybranchcount, Onerep, Notonerep, Oneloop, Notoneloop, Onelazy, Notonelazy,
		Setlazy, Setrep, Setloop, Oneloopatomic, Notoneloopatomic, Setloopatomic:
		return 3

	default:
		panic(fmt.Errorf("unexpected op code: %v", op))
	}
}

var codeStr = []string{
	"Onerep", "Notonerep", "Setrep",
	"Oneloop", "Notoneloop", "Setloop",
	"Onelazy", "Notonelazy", "Setlazy",
	"One", "Notone", "Set",
	"Multi", "Ref",
	"Bol", "Eol", "Boundary", "Nonboundary", "Beginning", "Start", "EndZ", "End",
	"Nothing",
	"Lazybranch", "Branchmark", "Lazybranchmark",
	"Nullcount", "Setcount", "Branchcount", "Lazybranchcount",
	"Nullmark", "Setmark", "Capturemark", "Getmark",
	"Setjump", "Backjump", "Forejump", "Testref", "Goto",
	"Prune", "Stop",
	"ECMABoundary", "NonECMABoundary",
	"Oneloopatomic", "Notoneloopatomic", "Setloopatomic",
	"Bumpalong",
}

func operatorDescription(op InstOp) string {
	desc := codeStr[op&Mask]
	if (op & Ci) != 0 {
		desc += "-Ci"
	}
	if (op & Rtl) != 0 {
		desc += "-Rtl"
	}
	if (op & Back) != 0 {
		desc += "-Back"
	}
	if (op & Back2) != 0 {
		desc += "-Back2"
	}

	return desc
}

// OpcodeDescription is a humman readable string of the specific offset
func (c *Code) OpcodeDescription(offset int) string {
	buf := &bytes.Buffer{}

	op := InstOp(c.Codes[offset])
	fmt.Fprintf(buf, "%06d ", offset)

	if opcodeBacktracks(op & Mask) {
		buf.WriteString("*")
	} else {
		buf.WriteString(" ")
	}
	buf.WriteString(operatorDescription(op))
	buf.WriteString("(")
	op &= Mask

	switch op {
	case One, Notone, Onerep, Notonerep, Oneloop, Notoneloop, Onelazy, Notonelazy,
		Oneloopatomic, Notoneloopatomic:
		buf.WriteString("Ch = ")
		buf.WriteString(CharDescription(rune(c.Codes[offset+1])))

	case Set, Setrep, Setloop, Setlazy, Setloopatomic:
		buf.WriteString("Set = ")
		buf.WriteString(c.Sets[c.Codes[offset+1]].String())

	case Multi:
		fmt.Fprintf(buf, "String = %s", string(c.Strings[c.Codes[offset+1]]))

	case Ref, Testref:
		fmt.Fprintf(buf, "Index = %d", c.Codes[offset+1])

	case Capturemark:
		fmt.Fprintf(buf, "Index = %d", c.Codes[offset+1])
		if c.Codes[offset+2] != -1 {
			fmt.Fprintf(buf, ", Unindex = %d", c.Codes[offset+2])
		}

	case Nullcount, Setcount:
		fmt.Fprintf(buf, "Value = %d", c.Codes[offset+1])

	case Goto, Lazybranch, Branchmark, Lazybranchmark, Branchcount, Lazybranchcount:
		fmt.Fprintf(buf, "Addr = %d", c.Codes[offset+1])
	}

	switch op {
	case Onerep, Notonerep, Oneloop, Notoneloop, Onelazy, Notonelazy, Setrep, Setloop, Setlazy,
		Oneloopatomic, Notoneloopatomic, Setloopatomic:
		buf.WriteString(", Rep = ")
		if c.Codes[offset+2] == math.MaxInt32 {
			buf.WriteString("inf")
		} else {
			fmt.Fprintf(buf, "%d", c.Codes[offset+2])
		}

	case Branchcount, Lazybranchcount:
		buf.WriteString(", Limit = ")
		if c.Codes[offset+2] == math.MaxInt32 {
			buf.WriteString("inf")
		} else {
			fmt.Fprintf(buf, "%d", c.Codes[offset+2])
		}

	}

	buf.WriteString(")")

	return buf.String()
}

func (c *Code) Dump() string {
	buf := &bytes.Buffer{}

	if c.RightToLeft {
		fmt.Fprintln(buf, "Direction:  right-to-left")
	} else {
		fmt.Fprintln(buf, "Direction:  left-to-right")
	}
	if c.FcPrefix == nil {
		fmt.Fprintln(buf, "Firstchars: n/a")
	} else {
		fmt.Fprintf(buf, "Firstchars: %v\n", c.FcPrefix.PrefixSet.String())
	}

	if c.BmPrefix == nil {
		fmt.Fprintln(buf, "Prefix:     n/a")
	} else {
		fmt.Fprintf(buf, "Prefix:     %v\n", Escape(c.BmPrefix.String()))
	}

	fmt.Fprintf(buf, "Anchors:    %v\n", c.Anchors)
	if c.FindOptimizations != nil {
		fmt.Fprint(buf, c.FindOptimizations.Dump())
	}
	fmt.Fprintln(buf)

	if c.BmPrefix != nil {
		fmt.Fprintln(buf, "BoyerMoore:")
		fmt.Fprintln(buf, c.BmPrefix.Dump("    "))
	}
	for i := 0; i < len(c.Codes); i += opcodeSize(InstOp(c.Codes[i])) {
		fmt.Fprintln(buf, c.OpcodeDescription(i))
	}

	return buf.String()
}
