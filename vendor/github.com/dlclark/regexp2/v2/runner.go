package regexp2

import (
	"bytes"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/dlclark/regexp2/v2/helpers"
	"github.com/dlclark/regexp2/v2/syntax"
)

type Runner struct {
	re    *Regexp
	code  *syntax.Code
	debug bool

	Runtextstart int // starting point for search

	Runtext    []rune // text to search
	Runtextpos int    // current position in text
	Runtextend int

	// The backtracking stack.  Opcodes use this to store data regarding
	// what they have matched and where to backtrack to.  Each "frame" on
	// the stack takes the form of [CodePosition Data1 Data2...], where
	// CodePosition is the position of the current opcode and
	// the data values are all optional.  The CodePosition can be negative, and
	// these values (also called "back2") are used by the BranchMark family of opcodes
	// to indicate whether they are backtracking after a successful or failed
	// match.
	// When we backtrack, we pop the CodePosition off the stack, set the current
	// instruction pointer to that code position, and mark the opcode
	// with a backtracking flag ("Back").  Each opcode then knows how to
	// handle its own data.
	runtrack    []int
	Runtrackpos int

	// This stack is used to track text positions across different opcodes.
	// For example, in /(a*b)+/, the parentheses result in a SetMark/CaptureMark
	// pair. SetMark records the text position before we match a*b.  Then
	// CaptureMark uses that position to figure out where the capture starts.
	// Opcodes which push onto this stack are always paired with other opcodes
	// which will pop the value from it later.  A successful match should mean
	// that this stack is empty.
	runstack    []int
	Runstackpos int

	// The crawl stack is used to keep track of captures.  Every time a group
	// has a capture, we push its group number onto the runcrawl stack.  In
	// the case of a balanced match, we push BOTH groups onto the stack.
	runcrawl    []int
	runcrawlpos int

	runtrackcount int // count of states that may do backtracking

	runmatch *Match // result object

	ignoreTimeout bool
	timeout       time.Duration // timeout in milliseconds (needed for actual)
	deadline      fasttime

	operator        syntax.InstOp
	codepos         int
	rightToLeft     bool
	caseInsensitive bool
}

// run searches for matches and can continue from the previous match.
//
// quick is usually false, but can be true to not return matches, just put it in caches.
// textstart is -1 to start at the "beginning" (depending on Right-To-Left), otherwise an index in input.
// textInfo is nil for quick scans that do not need returned capture text metadata.
func (re *Regexp) run(quick bool, textstart int, input []rune, textInfo *matchText) (*Match, error) {

	// get a cached runner
	runner := re.getRunner()
	defer re.putRunner(runner)

	if textstart < 0 {
		if re.RightToLeft() {
			textstart = len(input)
		} else {
			textstart = 0
		}
	}
	if quick && textInfo == nil && re.quickCode != nil {
		runner.code = re.quickCode
	}

	return runner.scan(input, textInfo, textstart, quick, re.MatchTimeout)
}

// Scans the string to find the first match. Uses the Match object
// both to feed text in and as a place to store matches that come out.
//
// All the action is in the Go() method. Our
// responsibility is to load up the class members before
// calling Go.
//
// The optimizer can compute a set of candidate starting characters,
// and we could use a separate method Skip() that will quickly scan past
// any characters that we know can't match.
//
// The input slice is passed separately from matchText so quick scans can avoid
// allocating match metadata. When textInfo is nil, successful matches are only
// used as a boolean result and capture text is intentionally unavailable. If
// we collapsed down to just textInfo it would "escape" and hit the GC for fast
// scans without captures.
func (r *Runner) scan(rt []rune, textInfo *matchText, textstart int, quick bool, timeout time.Duration) (*Match, error) {
	r.timeout = timeout
	r.ignoreTimeout = (time.Duration(math.MaxInt64) == timeout)
	r.debug = r.re.Debug()
	r.Runtextstart = textstart
	r.Runtext = rt
	r.Runtextend = len(rt)
	// Some internal callers use quick match tidying while still consuming
	// capture data (notably replacement). Capture elision is only safe when no
	// match text metadata was requested.

	stoppos := r.Runtextend
	bump := 1

	if r.re.RightToLeft() {
		bump = -1
		stoppos = 0
	}

	r.Runtextpos = textstart
	//initted := false

	// setup our scanner functions
	findFirstChar := r.re.findFirstChar
	execute := r.re.execute
	if quick && textInfo == nil && r.re.executeQuick != nil {
		execute = r.re.executeQuick
	}
	if findFirstChar == nil {
		findFirstChar = findFirstCharDefault
	}
	if execute == nil {
		execute = executeDefault
	}

	minRequiredLength := 0
	if r.code != nil && r.code.FindOptimizations != nil {
		minRequiredLength = r.code.FindOptimizations.MinRequiredLength
	}

	r.initMatch(textInfo)

	r.startTimeoutWatch()
	for {
		if minRequiredLength > 0 {
			if r.code.RightToLeft {
				if r.Runtextpos < minRequiredLength {
					r.tidyMatch(true)
					return nil, nil
				}
			} else if r.Runtextend-r.Runtextpos < minRequiredLength {
				r.tidyMatch(true)
				return nil, nil
			}
		}

		if r.debug {
			//fmt.Printf("\nSearch content: %v\n", string(r.runtext))
			fmt.Printf("\nSearch range: from 0 to %v\n", r.Runtextend)
			fmt.Printf("Firstchar search starting at %v stopping at %v\n", r.Runtextpos, stoppos)
		}

		if findFirstChar(r) {
			if !r.ignoreTimeout {
				if err := r.CheckTimeout(); err != nil {
					return nil, err
				}
			}

			if r.debug {
				fmt.Printf("Executing engine starting at %v\n\n", r.Runtextpos)
			}

			if err := execute(r); err != nil {
				return nil, err
			}

			if r.runmatch.matchcount[0] > 0 {
				// We'll return a match even if it touches a previous empty match
				return r.tidyMatch(quick), nil
			}

			// reset state for another go
			r.Runtrackpos = len(r.runtrack)
			r.Runstackpos = len(r.runstack)
			r.runcrawlpos = len(r.runcrawl)
		}

		// failure!

		if r.Runtextpos == stoppos {
			r.tidyMatch(true)
			return nil, nil
		}

		// Recognize leading []* and various anchors, and bump on failure accordingly

		// r.bump by one and start again

		r.Runtextpos += bump
	}
	// We never get here
}

func executeDefault(r *Runner) error {

	if err := r.goTo(0); err != nil {
		return err
	}

	for {

		if r.debug {
			r.dumpState()
		}

		if !r.ignoreTimeout {
			if err := r.CheckTimeout(); err != nil {
				return err
			}
		}

		switch r.operator {
		case syntax.Stop:
			return nil

		case syntax.Nothing:
			//noop

		case syntax.Goto:
			if err := r.goTo(r.operand(0)); err != nil {
				return err
			}
			continue

		case syntax.Testref:
			if !r.runmatch.isMatched(r.operand(0)) {
				break
			}
			r.advance(1)
			continue

		case syntax.Lazybranch:
			r.trackPush1(r.textPos())
			r.advance(1)
			continue

		case syntax.Lazybranch | syntax.Back:
			r.trackPop()
			r.textto(r.trackPeek())
			if err := r.goTo(r.operand(0)); err != nil {
				return err
			}
			continue

		case syntax.Setmark:
			r.stackPush(r.textPos())
			r.trackPush()
			r.advance(0)
			continue

		case syntax.Nullmark:
			r.stackPush(-1)
			r.trackPush()
			r.advance(0)
			continue

		case syntax.Setmark | syntax.Back, syntax.Nullmark | syntax.Back:
			r.stackPop()

		case syntax.Getmark:
			r.stackPop()
			r.trackPush1(r.stackPeek())
			r.textto(r.stackPeek())
			r.advance(0)
			continue

		case syntax.Getmark | syntax.Back:
			r.trackPop()
			r.stackPush(r.trackPeek())

		case syntax.Capturemark:
			if r.operand(1) != -1 && !r.runmatch.isMatched(r.operand(1)) {
				break
			}
			r.stackPop()
			if r.operand(1) != -1 {
				r.transferCapture(r.operand(0), r.operand(1), r.stackPeek(), r.textPos())
			} else {
				r.Capture(r.operand(0), r.stackPeek(), r.textPos())
			}
			r.trackPush1(r.stackPeek())

			r.advance(2)

			continue

		case syntax.Capturemark | syntax.Back:
			r.trackPop()
			r.stackPush(r.trackPeek())
			r.uncapture()
			if r.operand(0) != -1 && r.operand(1) != -1 {
				r.uncapture()
			}

		case syntax.Branchmark:
			r.stackPop()

			matched := r.textPos() - r.stackPeek()

			if matched != 0 { // Nonempty match -> loop now
				r.trackPush2(r.stackPeek(), r.textPos())     // Save old mark, textpos
				r.stackPush(r.textPos())                     // Make new mark
				if err := r.goTo(r.operand(0)); err != nil { // Loop
					return err
				}
			} else { // Empty match -> straight now
				r.trackPushNeg1(r.stackPeek()) // Save old mark
				r.advance(1)                   // Straight
			}
			continue

		case syntax.Branchmark | syntax.Back:
			r.trackPopN(2)
			r.stackPop()
			r.textto(r.trackPeekN(1))      // Recall position
			r.trackPushNeg1(r.trackPeek()) // Save old mark
			r.advance(1)                   // Straight
			continue

		case syntax.Branchmark | syntax.Back2:
			r.trackPop()
			r.stackPush(r.trackPeek()) // Recall old mark
			// Backtrack

		case syntax.Lazybranchmark:
			{
				// We hit this the first time through a lazy loop and after each
				// successful match of the inner expression.  It simply continues
				// on and doesn't loop.
				r.stackPop()

				oldMarkPos := r.stackPeek()

				if r.textPos() != oldMarkPos { // Nonempty match -> try to loop again by going to 'back' state
					if oldMarkPos != -1 {
						r.trackPush2(oldMarkPos, r.textPos()) // Save old mark, textpos
					} else {
						r.trackPush2(r.textPos(), r.textPos())
					}
				} else {
					// The inner expression found an empty match, so we'll go directly to 'back2' if we
					// backtrack. Don't touch the grouping stack here; instead, record the old mark and
					// a flag indicating that backtracking doesn't need to pop a grouping stack frame.
					r.trackPushNeg2(oldMarkPos, 0)
				}
				r.advance(1)
				continue
			}

		case syntax.Lazybranchmark | syntax.Back:

			// After the first time, Lazybranchmark | syntax.Back occurs
			// with each iteration of the loop, and therefore with every attempted
			// match of the inner expression.  We'll try to match the inner expression,
			// then go back to Lazybranchmark if successful.  If the inner expression
			// fails, we go to Lazybranchmark | syntax.Back2

			r.trackPopN(2)
			pos := r.trackPeekN(1)
			r.trackPushNeg2(r.trackPeek(), 1)            // Save old mark, note that we pushed a new mark
			r.stackPush(pos)                             // Make new mark
			r.textto(pos)                                // Recall position
			if err := r.goTo(r.operand(0)); err != nil { // Loop
				return err
			}
			continue

		case syntax.Lazybranchmark | syntax.Back2:
			// The lazy loop has failed.  We'll do a true backtrack and
			// start over before the lazy loop.
			r.trackPopN(2)
			oldMark := r.trackPeek()
			needsPop := r.trackPeekN(1)
			if needsPop != 0 {
				r.stackPop()
			}
			r.stackPush(oldMark) // Recall old mark

		case syntax.Setcount:
			r.stackPush2(r.textPos(), r.operand(0))
			r.trackPush()
			r.advance(1)
			continue

		case syntax.Nullcount:
			r.stackPush2(-1, r.operand(0))
			r.trackPush()
			r.advance(1)
			continue

		case syntax.Setcount | syntax.Back:
			r.stackPopN(2)

		case syntax.Nullcount | syntax.Back:
			r.stackPopN(2)

		case syntax.Branchcount:
			// r.stackPush:
			//  0: Mark
			//  1: Count

			r.stackPopN(2)
			mark := r.stackPeek()
			count := r.stackPeekN(1)
			matched := r.textPos() - mark

			if count >= r.operand(1) || (matched == 0 && count >= 0) { // Max loops or empty match -> straight now
				r.trackPushNeg2(mark, count) // Save old mark, count
				r.advance(2)                 // Straight
			} else { // Nonempty match -> count+loop now
				r.trackPush1(mark)                           // remember mark
				r.stackPush2(r.textPos(), count+1)           // Make new mark, incr count
				if err := r.goTo(r.operand(0)); err != nil { // Loop
					return err
				}
			}
			continue

		case syntax.Branchcount | syntax.Back:
			// r.trackPush:
			//  0: Previous mark
			// r.stackPush:
			//  0: Mark (= current pos, discarded)
			//  1: Count
			r.trackPop()
			r.stackPopN(2)
			if r.stackPeekN(1) > 0 { // Positive -> can go straight
				r.textto(r.stackPeek())                           // Zap to mark
				r.trackPushNeg2(r.trackPeek(), r.stackPeekN(1)-1) // Save old mark, old count
				r.advance(2)                                      // Straight
				continue
			}
			r.stackPush2(r.trackPeek(), r.stackPeekN(1)-1) // recall old mark, old count

		case syntax.Branchcount | syntax.Back2:
			// r.trackPush:
			//  0: Previous mark
			//  1: Previous count
			r.trackPopN(2)
			r.stackPush2(r.trackPeek(), r.trackPeekN(1)) // Recall old mark, old count

		case syntax.Lazybranchcount:
			// r.stackPush:
			//  0: Mark
			//  1: Count

			r.stackPopN(2)
			mark := r.stackPeek()
			count := r.stackPeekN(1)

			if count < 0 { // Negative count -> loop now
				r.trackPushNeg1(mark)                        // Save old mark
				r.stackPush2(r.textPos(), count+1)           // Make new mark, incr count
				if err := r.goTo(r.operand(0)); err != nil { // Loop
					return err
				}
			} else { // Nonneg count -> straight now
				r.trackPush3(mark, count, r.textPos()) // Save mark, count, position
				r.advance(2)                           // Straight
			}
			continue

		case syntax.Lazybranchcount | syntax.Back:
			// r.trackPush:
			//  0: Mark
			//  1: Count
			//  2: r.textPos

			r.trackPopN(3)
			mark := r.trackPeek()
			textpos := r.trackPeekN(2)

			if r.trackPeekN(1) < r.operand(1) && textpos != mark { // Under limit and not empty match -> loop
				r.textto(textpos)                            // Recall position
				r.stackPush2(textpos, r.trackPeekN(1)+1)     // Make new mark, incr count
				r.trackPushNeg1(mark)                        // Save old mark
				if err := r.goTo(r.operand(0)); err != nil { // Loop
					return err
				}
				continue
			} else { // Max loops or empty match -> backtrack
				r.stackPush2(r.trackPeek(), r.trackPeekN(1)) // Recall old mark, count
				// backtrack
			}

		case syntax.Lazybranchcount | syntax.Back2:
			// r.trackPush:
			//  0: Previous mark
			// r.stackPush:
			//  0: Mark (== current pos, discarded)
			//  1: Count
			r.trackPop()
			r.stackPopN(2)
			r.stackPush2(r.trackPeek(), r.stackPeekN(1)-1) // Recall old mark, count
			// Backtrack

		case syntax.Setjump:
			r.stackPush2(r.trackpos(), r.Crawlpos())
			r.trackPush()
			r.advance(0)
			continue

		case syntax.Setjump | syntax.Back:
			r.stackPopN(2)

		case syntax.Backjump:
			// r.stackPush:
			//  0: Saved trackpos
			//  1: r.crawlpos
			r.stackPopN(2)
			r.trackto(r.stackPeek())

			for r.Crawlpos() != r.stackPeekN(1) {
				r.uncapture()
			}

		case syntax.Forejump:
			// r.stackPush:
			//  0: Saved trackpos
			//  1: r.crawlpos
			r.stackPopN(2)
			r.trackto(r.stackPeek())
			r.trackPush1(r.stackPeekN(1))
			r.advance(0)
			continue

		case syntax.Forejump | syntax.Back:
			// r.trackPush:
			//  0: r.crawlpos
			r.trackPop()

			for r.Crawlpos() != r.trackPeek() {
				r.uncapture()
			}

		case syntax.Bol:
			if r.leftchars() > 0 && r.charAt(r.textPos()-1) != '\n' {
				break
			}
			r.advance(0)
			continue

		case syntax.Eol:
			if r.rightchars() > 0 && r.charAt(r.textPos()) != '\n' {
				break
			}
			r.advance(0)
			continue

		case syntax.Boundary:
			if !r.IsBoundary(r.textPos()) {
				break
			}
			r.advance(0)
			continue

		case syntax.Nonboundary:
			if r.IsBoundary(r.textPos()) {
				break
			}
			r.advance(0)
			continue

		case syntax.ECMABoundary:
			if !r.IsECMABoundary(r.textPos()) {
				break
			}
			r.advance(0)
			continue

		case syntax.NonECMABoundary:
			if r.IsECMABoundary(r.textPos()) {
				break
			}
			r.advance(0)
			continue

		case syntax.Beginning:
			if r.leftchars() > 0 {
				break
			}
			r.advance(0)
			continue

		case syntax.Start:
			if r.textPos() != r.textstart() {
				break
			}
			r.advance(0)
			continue

		case syntax.EndZ:
			rchars := r.rightchars()
			if rchars > 1 {
				break
			}
			// RE2 and EcmaScript define $ as "asserts position at the end of the string"
			// PCRE/.NET adds "or before the line terminator right at the end of the string (if any)"
			if (r.re.options & (RE2 | ECMAScript)) != 0 {
				// RE2/Ecmascript mode
				if rchars > 0 {
					break
				}
			} else if rchars == 1 && r.charAt(r.textPos()) != '\n' {
				// "regular" mode
				break
			}

			r.advance(0)
			continue

		case syntax.End:
			if r.rightchars() > 0 {
				break
			}
			r.advance(0)
			continue

		case syntax.One:
			if r.forwardchars() < 1 || r.forwardcharnext() != rune(r.operand(0)) {
				break
			}

			r.advance(1)
			continue

		case syntax.Notone:
			if r.forwardchars() < 1 || r.forwardcharnext() == rune(r.operand(0)) {
				break
			}

			r.advance(1)
			continue

		case syntax.Set:

			if r.forwardchars() < 1 || !r.code.Sets[r.operand(0)].CharIn(r.forwardcharnext()) {
				break
			}

			r.advance(1)
			continue

		case syntax.Multi:
			if !r.runematch(r.code.Strings[r.operand(0)]) {
				break
			}

			r.advance(1)
			continue

		case syntax.Ref:

			capnum := r.operand(0)

			if r.runmatch.isMatched(capnum) {
				if !r.refmatch(r.runmatch.matchIndex(capnum), r.runmatch.matchLength(capnum)) {
					break
				}
			} else {
				if (r.re.options & ECMAScript) == 0 {
					break
				}
			}

			r.advance(1)
			continue

		case syntax.Onerep:

			c := r.operand(1)

			if r.forwardchars() < c {
				break
			}

			ch := rune(r.operand(0))

			for c > 0 {
				if r.forwardcharnext() != ch {
					goto BreakBackward
				}
				c--
			}

			r.advance(2)
			continue

		case syntax.Notonerep:

			c := r.operand(1)

			if r.forwardchars() < c {
				break
			}
			ch := rune(r.operand(0))

			for c > 0 {
				if r.forwardcharnext() == ch {
					goto BreakBackward
				}
				c--
			}

			r.advance(2)
			continue

		case syntax.Setrep:

			c := r.operand(1)

			if r.forwardchars() < c {
				break
			}

			set := r.code.Sets[r.operand(0)]

			for c > 0 {
				if !set.CharIn(r.forwardcharnext()) {
					goto BreakBackward
				}
				c--
			}

			r.advance(2)
			continue

		case syntax.Oneloop, syntax.Oneloopatomic:

			c := r.operand(1)

			if c > r.forwardchars() {
				c = r.forwardchars()
			}

			ch := rune(r.operand(0))
			i := c

			for ; i > 0; i-- {
				if r.forwardcharnext() != ch {
					r.backwardnext()
					break
				}
			}

			if c > i && r.operator == syntax.Oneloop {
				r.trackPush2(c-i-1, r.textPos()-r.bump())
			}

			r.advance(2)
			continue

		case syntax.Notoneloop, syntax.Notoneloopatomic:

			c := r.operand(1)

			if c > r.forwardchars() {
				c = r.forwardchars()
			}

			ch := rune(r.operand(0))
			i := c

			for ; i > 0; i-- {
				if r.forwardcharnext() == ch {
					r.backwardnext()
					break
				}
			}

			if c > i && r.operator == syntax.Notoneloop {
				r.trackPush2(c-i-1, r.textPos()-r.bump())
			}

			r.advance(2)
			continue

		case syntax.Setloop, syntax.Setloopatomic:

			c := r.operand(1)

			if c > r.forwardchars() {
				c = r.forwardchars()
			}

			set := r.code.Sets[r.operand(0)]
			i := c

			for ; i > 0; i-- {
				if !set.CharIn(r.forwardcharnext()) {
					r.backwardnext()
					break
				}
			}

			if c > i && r.operator == syntax.Setloop {
				r.trackPush2(c-i-1, r.textPos()-r.bump())
			}

			r.advance(2)
			continue

		case syntax.Oneloop | syntax.Back, syntax.Notoneloop | syntax.Back:

			r.trackPopN(2)
			i := r.trackPeek()
			pos := r.trackPeekN(1)

			r.textto(pos)

			if i > 0 {
				r.trackPush2(i-1, pos-r.bump())
			}

			r.advance(2)
			continue

		case syntax.Setloop | syntax.Back:

			r.trackPopN(2)
			i := r.trackPeek()
			pos := r.trackPeekN(1)

			r.textto(pos)

			if i > 0 {
				r.trackPush2(i-1, pos-r.bump())
			}

			r.advance(2)
			continue

		case syntax.Onelazy, syntax.Notonelazy:

			c := r.operand(1)

			if c > r.forwardchars() {
				c = r.forwardchars()
			}

			if c > 0 {
				r.trackPush2(c-1, r.textPos())
			}

			r.advance(2)
			continue

		case syntax.Setlazy:

			c := r.operand(1)

			if c > r.forwardchars() {
				c = r.forwardchars()
			}

			if c > 0 {
				r.trackPush2(c-1, r.textPos())
			}

			r.advance(2)
			continue

		case syntax.Onelazy | syntax.Back:

			r.trackPopN(2)
			pos := r.trackPeekN(1)
			r.textto(pos)

			if r.forwardcharnext() != rune(r.operand(0)) {
				break
			}

			i := r.trackPeek()

			if i > 0 {
				r.trackPush2(i-1, pos+r.bump())
			}

			r.advance(2)
			continue

		case syntax.Notonelazy | syntax.Back:

			r.trackPopN(2)
			pos := r.trackPeekN(1)
			r.textto(pos)

			if r.forwardcharnext() == rune(r.operand(0)) {
				break
			}

			i := r.trackPeek()

			if i > 0 {
				r.trackPush2(i-1, pos+r.bump())
			}

			r.advance(2)
			continue

		case syntax.Setlazy | syntax.Back:

			r.trackPopN(2)
			pos := r.trackPeekN(1)
			r.textto(pos)

			if !r.code.Sets[r.operand(0)].CharIn(r.forwardcharnext()) {
				break
			}

			i := r.trackPeek()

			if i > 0 {
				r.trackPush2(i-1, pos+r.bump())
			}

			r.advance(2)
			continue

		case syntax.UpdateBumpalong:
			// UpdateBumpalong should only exist in the code stream at such a point where the root
			// of the backtracking stack contains the runtextpos from the start of this Go call. Replace
			// that tracking value with the current runtextpos value if it's greater.
			trackingpos := r.runtrack[len(r.runtrack)-1]
			if trackingpos < r.Runtextpos {
				r.runtrack[len(r.runtrack)-1] = r.Runtextpos
			}
			r.advance(0)
			continue

		default:
			return fmt.Errorf("unknown state in regex runner: %v", r.operator)
		}

	BreakBackward:
		;

		// "break Backward" comes here:
		if err := r.backtrack(); err != nil {
			return err
		}
	}
}

// increase the size of stack and track storage
func (r *Runner) ensureStorage() error {
	if r.Runstackpos < r.runtrackcount*4 {
		doubleIntSlice(&r.runstack, &r.Runstackpos)
	}
	if r.Runtrackpos < r.runtrackcount*4 && !r.growTrack() {
		return ErrBacktrackingStackLimit
	}
	return nil
}

func (r *Runner) ensureStack(plus int) {
	if r.Runstackpos-plus < r.runtrackcount*4 {
		doubleIntSlice(&r.runstack, &r.Runstackpos)
	}
}

func doubleIntSlice(s *[]int, pos *int) {
	oldLen := len(*s)
	newS := make([]int, oldLen*2)

	copy(newS[oldLen:], *s)
	*pos += oldLen
	*s = newS
}

// Save a number on the longjump unrolling stack
func (r *Runner) crawl(i int) {
	if r.runcrawlpos == 0 {
		doubleIntSlice(&r.runcrawl, &r.runcrawlpos)
	}
	r.runcrawlpos--
	r.runcrawl[r.runcrawlpos] = i
}

// Remove a number from the longjump unrolling stack
func (r *Runner) popcrawl() int {
	val := r.runcrawl[r.runcrawlpos]
	r.runcrawlpos++
	return val
}

// Get the height of the stack
func (r *Runner) Crawlpos() int {
	return len(r.runcrawl) - r.runcrawlpos
}

func (r *Runner) advance(i int) {
	r.codepos += (i + 1)
	r.setOperator(r.code.Codes[r.codepos])
}

func (r *Runner) goTo(newpos int) error {
	// when branching backward or in place, ensure storage
	if newpos <= r.codepos {
		if err := r.ensureStorage(); err != nil {
			return err
		}
	}

	r.setOperator(r.code.Codes[newpos])
	r.codepos = newpos
	return nil
}

func (r *Runner) textto(newpos int) {
	r.Runtextpos = newpos
}

func (r *Runner) trackto(newpos int) {
	r.Runtrackpos = len(r.runtrack) - newpos
}

func (r *Runner) textstart() int {
	return r.Runtextstart
}

func (r *Runner) textPos() int {
	return r.Runtextpos
}

// push onto the backtracking stack
func (r *Runner) trackpos() int {
	return len(r.runtrack) - r.Runtrackpos
}

func (r *Runner) growTrack() bool {
	oldLen := len(r.runtrack)
	newLen := oldLen * 2
	if newLen == 0 {
		newLen = 1
	}
	if limit := r.re.optimizations.MaxBacktrackingStackSize; limit >= 0 && newLen > limit {
		newLen = limit
	}
	if newLen <= oldLen {
		return false
	}

	newTrack := make([]int, newLen)
	copy(newTrack[newLen-oldLen:], r.runtrack)
	r.Runtrackpos += newLen - oldLen
	r.runtrack = newTrack
	return true
}

func (r *Runner) trackPush() {
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = r.codepos
}

func (r *Runner) trackPush1(I1 int) {
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = I1
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = r.codepos
}

func (r *Runner) trackPush2(I1, I2 int) {
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = I1
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = I2
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = r.codepos
}

func (r *Runner) trackPush3(I1, I2, I3 int) {
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = I1
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = I2
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = I3
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = r.codepos
}

func (r *Runner) trackPushNeg1(I1 int) {
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = I1
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = -r.codepos
}

func (r *Runner) trackPushNeg2(I1, I2 int) {
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = I1
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = I2
	r.Runtrackpos--
	r.runtrack[r.Runtrackpos] = -r.codepos
}

func (r *Runner) backtrack() error {
	newpos := r.runtrack[r.Runtrackpos]
	r.Runtrackpos++

	if r.debug {
		if newpos < 0 {
			fmt.Printf("       Backtracking (back2) to code position %v\n", -newpos)
		} else {
			fmt.Printf("       Backtracking to code position %v\n", newpos)
		}
	}

	if newpos < 0 {
		newpos = -newpos
		r.setOperator(r.code.Codes[newpos] | int(syntax.Back2))
	} else {
		r.setOperator(r.code.Codes[newpos] | int(syntax.Back))
	}

	// When branching backward, ensure storage
	if newpos < r.codepos {
		if err := r.ensureStorage(); err != nil {
			return err
		}
	}

	r.codepos = newpos
	return nil
}

func (r *Runner) setOperator(op int) {
	r.caseInsensitive = (0 != (op & int(syntax.Ci)))
	r.rightToLeft = (0 != (op & int(syntax.Rtl)))
	r.operator = syntax.InstOp(op & ^int(syntax.Rtl|syntax.Ci))
}

func (r *Runner) trackPop() {
	r.Runtrackpos++
}

// pop framesize items from the backtracking stack
func (r *Runner) trackPopN(framesize int) {
	r.Runtrackpos += framesize
}

// Technically we are actually peeking at items already popped.  So if you want to
// get and pop the top item from the stack, you do
// r.trackPop();
// r.trackPeek();
func (r *Runner) trackPeek() int {
	return r.runtrack[r.Runtrackpos-1]
}

// get the ith element down on the backtracking stack
func (r *Runner) trackPeekN(i int) int {
	return r.runtrack[r.Runtrackpos-i-1]
}

// Push onto the grouping stack
func (r *Runner) stackPush(I1 int) {
	r.Runstackpos--
	r.runstack[r.Runstackpos] = I1
}

func (r *Runner) stackPush2(I1, I2 int) {
	r.Runstackpos--
	r.runstack[r.Runstackpos] = I1
	r.Runstackpos--
	r.runstack[r.Runstackpos] = I2
}

func (r *Runner) stackPop() {
	r.Runstackpos++
}

// pop framesize items from the grouping stack
func (r *Runner) stackPopN(framesize int) {
	r.Runstackpos += framesize
}

// Technically we are actually peeking at items already popped.  So if you want to
// get and pop the top item from the stack, you do
// r.stackPop();
// r.stackPeek();
func (r *Runner) stackPeek() int {
	return r.runstack[r.Runstackpos-1]
}

// get the ith element down on the grouping stack
func (r *Runner) stackPeekN(i int) int {
	return r.runstack[r.Runstackpos-i-1]
}

func (r *Runner) operand(i int) int {
	return r.code.Codes[r.codepos+i+1]
}

func (r *Runner) leftchars() int {
	return r.Runtextpos
}

func (r *Runner) rightchars() int {
	return r.Runtextend - r.Runtextpos
}

func (r *Runner) bump() int {
	if r.rightToLeft {
		return -1
	}
	return 1
}

func (r *Runner) forwardchars() int {
	if r.rightToLeft {
		return r.Runtextpos
	}
	return r.Runtextend - r.Runtextpos
}

func (r *Runner) forwardcharnext() rune {
	var ch rune
	if r.rightToLeft {
		r.Runtextpos--
		ch = r.Runtext[r.Runtextpos]
	} else {
		ch = r.Runtext[r.Runtextpos]
		r.Runtextpos++
	}

	// move this to compile time for individual runes
	/*if r.caseInsensitive {
		return unicode.ToLower(ch)
	}*/
	return ch
}

func (r *Runner) runematch(str []rune) bool {
	var pos int

	c := len(str)
	if !r.rightToLeft {
		if r.Runtextend-r.Runtextpos < c {
			return false
		}

		pos = r.Runtextpos + c
	} else {
		if r.Runtextpos-0 < c {
			return false
		}

		pos = r.Runtextpos
	}

	if !r.caseInsensitive {
		for c != 0 {
			c--
			pos--
			if str[c] != r.Runtext[pos] {
				return false
			}
		}
	} else {
		for c != 0 {
			c--
			pos--
			if str[c] != unicode.ToLower(r.Runtext[pos]) {
				return false
			}
		}
	}

	if !r.rightToLeft {
		pos += len(str)
	}

	r.Runtextpos = pos

	return true
}

func (r *Runner) refmatch(index, len int) bool {
	var c, pos, cmpos int

	if !r.rightToLeft {
		if r.Runtextend-r.Runtextpos < len {
			return false
		}

		pos = r.Runtextpos + len
	} else {
		if r.Runtextpos-0 < len {
			return false
		}

		pos = r.Runtextpos
	}
	cmpos = index + len

	c = len

	if !r.caseInsensitive {
		for c != 0 {
			c--
			cmpos--
			pos--
			if r.Runtext[cmpos] != r.Runtext[pos] {
				return false
			}

		}
	} else {
		for c != 0 {
			c--
			cmpos--
			pos--

			if unicode.ToLower(r.Runtext[cmpos]) != unicode.ToLower(r.Runtext[pos]) {
				return false
			}
		}
	}

	if !r.rightToLeft {
		pos += len
	}

	r.Runtextpos = pos

	return true
}

func (r *Runner) backwardnext() {
	if r.rightToLeft {
		r.Runtextpos++
	} else {
		r.Runtextpos--
	}
}

func (r *Runner) charAt(j int) rune {
	return r.Runtext[j]
}

func findFirstCharDefault(r *Runner) bool {
	if 0 != (r.code.Anchors & (syntax.AnchorBeginning | syntax.AnchorStart | syntax.AnchorEndZ | syntax.AnchorEnd)) {
		if !r.code.RightToLeft {
			if (0 != (r.code.Anchors&syntax.AnchorBeginning) && r.Runtextpos > 0) ||
				(0 != (r.code.Anchors&syntax.AnchorStart) && r.Runtextpos > r.Runtextstart) {
				r.Runtextpos = r.Runtextend
				return false
			}
			if 0 != (r.code.Anchors&syntax.AnchorEndZ) && r.Runtextpos < r.Runtextend-1 {
				r.Runtextpos = r.Runtextend - 1
			} else if 0 != (r.code.Anchors&syntax.AnchorEnd) && r.Runtextpos < r.Runtextend {
				r.Runtextpos = r.Runtextend
			}
		} else {
			if (0 != (r.code.Anchors&syntax.AnchorEnd) && r.Runtextpos < r.Runtextend) ||
				(0 != (r.code.Anchors&syntax.AnchorEndZ) && (r.Runtextpos < r.Runtextend-1 ||
					(r.Runtextpos == r.Runtextend-1 && r.charAt(r.Runtextpos) != '\n'))) ||
				(0 != (r.code.Anchors&syntax.AnchorStart) && r.Runtextpos < r.Runtextstart) {
				r.Runtextpos = 0
				return false
			}
			if 0 != (r.code.Anchors&syntax.AnchorBeginning) && r.Runtextpos > 0 {
				r.Runtextpos = 0
			}
		}

		if r.code.BmPrefix != nil {
			return r.code.BmPrefix.IsMatch(r.Runtext, r.Runtextpos, 0, r.Runtextend)
		}

		return true // found a valid start or end anchor
	} else if r.code.BmPrefix != nil {
		r.Runtextpos = r.code.BmPrefix.Scan(r.Runtext, r.Runtextpos, 0, r.Runtextend)

		if r.Runtextpos == -1 {
			if r.code.RightToLeft {
				r.Runtextpos = 0
			} else {
				r.Runtextpos = r.Runtextend
			}
			return false
		}

		return true
	}

	if shouldUseFindFirstCharOptimized(r) {
		if handled, found := findFirstCharOptimized(r); handled {
			return found
		}
	}

	if r.code.FcPrefix == nil {
		return true
	}

	r.rightToLeft = r.code.RightToLeft
	r.caseInsensitive = r.code.FcPrefix.CaseInsensitive

	set := r.code.FcPrefix.PrefixSet
	if set.IsSingleton() {
		ch := set.SingletonChar()
		for i := r.forwardchars(); i > 0; i-- {
			if ch == r.forwardcharnext() {
				r.backwardnext()
				return true
			}
		}
	} else {
		for i := r.forwardchars(); i > 0; i-- {
			n := r.forwardcharnext()
			//fmt.Printf("%v in %v: %v\n", string(n), set.String(), set.CharIn(n))
			if set.CharIn(n) {
				r.backwardnext()
				return true
			}
		}
	}

	return false
}

func shouldUseFindFirstCharOptimized(r *Runner) bool {
	if r.code == nil || r.code.FindOptimizations == nil {
		return false
	}

	opts := r.code.FindOptimizations
	switch opts.FindMode {
	case syntax.TrailingAnchor_FixedLength_LeftToRight_End,
		syntax.LeadingString_OrdinalIgnoreCase_LeftToRight,
		syntax.LeadingStrings_LeftToRight,
		syntax.LeadingStrings_OrdinalIgnoreCase_LeftToRight,
		syntax.FixedDistanceChar_LeftToRight,
		syntax.FixedDistanceString_LeftToRight,
		syntax.FixedDistanceSets_LeftToRight,
		syntax.LiteralAfterLoop_LeftToRight,
		syntax.RequiredLandmarkChain_LeftToRight:
		return true
	case syntax.LeadingSet_LeftToRight:
		// General Unicode sets already have a direct fallback loop below.
		// Large enumerated sets are also faster through the set's ASCII bitmap
		// than through the linear IndexOfAny helper.
		return len(opts.FixedDistanceSets) > 0 &&
			((len(opts.FixedDistanceSets[0].Chars) > 0 && len(opts.FixedDistanceSets[0].Chars) <= 5) ||
				opts.FixedDistanceSets[0].Range != nil)
	default:
		return false
	}
}

func findFirstCharOptimized(r *Runner) (handled bool, found bool) {
	if r.code == nil || r.code.FindOptimizations == nil {
		return false, false
	}

	opts := r.code.FindOptimizations
	switch opts.FindMode {
	case syntax.NoSearch:
		return false, false
	case syntax.TrailingAnchor_FixedLength_LeftToRight_End:
		return true, findTrailingFixedLengthEnd(r, opts.MinRequiredLength)
	case syntax.LeadingString_LeftToRight:
		return true, findLeadingStringLeftToRight(r, []rune(opts.LeadingPrefix), false)
	case syntax.LeadingString_OrdinalIgnoreCase_LeftToRight:
		return true, findLeadingStringLeftToRight(r, []rune(opts.LeadingPrefix), true)
	case syntax.LeadingStrings_LeftToRight:
		return true, findLeadingStringsLeftToRight(r, opts.LeadingPrefixesRunes, opts.LeadingPrefixFirstRunes, false)
	case syntax.LeadingStrings_OrdinalIgnoreCase_LeftToRight:
		return true, findLeadingStringsLeftToRight(r, opts.LeadingPrefixesRunes, opts.LeadingPrefixFirstRunes, true)
	case syntax.LeadingSet_LeftToRight, syntax.FixedDistanceSets_LeftToRight:
		return true, findFixedDistanceSetsLeftToRight(r, opts.FixedDistanceSets)
	case syntax.FixedDistanceChar_LeftToRight:
		return true, findFixedDistanceCharLeftToRight(r, opts.FixedDistanceLiteral.C, opts.FixedDistanceLiteral.Distance)
	case syntax.FixedDistanceString_LeftToRight:
		return true, findFixedDistanceStringLeftToRight(r, []rune(opts.FixedDistanceLiteral.S), opts.FixedDistanceLiteral.Distance)
	case syntax.LiteralAfterLoop_LeftToRight:
		return true, findLiteralAfterLoopLeftToRight(r, opts.LiteralAfterLoop)
	case syntax.RequiredLandmarkChain_LeftToRight:
		return true, findRequiredLandmarkChainLeftToRight(r, opts.LandmarkChain)
	default:
		return false, false
	}
}

func findTrailingFixedLengthEnd(r *Runner, fixedLength int) bool {
	start := r.Runtextend - fixedLength
	if start < r.Runtextpos || start < 0 {
		r.Runtextpos = r.Runtextend
		return false
	}
	r.Runtextpos = start
	return true
}

func findLeadingStringLeftToRight(r *Runner, prefix []rune, ignoreCase bool) bool {
	if len(prefix) == 0 {
		return true
	}

	search := r.Runtext[r.Runtextpos:]
	var offset int
	if ignoreCase {
		if isASCIIRunes(prefix) {
			offset = helpers.IndexOfIgnoreCaseAscii(search, prefix)
		} else {
			offset = helpers.IndexOfIgnoreCase(search, prefix)
		}
	} else {
		offset = helpers.IndexOf(search, prefix)
	}
	if offset < 0 {
		r.Runtextpos = r.Runtextend
		return false
	}

	start := r.Runtextpos + offset
	if !hasRequiredLengthAt(r, start) {
		r.Runtextpos = r.Runtextend
		return false
	}
	r.Runtextpos = start
	return true
}

func findLeadingStringsLeftToRight(r *Runner, prefixes [][]rune, firstRunes []rune, ignoreCase bool) bool {
	if len(prefixes) == 0 {
		return false
	}

	// Unicode ordinal-ignore-case matching has more possible first-rune folds
	// than a small precomputed set can safely represent. Keep its conservative
	// position-by-position scan; the common case-sensitive path skips directly
	// between possible first runes.
	if ignoreCase || len(firstRunes) == 0 {
		for start := r.Runtextpos; start <= latestPossibleStart(r); start++ {
			for _, prefix := range prefixes {
				if ignoreCase {
					if helpers.StartsWithIgnoreCase(r.Runtext[start:], prefix) {
						r.Runtextpos = start
						return true
					}
				} else if helpers.StartsWith(r.Runtext[start:], prefix) {
					r.Runtextpos = start
					return true
				}
			}
		}
		r.Runtextpos = r.Runtextend
		return false
	}

	latest := min(latestPossibleStart(r), r.Runtextend-1)
	for searchAt := r.Runtextpos; searchAt <= latest; {
		offset := indexOfAnyRunes(r.Runtext[searchAt:latest+1], firstRunes)
		if offset < 0 {
			break
		}
		start := searchAt + offset
		first := r.Runtext[start]
		for _, prefix := range prefixes {
			if len(prefix) > 0 && prefix[0] == first && helpers.StartsWith(r.Runtext[start:], prefix) {
				r.Runtextpos = start
				return true
			}
		}
		searchAt = start + 1
	}

	r.Runtextpos = r.Runtextend
	return false
}

func indexOfAnyRunes(input, find []rune) int {
	switch len(find) {
	case 0:
		return -1
	case 1:
		return helpers.IndexOfAny1(input, find[0])
	case 2:
		return helpers.IndexOfAny2(input, find[0], find[1])
	case 3:
		return helpers.IndexOfAny3(input, find[0], find[1], find[2])
	default:
		return helpers.IndexOfAny(input, find)
	}
}

func findFixedDistanceCharLeftToRight(r *Runner, ch rune, distance int) bool {
	searchStart := r.Runtextpos + distance
	for searchStart < r.Runtextend {
		offset := helpers.IndexOfAny1(r.Runtext[searchStart:], ch)
		if offset < 0 {
			r.Runtextpos = r.Runtextend
			return false
		}
		literalIndex := searchStart + offset
		start := literalIndex - distance
		if start >= r.Runtextpos && hasRequiredLengthAt(r, start) {
			r.Runtextpos = start
			return true
		}
		if start > latestPossibleStart(r) {
			break
		}
		searchStart = literalIndex + 1
	}

	r.Runtextpos = r.Runtextend
	return false
}

func findFixedDistanceStringLeftToRight(r *Runner, literal []rune, distance int) bool {
	if len(literal) == 0 {
		return true
	}

	searchStart := r.Runtextpos + distance
	for searchStart <= r.Runtextend-len(literal) {
		offset := helpers.IndexOf(r.Runtext[searchStart:], literal)
		if offset < 0 {
			r.Runtextpos = r.Runtextend
			return false
		}
		literalIndex := searchStart + offset
		start := literalIndex - distance
		if start >= r.Runtextpos && hasRequiredLengthAt(r, start) {
			r.Runtextpos = start
			return true
		}
		if start > latestPossibleStart(r) {
			break
		}
		searchStart = literalIndex + 1
	}

	r.Runtextpos = r.Runtextend
	return false
}

func findFixedDistanceSetsLeftToRight(r *Runner, sets []syntax.FixedDistanceSet) bool {
	if len(sets) == 0 || sets[0].Set == nil {
		return false
	}

	primary := sets[0]
	searchStart := r.Runtextpos + primary.Distance
	for searchStart < r.Runtextend {
		offset := indexOfSet(r.Runtext[searchStart:], primary)
		if offset < 0 {
			r.Runtextpos = r.Runtextend
			return false
		}

		charIndex := searchStart + offset
		start := charIndex - primary.Distance
		if start > latestPossibleStart(r) {
			break
		}
		if start >= r.Runtextpos && hasRequiredLengthAt(r, start) && fixedDistanceSetsMatchAt(r, sets, start) {
			r.Runtextpos = start
			return true
		}
		searchStart = charIndex + 1
	}

	r.Runtextpos = r.Runtextend
	return false
}

func findLiteralAfterLoopLeftToRight(r *Runner, literal *syntax.LiteralAfterLoop) bool {
	if literal == nil || literal.LoopNode == nil || literal.LoopNode.Set == nil {
		return false
	}

	searchStart := r.Runtextpos
	for searchStart < r.Runtextend {
		literalIndex := indexOfLiteralAfterLoop(r, literal, searchStart)
		if literalIndex < 0 {
			r.Runtextpos = r.Runtextend
			return false
		}

		start := literalIndex
		for start > r.Runtextpos && literal.LoopNode.Set.CharIn(r.Runtext[start-1]) {
			start--
		}
		if hasRequiredLengthAt(r, start) {
			r.Runtextpos = start
			return true
		}
		searchStart = literalIndex + 1
	}

	r.Runtextpos = r.Runtextend
	return false
}

func findRequiredLandmarkChainLeftToRight(r *Runner, chain *syntax.RequiredLandmarkChain) bool {
	if chain == nil || chain.LeadingLoopSet == nil || len(chain.Landmarks) == 0 {
		return false
	}

	for searchStart := r.Runtextpos; searchStart <= latestPossibleStart(r); {
		first, ok := findNextRequiredLandmarkRunes(r.Runtext, searchStart, r.Runtextend, chain.Landmarks[0])
		if !ok {
			r.Runtextpos = r.Runtextend
			return false
		}

		nextStart := first.End
		for i := 1; i < len(chain.Landmarks); i++ {
			landmark, ok := findNextRequiredLandmarkRunes(r.Runtext, nextStart, r.Runtextend, chain.Landmarks[i])
			if !ok {
				r.Runtextpos = r.Runtextend
				return false
			}
			nextStart = landmark.End
		}

		candidate := first.Start
		if candidate < r.Runtextpos {
			candidate = r.Runtextpos
		}
		for candidate > r.Runtextpos && chain.LeadingLoopSet.CharIn(r.Runtext[candidate-1]) {
			candidate--
		}
		if hasRequiredLengthAt(r, candidate) {
			r.Runtextpos = candidate
			return true
		}

		searchStart = first.CoreStart + 1
	}

	r.Runtextpos = r.Runtextend
	return false
}

type requiredLandmarkMatch struct {
	Start     int
	CoreStart int
	End       int
}

func findNextRequiredLandmarkRunes(input []rune, startAt, endAt int, landmark syntax.RequiredLandmark) (requiredLandmarkMatch, bool) {
	for i := startAt; i < endAt; i++ {
		for _, alt := range landmark.Alternatives {
			if match, ok := requiredLandmarkAlternativeMatch(input, i, endAt, alt); ok {
				return match, true
			}
		}
	}
	return requiredLandmarkMatch{}, false
}

func requiredLandmarkAlternativeMatch(input []rune, start, endAt int, alt syntax.RequiredLandmarkAlternative) (requiredLandmarkMatch, bool) {
	if alt.RequireWhitespaceBefore &&
		(start == 0 || alt.LeadingWhitespaceSet == nil || !alt.LeadingWhitespaceSet.CharIn(input[start-1])) {
		return requiredLandmarkMatch{}, false
	}

	var end int
	if len(alt.Literal) > 0 {
		if start+len(alt.Literal) > endAt || !helpers.StartsWith(input[start:], alt.Literal) {
			return requiredLandmarkMatch{}, false
		}
		end = start + len(alt.Literal)
	} else if alt.Set != nil && alt.MinRepeat > 0 {
		end = start
		maxRepeat := alt.MaxRepeat
		if maxRepeat <= 0 {
			maxRepeat = alt.MinRepeat
		}
		for end < endAt && end-start < maxRepeat && alt.Set.CharIn(input[end]) {
			end++
		}
		if end-start < alt.MinRepeat {
			return requiredLandmarkMatch{}, false
		}
	} else {
		return requiredLandmarkMatch{}, false
	}

	if alt.RequireWhitespaceAfter &&
		(end >= endAt || alt.TrailingWhitespaceSet == nil || !alt.TrailingWhitespaceSet.CharIn(input[end])) {
		return requiredLandmarkMatch{}, false
	}

	matchStart := start
	for matchStart > 0 && alt.LeadingWhitespaceSet != nil && alt.LeadingWhitespaceSet.CharIn(input[matchStart-1]) {
		matchStart--
	}
	return requiredLandmarkMatch{Start: matchStart, CoreStart: start, End: end}, true
}

func indexOfLiteralAfterLoop(r *Runner, literal *syntax.LiteralAfterLoop, searchStart int) int {
	switch {
	case literal.String != "":
		needle := []rune(literal.String)
		if literal.StringIgnoreCase {
			var offset int
			if isASCIIString(literal.String) {
				offset = helpers.IndexOfIgnoreCaseAscii(r.Runtext[searchStart:], needle)
			} else {
				offset = helpers.IndexOfIgnoreCase(r.Runtext[searchStart:], needle)
			}
			if offset >= 0 {
				return searchStart + offset
			}
		} else if offset := helpers.IndexOf(r.Runtext[searchStart:], needle); offset >= 0 {
			return searchStart + offset
		}
	case len(literal.Chars) > 0:
		if offset := helpers.IndexOfAny(r.Runtext[searchStart:], literal.Chars); offset >= 0 {
			return searchStart + offset
		}
	default:
		if offset := helpers.IndexOfAny1(r.Runtext[searchStart:], literal.Char); offset >= 0 {
			return searchStart + offset
		}
	}
	return -1
}

func isASCIIRunes(in []rune) bool {
	for _, ch := range in {
		if ch > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func indexOfSet(chars []rune, set syntax.FixedDistanceSet) int {
	if len(set.Chars) > 0 && !set.Negated {
		return helpers.IndexOfAny(chars, set.Chars)
	}
	if len(set.Chars) > 0 && set.Negated {
		return helpers.IndexOfAnyExcept(chars, set.Chars)
	}
	if set.Range != nil {
		if set.Negated {
			return helpers.IndexOfAnyExceptInRange(chars, set.Range.First, set.Range.Last)
		}
		return helpers.IndexOfAnyInRange(chars, set.Range.First, set.Range.Last)
	}
	return helpers.IndexFunc(chars, func(ch rune) bool {
		return charInFixedDistanceSet(set, ch)
	})
}

func fixedDistanceSetsMatchAt(r *Runner, sets []syntax.FixedDistanceSet, start int) bool {
	for _, set := range sets {
		index := start + set.Distance
		if index < 0 || index >= r.Runtextend || !charInFixedDistanceSet(set, r.Runtext[index]) {
			return false
		}
	}
	return true
}

func charInFixedDistanceSet(set syntax.FixedDistanceSet, ch rune) bool {
	if len(set.Chars) > 0 {
		found := slices.Contains(set.Chars, ch)
		if set.Negated {
			return !found
		}
		return found
	}
	if set.Range != nil {
		found := ch >= set.Range.First && ch <= set.Range.Last
		if set.Negated {
			return !found
		}
		return found
	}
	return set.Set != nil && set.Set.CharIn(ch)
}

func latestPossibleStart(r *Runner) int {
	if r.code == nil || r.code.FindOptimizations == nil {
		return r.Runtextend
	}
	minRequiredLength := r.code.FindOptimizations.MinRequiredLength
	if minRequiredLength <= 0 {
		return r.Runtextend
	}
	return r.Runtextend - minRequiredLength
}

func hasRequiredLengthAt(r *Runner, start int) bool {
	return start >= 0 && start <= latestPossibleStart(r)
}

func (r *Runner) initMatch(textInfo *matchText) {
	// Use a hashtable'ed Match object if the capture numbers are sparse

	if r.runmatch == nil {
		if r.re.caps != nil {
			r.runmatch = newMatchSparse(r.re, r.re.caps, r.re.capsize, textInfo, r.Runtextstart)
		} else {
			r.runmatch = newMatch(r.re, r.re.capsize, textInfo, r.Runtextstart)
		}
	} else {
		r.runmatch.reset(textInfo, r.Runtextstart)
	}

	// note we test runcrawl, because it is the last one to be allocated
	// If there is an alloc failure in the middle of the three allocations,
	// we may still return to reuse this instance, and we want to behave
	// as if the allocations didn't occur. (we used to test _trackcount != 0)

	if r.runcrawl != nil {
		r.Runtrackpos = len(r.runtrack)
		r.Runstackpos = len(r.runstack)
		r.runcrawlpos = len(r.runcrawl)
		return
	}

	r.initTrackCount()

	tracksize := r.runtrackcount * 8
	stacksize := r.runtrackcount * 8

	if tracksize < 64 {
		tracksize = 64
	}
	if limit := r.re.optimizations.MaxBacktrackingStackSize; limit >= 0 && tracksize > limit {
		tracksize = limit
	}
	if stacksize < 32 {
		stacksize = 32
	}

	r.runtrack = make([]int, tracksize)
	r.Runtrackpos = tracksize

	r.runstack = make([]int, stacksize)
	r.Runstackpos = stacksize

	r.runcrawl = make([]int, 32)
	r.runcrawlpos = 32
}

func (r *Runner) tidyMatch(quick bool) *Match {
	if !quick {
		match := r.runmatch

		r.runmatch = nil

		match.tidy(r.Runtextpos)
		return match
	} else {
		// send back our match -- it's not leaving the package, so it's safe to not clean it up
		// this reduces allocs for frequent calls to the "IsMatch" bool-only functions
		m := r.runmatch
		if m == nil {
			return nil
		}
		m.textpos = r.Runtextpos
		if m.matchcount[0] > 0 {
			interval := m.matches[0]
			// bytes indices aren't used so just use fast path
			m.RuneIndex = interval[0]
			m.RuneLength = interval[1]
		}
		return m
	}
}

// Capture captures a subexpression. Note that the
// capnum used here has already been mapped to a non-sparse
// index (by the code generator RegexWriter).
func (r *Runner) Capture(capnum, start, end int) {
	if end < start {
		T := end
		end = start
		start = T
	}

	r.crawl(capnum)
	r.runmatch.addMatch(capnum, start, end-start)
}

// transferCapture captures a subexpression. Note that the
// capnum used here has already been mapped to a non-sparse
// index (by the code generator RegexWriter).
func (r *Runner) transferCapture(capnum, uncapnum, start, end int) {
	var start2, end2 int

	// these are the two intervals that are cancelling each other

	if end < start {
		T := end
		end = start
		start = T
	}

	start2 = r.runmatch.matchIndex(uncapnum)
	end2 = start2 + r.runmatch.matchLength(uncapnum)

	// The new capture gets the innermost defined interval

	if start >= end2 {
		end = start
		start = end2
	} else if end <= start2 {
		start = start2
	} else {
		if end > end2 {
			end = end2
		}
		if start2 > start {
			start = start2
		}
	}

	r.crawl(uncapnum)
	r.runmatch.balanceMatch(uncapnum)

	if capnum != -1 {
		r.crawl(capnum)
		r.runmatch.addMatch(capnum, start, end-start)
	}
}

// revert the last capture
func (r *Runner) uncapture() {
	capnum := r.popcrawl()
	r.runmatch.removeMatch(capnum)
}

//debug

func (r *Runner) dumpState() {
	back := ""
	if r.operator&syntax.Back != 0 {
		back = " Back"
	}
	if r.operator&syntax.Back2 != 0 {
		back += " Back2"
	}
	fmt.Printf("Text:  %v\nTrack: %v\nStack: %v\n       %s%s\n\n",
		r.textposDescription(),
		r.stackDescription(r.runtrack, r.Runtrackpos),
		r.stackDescription(r.runstack, r.Runstackpos),
		r.code.OpcodeDescription(r.codepos),
		back)
}

func (r *Runner) stackDescription(a []int, index int) string {
	buf := &bytes.Buffer{}

	fmt.Fprintf(buf, "%v/%v", len(a)-index, len(a))
	if buf.Len() < 8 {
		buf.WriteString(strings.Repeat(" ", 8-buf.Len()))
	}

	buf.WriteRune('(')
	for i := index; i < len(a); i++ {
		if i > index {
			buf.WriteRune(' ')
		}

		buf.WriteString(strconv.Itoa(a[i]))
	}

	buf.WriteRune(')')

	return buf.String()
}

func (r *Runner) textposDescription() string {
	buf := &bytes.Buffer{}

	buf.WriteString(strconv.Itoa(r.Runtextpos))

	if buf.Len() < 8 {
		buf.WriteString(strings.Repeat(" ", 8-buf.Len()))
	}

	if r.Runtextpos > 0 {
		buf.WriteString(syntax.CharDescription(r.Runtext[r.Runtextpos-1]))
	} else {
		buf.WriteRune('^')
	}

	buf.WriteRune('>')

	for i := r.Runtextpos; i < r.Runtextend; i++ {
		buf.WriteString(syntax.CharDescription(r.Runtext[i]))
	}
	if buf.Len() >= 64 {
		buf.Truncate(61)
		buf.WriteString("...")
	} else {
		buf.WriteRune('$')
	}

	return buf.String()
}

// decide whether the pos
// at the specified index is a boundary or not. It's just not worth
// emitting inline code for this logic.
func (r *Runner) IsBoundary(index int) bool {
	return (index > 0 && syntax.IsWordChar(r.Runtext[index-1])) !=
		(index < r.Runtextend && syntax.IsWordChar(r.Runtext[index]))
}

func (r *Runner) IsECMABoundary(index int) bool {
	return (index > 0 && syntax.IsECMAWordChar(r.Runtext[index-1])) !=
		(index < r.Runtextend && syntax.IsECMAWordChar(r.Runtext[index]))
}

func (r *Runner) startTimeoutWatch() {
	if r.ignoreTimeout {
		return
	}
	r.deadline = makeDeadline(r.timeout)
}

func (r *Runner) CheckTimeout() error {
	if r.ignoreTimeout || !r.deadline.reached() {
		return nil
	}

	return fmt.Errorf("match timeout after %v on input `%v`", r.timeout, string(r.Runtext))
}

func (r *Runner) initTrackCount() {
	if r.code != nil {
		r.runtrackcount = r.code.TrackCount
	}
}

// decodeString converts s to []rune using a shared size-classed buffer pool when
// allowed by the regexp optimization settings. Pooled slices must be returned
// after the runner is done with them.
func (r *Runner) decodeString(s string) ([]rune, *[]rune) {
	buf, pooled := pooledRuneBuffers.get(len(s), r.re.optimizations.MaxCachedRuneBufferLength)
	n := 0
	for _, ch := range s {
		buf[n] = ch
		n++
	}
	return buf[:n], pooled
}

func (r *Runner) decodeStringWithStart(s string, startAt int) (runes []rune, runeStart int, pooled *[]rune) {
	buf, pooled := pooledRuneBuffers.get(len(s), r.re.optimizations.MaxCachedRuneBufferLength)
	n := 0
	runeStart = -1
	for strIdx, ch := range s {
		if startAt >= 0 && strIdx == startAt {
			runeStart = n
		}
		buf[n] = ch
		n++
	}
	if startAt >= 0 && startAt == len(s) {
		runeStart = n
	}
	return buf[:n], runeStart, pooled
}

// getRunner returns a runner to use for matching re.
func (re *Regexp) getRunner() *Runner {
	if re.runnerPool == nil {
		re.initCaches()
	}
	return re.runnerPool.Get().(*Runner)
}

// putRunner returns a runner to the re's pool cache.
func (re *Regexp) putRunner(r *Runner) {
	r.Runtext = nil
	r.code = re.code
	if r.runmatch != nil {
		r.runmatch.text = nil
	}
	re.runnerPool.Put(r)
}

func (r *Runner) LastIndexOfRune(startIndex int, endIndex int, find rune) int {
	for i := endIndex - 1; i >= startIndex; i-- {
		if r.Runtext[i] == find {
			return i
		}
	}
	return -1
}

// Undo captures until it reaches the specified capture position
func (r *Runner) UncaptureUntil(capturePos int) {
	for r.Crawlpos() > capturePos {
		r.uncapture()
	}
}

func (r *Runner) StackPop() int {
	//get it
	val := r.runstack[r.Runstackpos]
	// pop it
	r.Runstackpos++
	// return it
	return val
}

// StackDepth returns the number of integer slots currently used by the
// generated engine's backtracking stack.
func (r *Runner) StackDepth() int {
	return len(r.runstack) - r.Runstackpos
}

func (r *Runner) StackPush(val int) {
	// check if we need to size up stack
	r.ensureStack(1)
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val
}
func (r *Runner) StackPush2(val1, val2 int) {
	// check if we need to size up stack
	r.ensureStack(2)
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val1
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val2
}
func (r *Runner) StackPush3(val1, val2, val3 int) {
	// check if we need to size up stack
	r.ensureStack(3)
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val1
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val2
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val3
}
func (r *Runner) StackPush4(val1, val2, val3, val4 int) {
	// check if we need to size up stack
	r.ensureStack(4)
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val1
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val2
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val3
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val4
}
func (r *Runner) StackPush5(val1, val2, val3, val4, val5 int) {
	// check if we need to size up stack
	r.ensureStack(5)
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val1
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val2
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val3
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val4
	r.Runstackpos--
	r.runstack[r.Runstackpos] = val5
}
func (r *Runner) StackPushN(vals ...int) {
	// check if we need to size up stack
	r.ensureStack(len(vals))
	for _, val := range vals {
		r.Runstackpos--
		r.runstack[r.Runstackpos] = val
	}
}
func (r *Runner) IsMatched(cap int) bool {
	return r.runmatch.isMatched(cap)
}
func (r *Runner) MatchLength(cap int) int {
	return r.runmatch.matchLength(cap)
}
func (r *Runner) MatchIndex(cap int) int {
	return r.runmatch.matchIndex(cap)
}
