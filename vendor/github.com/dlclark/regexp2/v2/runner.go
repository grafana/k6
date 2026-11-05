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
	re   *Regexp
	code *syntax.Code

	// Keep the byte-sized flags together to avoid padding between word-sized fields.
	debug           bool
	ignoreTimeout   bool
	rightToLeft     bool
	caseInsensitive bool

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
	// Reusable metadata for internal string scans. Returned matches own theirs.
	textInfo *matchText

	timeout  time.Duration // timeout in milliseconds (needed for actual)
	deadline fasttime

	operator syntax.InstOp
	codepos  int

	// landmarkCore and landmarkCandidate come from the last required landmark
	// chain search in this scan, and landmarkCore is -1 before it. A search
	// from a later position up to landmarkCore would return the larger of that
	// position and landmarkCandidate.
	landmarkCore      int
	landmarkCandidate int
	// landmarks holds what the required landmark searches learned about
	// Runtext. Scans of the same Runtext between getRunner and putRunner
	// start where earlier ones ended, so they keep it. FindNextMatch moves it
	// to the match it returns and back.
	landmarks *requiredLandmarkState
}

// scanMode controls capture elision and ownership independently of text metadata.
type scanMode uint8

const (
	// scanBounds may elide unobserved captures and keeps the match in the runner.
	scanBounds scanMode = iota
	// scanCaptures records all captures and keeps the match in the runner.
	scanCaptures
	// scanFullMatch records all captures and returns an independently owned match.
	scanFullMatch
)

// run starts a search on rune input.
//
// mode selects bounds-only, borrowed-capture, or independently owned results.
// textstart is -1 to start at the "beginning" (depending on Right-To-Left), otherwise an index in input.
func (re *Regexp) run(mode scanMode, textstart int, input []rune) (*Match, error) {

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
	if mode == scanBounds && re.quickCode != nil {
		runner.code = re.quickCode
	}

	return runner.scan(input, nil, textstart, textstart, -1, mode, re.MatchTimeout)
}

// scan starts at candidate while preserving textstart for \G. Both are rune
// indexes in rt. A nil textInfo defers rune input metadata until a match is returned
// and allows borrowed scans to omit it entirely.
func (r *Runner) scan(rt []rune, textInfo *matchText, textstart, candidate, previousMatchLength int, mode scanMode, timeout time.Duration) (*Match, error) {
	r.timeout = timeout
	r.ignoreTimeout = (time.Duration(math.MaxInt64) == timeout)
	r.debug = r.re.Debug()
	r.Runtextstart = textstart
	// putRunner clears Runtext, so another call keeps landmarks only if
	// FindNextMatch sets them.
	if r.landmarks != nil && (len(r.Runtext) != len(rt) || len(rt) == 0 || &r.Runtext[0] != &rt[0]) {
		r.resetRequiredLandmarkSearches()
	}
	r.Runtext = rt
	r.Runtextend = len(rt)

	stoppos := r.Runtextend
	bump := 1

	if r.re.RightToLeft() {
		bump = -1
		stoppos = 0
	}

	r.Runtextpos = candidate
	r.landmarkCore = -1

	// setup our scanner functions
	findFirstChar := r.re.findFirstChar
	execute := r.re.execute
	if mode == scanBounds && r.re.executeQuick != nil {
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

	// An empty previous match must not be returned again. Keep Runtextstart at
	// the previous match position for \G, but move the candidate scan position.
	if previousMatchLength == 0 {
		if r.Runtextpos == stoppos {
			r.tidyMatch(scanBounds)
			return nil, nil
		}
		r.Runtextpos += bump
	}

	r.startTimeoutWatch()
	for {
		if minRequiredLength > 0 {
			if r.code.RightToLeft {
				if r.Runtextpos < minRequiredLength {
					r.tidyMatch(scanBounds)
					return nil, nil
				}
			} else if r.Runtextend-r.Runtextpos < minRequiredLength {
				r.tidyMatch(scanBounds)
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
				return r.tidyMatch(mode), nil
			}

			// reset state for another go
			r.Runtrackpos = len(r.runtrack)
			r.Runstackpos = len(r.runstack)
			r.runcrawlpos = len(r.runcrawl)
		}

		// failure!

		if r.Runtextpos == stoppos {
			r.tidyMatch(scanBounds)
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

		case syntax.Dispatch:
			// Dispatch only peeks at the next rune. It consumes it after finding a
			// matching branch, so a failed dispatch leaves the input position alone.
			if r.forwardchars() < 1 {
				break
			}
			// Pick the next rune in the current execution direction.
			pos := r.Runtextpos
			if r.rightToLeft {
				pos--
			}
			ch := r.Runtext[pos]
			tableIndex := r.operand(0)
			table := &r.code.Dispatches[tableIndex]
			branch := -1
			if ch >= 0 && ch < 128 {
				if table.ASCII != nil {
					// Larger dispatches use a direct ASCII branch lookup.
					branch = int(table.ASCII[ch]) - 1
				} else {
					// Smaller dispatches use two compact ASCII bitmasks per set.
					word := int(ch >> 6)
					bit := uint64(1) << (ch & 63)
					for i := range table.Sets {
						if table.ASCIIMasks[i*2+word]&bit != 0 {
							branch = i
							break
						}
					}
				}
			} else {
				// Non-ASCII runes fall back to the complete character sets.
				for i, setIndex := range table.Sets {
					if r.code.Sets[setIndex].Contains(ch) {
						branch = i
						break
					}
				}
			}
			if branch < 0 {
				break
			}
			// The selected branch starts with this rune, so consume it and jump
			// directly to the rest of that branch.
			r.Runtextpos += r.bump()
			if err := r.goTo(table.Branches[branch]); err != nil {
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
			if matched == 0 && count > 0 && r.re.options&ECMAScript != 0 {
				// ECMAScript 2025 §22.2.2.3.1 RepeatMatcher, step 2.2,
				// rejects an empty iteration once no required repeats remain.
				// Fail the iteration so its captures are undone and any remaining
				// alternatives in the body can still be tried.
				// https://tc39.es/ecma262/2025/multipage/text-processing.html#sec-repeatmatcher
				r.stackPush2(mark, count)
				break
			}

			if count >= r.operand(1) || (matched == 0 && count >= 0 && r.re.options&ECMAScript == 0) { // Max loops or empty match -> straight now
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
			if r.textPos() == mark && count > 0 && r.re.options&ECMAScript != 0 {
				// RepeatMatcher step 2.2 applies to lazy quantifiers too.
				r.stackPush2(mark, count)
				break
			}

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

			// RepeatMatcher permits a required empty iteration, then tries
			// another iteration; step 2.2 rejects it only if it is also empty.
			if r.trackPeekN(1) < r.operand(1) && (textpos != mark || (r.trackPeekN(1) == 0 && r.re.options&ECMAScript != 0)) { // Under limit and another iteration is permitted
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
			if r.leftchars() > 0 && !r.isLineBreak(r.charAt(r.textPos()-1)) {
				break
			}
			r.advance(0)
			continue

		case syntax.Eol:
			if r.rightchars() > 0 && !r.isLineBreak(r.charAt(r.textPos())) {
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
			if !r.IsECMABoundaryWithOptions(r.textPos(), r.ecmaBoundaryOptions()) {
				break
			}
			r.advance(0)
			continue

		case syntax.NonECMABoundary:
			if r.IsECMABoundaryWithOptions(r.textPos(), r.ecmaBoundaryOptions()) {
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

			if r.forwardchars() < 1 || !r.code.Sets[r.operand(0)].Contains(r.forwardcharnext()) {
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

		case syntax.Grapheme:
			if !r.TryMatchGrapheme(r.rightToLeft) {
				break
			}
			r.advance(0)
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
				if !set.Contains(r.forwardcharnext()) {
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
				if !set.Contains(r.forwardcharnext()) {
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

			if !r.code.Sets[r.operand(0)].Contains(r.forwardcharnext()) {
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
			// Keep resets outside the main dispatch: adding them to its cases
			// slows existing programs that never execute a reset instruction.
			switch r.operator {
			case syntax.ResetCapture:
				capnum := r.operand(0)
				if r.runmatch.isMatched(capnum) {
					// Use the crawl stack so lookarounds and atomic groups can
					// undo the reset when they unwind discarded backtracking frames.
					r.crawl(capnum)
					r.runmatch.balanceMatch(capnum)
					r.trackPush()
				}
				r.advance(1)
				continue

			case syntax.ResetCapture | syntax.Back:
				r.uncapture()

			default:
				return fmt.Errorf("unknown state in regex runner: %v", r.operator)
			}
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

// TryMatchGrapheme consumes one Unicode extended grapheme cluster in the
// runner's current direction. It is exported for regexp2cg-generated engines.
func (r *Runner) TryMatchGrapheme(rightToLeft bool) bool {
	var boundary int
	if rightToLeft {
		boundary = syntax.PreviousGraphemeClusterBoundary(r.Runtext, r.Runtextpos)
	} else {
		boundary = syntax.NextGraphemeClusterBoundary(r.Runtext, r.Runtextpos)
	}
	if boundary < 0 {
		return false
	}
	r.Runtextpos = boundary
	return true
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

			if !syntax.IsCaseEquivalent(r.Runtext[cmpos], r.Runtext[pos], syntax.RegexOptions(r.re.options)) {
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
	// A fixed-length expression ending at an end anchor has at most two
	// possible starts. Use that information before scanning a literal prefix.
	if opts := r.code.FindOptimizations; opts != nil {
		switch opts.FindMode {
		case syntax.TrailingAnchor_FixedLength_LeftToRight_End:
			return findTrailingFixedLengthEnd(r, opts.MinRequiredLength, false)
		case syntax.TrailingAnchor_FixedLength_LeftToRight_EndZ:
			return findTrailingFixedLengthEnd(r, opts.MinRequiredLength, true)
		}
	}

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
			//fmt.Printf("%v in %v: %v\n", string(n), set.String(), set.Contains(n))
			if set.Contains(n) {
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
		syntax.TrailingAnchor_FixedLength_LeftToRight_EndZ,
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
		return true, findTrailingFixedLengthEnd(r, opts.MinRequiredLength, false)
	case syntax.TrailingAnchor_FixedLength_LeftToRight_EndZ:
		return true, findTrailingFixedLengthEnd(r, opts.MinRequiredLength, true)
	case syntax.LeadingString_LeftToRight:
		return true, findLeadingStringLeftToRight(r, []rune(opts.LeadingPrefix), false)
	case syntax.LeadingString_OrdinalIgnoreCase_LeftToRight:
		return true, findLeadingStringLeftToRight(r, []rune(opts.LeadingPrefix), true)
	case syntax.LeadingStrings_LeftToRight:
		if r.re != nil && r.re.prefixSearch != nil && r.re.prefixSearch.shouldUseRunes(r.Runtext, r.Runtextpos) {
			start := r.re.prefixSearch.indexRunes(r.Runtext, r.Runtextpos)
			if start >= 0 && hasRequiredLengthAt(r, start) {
				r.Runtextpos = start
				return true, true
			}
			r.Runtextpos = r.Runtextend
			return true, false
		}
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

func findTrailingFixedLengthEnd(r *Runner, fixedLength int, allowFinalNewline bool) bool {
	start := r.Runtextend - fixedLength
	if start < r.Runtextpos || start < 0 {
		r.Runtextpos = r.Runtextend
		return false
	}
	// Prefer the position before a final newline when allowed by the anchor.
	// A rejected prefix must still allow the absolute-end candidate: the
	// expression itself may consume the newline.
	prefix := r.code.BmPrefix
	if allowFinalNewline && r.Runtextend > 0 && r.Runtext[r.Runtextend-1] == '\n' && start-1 >= r.Runtextpos {
		if prefix == nil || prefix.IsMatch(r.Runtext, start-1, 0, r.Runtextend) {
			r.Runtextpos = start - 1
			return true
		}
	}
	// Retain the literal prefilter when narrowing the search to the end.
	// Otherwise even a first-character mismatch would enter the interpreter.
	if prefix != nil && !prefix.IsMatch(r.Runtext, start, 0, r.Runtextend) {
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
		for start > r.Runtextpos && literal.LoopNode.Set.Contains(r.Runtext[start-1]) {
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

	if r.Runtextpos <= r.landmarkCore {
		if candidate := max(r.Runtextpos, r.landmarkCandidate); hasRequiredLengthAt(r, candidate) {
			r.Runtextpos = candidate
			return true
		}
	}
	searches := r.requiredLandmarkSearches(chain)

	for searchStart := r.Runtextpos; searchStart <= latestPossibleStart(r); {
		first, ok := searches[0].next(r.Runtext, searchStart, r.Runtextend, chain.Landmarks[0])
		if !ok {
			r.Runtextpos = r.Runtextend
			return false
		}

		nextStart := first.End
		for i := 1; i < len(chain.Landmarks); i++ {
			landmark, ok := searches[i].next(r.Runtext, nextStart, r.Runtextend, chain.Landmarks[i])
			if !ok {
				r.Runtextpos = r.Runtextend
				return false
			}
			nextStart = landmark.End
		}

		candidate := requiredLandmarkStart(r, chain, first.CoreStart, searches[0].alternatives)
		for candidate > r.Runtextpos && chain.LeadingLoopSet.Contains(r.Runtext[candidate-1]) {
			candidate--
		}
		if hasRequiredLengthAt(r, candidate) {
			r.Runtextpos = candidate
			r.landmarkCore, r.landmarkCandidate = first.CoreStart, candidate
			return true
		}

		searchStart = first.CoreStart + 1
	}

	r.Runtextpos = r.Runtextend
	return false
}

// requiredLandmarkState holds what the searches for chain's landmarks learned
// about one input.
type requiredLandmarkState struct {
	chain    *syntax.RequiredLandmarkChain
	searches []requiredLandmarkSearch
}

// requiredLandmarkSearches returns the search state of chain's landmarks.
func (r *Runner) requiredLandmarkSearches(chain *syntax.RequiredLandmarkChain) []requiredLandmarkSearch {
	if r.landmarks == nil || r.landmarks.chain != chain {
		searches := make([]requiredLandmarkSearch, len(chain.Landmarks))
		for i, landmark := range chain.Landmarks {
			searches[i].alternatives = make([]requiredLandmarkCursor, len(landmark.Alternatives))
		}
		r.landmarks = &requiredLandmarkState{chain: chain, searches: searches}
	}
	return r.landmarks.searches
}

// resetRequiredLandmarkSearches discards everything learned about Runtext.
func (r *Runner) resetRequiredLandmarkSearches() {
	for i := range r.landmarks.searches {
		s := &r.landmarks.searches[i]
		s.last, s.found = requiredLandmarkMatch{}, false
		for j := range s.alternatives {
			s.alternatives[j] = requiredLandmarkCursor{prefix: s.alternatives[j].prefix}
		}
	}
}

type requiredLandmarkMatch struct {
	CoreStart int
	End       int
}

// requiredLandmarkSearch finds the cores of one landmark. Calls must pass the
// same input, endAt and landmark and must not decrease startAt, so each
// cursor only moves forward. A call makes O(1) calls to each cursor per start
// it passes, so the calls on one input read O(n) runes per alternative.
type requiredLandmarkSearch struct {
	// last is the last core found, if found. It is also the result of a
	// call from any position up to last.CoreStart.
	last         requiredLandmarkMatch
	found        bool
	alternatives []requiredLandmarkCursor
}

// next returns the first core at or after startAt. Its End holds for
// whichever alternative the VM uses, at this core or a later one.
func (s *requiredLandmarkSearch) next(input []rune, startAt, endAt int, landmark syntax.RequiredLandmark) (requiredLandmarkMatch, bool) {
	if s.found && startAt <= s.last.CoreStart {
		return s.last, true
	}
	for i := startAt; i < endAt; {
		// Let the cursors read up to about twice as far from startAt as i,
		// so that each reads many runes per call.
		limit := endAt
		if i-startAt < endAt-i {
			limit = i + (i - startAt) + 1
		}
		// After this pass no alternative has a core in [i, first). Unless
		// first is limit, one has a core at first or cannot rule one out
		// yet. found is the alternative with a core at first, if no earlier
		// one is still open there.
		first, found := limit, -1
		var match requiredLandmarkMatch
		for j := 0; j < len(landmark.Alternatives) && first > i; j++ {
			c := &s.alternatives[j]
			if m, ok := c.find(input, i, endAt, first, first, &landmark.Alternatives[j]); ok {
				match, first, found = m, m.CoreStart, j
			} else if u := c.undecided(endAt); u < first {
				first, found = u, -1
			}
		}
		if first == limit {
			i = limit
			continue
		}
		// Unless found is set, test the alternatives at first in order,
		// reading as far as each needs. No alternative has a core in
		// [startAt, first), so these reads decide the earliest remaining start.
		for j := 0; j < len(landmark.Alternatives) && found < 0; j++ {
			if m, ok := s.alternatives[j].find(input, first, endAt, first+1, endAt, &landmark.Alternatives[j]); ok {
				match, found = m, j
			}
		}
		if found < 0 {
			i = first + 1
			continue
		}
		match = s.bound(input, landmark, found, match)
		// The last End also holds here, since this core is later. Keeping
		// the larger one keeps the next landmark's startAt from decreasing.
		if s.found {
			match.End = max(match.End, s.last.End)
		}
		s.last, s.found = match, true
		return match, true
	}
	return requiredLandmarkMatch{}, false
}

// bound lowers match.End so that it holds for every alternative. Another
// alternative can end before End only if its core starts before End, so End
// drops to the earliest end that such a start allows.
func (s *requiredLandmarkSearch) bound(input []rune, landmark syntax.RequiredLandmark, found int, match requiredLandmarkMatch) requiredLandmarkMatch {
	for j := range landmark.Alternatives {
		if j == found {
			continue
		}
		alt := &landmark.Alternatives[j]
		minLength := alt.MinRepeat
		if len(alt.Literal) > 0 {
			minLength = len(alt.Literal)
		}
		c := &s.alternatives[j]
		c.first = max(c.first, match.CoreStart)
		// minLength can be close to MaxInt32, so first+minLength can overflow
		// int on 32-bit platforms.
		for minLength < match.End-c.first {
			ch := input[c.first]
			if (len(alt.Literal) > 0 && ch == alt.Literal[0]) || (alt.Set != nil && alt.Set.Contains(ch)) {
				match.End = c.first + minLength
				break
			}
			c.first++
		}
	}
	return match
}

// requiredLandmarkCursor finds the cores of one landmark alternative. Calls
// to find and reachable must pass the same input, endAt and alternative and
// must not decrease their start. It keeps what it has read, so its calls on
// one input read O(n) runes plus O(1) per call, whatever the repeat counts
// and literal lengths.
type requiredLandmarkCursor struct {
	// prefix is the KMP failure function of a literal core. It depends only
	// on the alternative.
	prefix []int

	// For a set core, pos is the next start to test. For a literal core, it
	// is the next rune to read, and the matched runes before it are the
	// longest proper prefix of the literal there that starts at or after the
	// last startAt. Neither is used once exhausted.
	pos, matched int
	// core is the first core at or after the last startAt, if found. If
	// exhausted, there is none.
	core             requiredLandmarkMatch
	found, exhausted bool

	// The runes from the current start to setEnd are in Set. The runes from
	// the current start plus MinRepeat to trailingEnd are not in
	// TrailingWhitespaceSet.
	setEnd, trailingEnd int

	// For bound: no rune from the last core to first can start this
	// alternative.
	first int

	// For reachable: the runes from the last pos to leadingEnd are in
	// LeadingWhitespaceSet, and the rune at leadingEnd, if any, is not.
	leadingEnd int
}

// find returns the first core that starts in [startAt, limit). It reads past
// limit only to finish a core that starts before limit, and reads no rune at
// or past readEnd other than the one after a literal core. If a start before
// the first core needs such runes, find returns no core and leaves that start
// for a later call. Unlike startAt, limit and readEnd can decrease between
// calls.
func (c *requiredLandmarkCursor) find(input []rune, startAt, endAt, limit, readEnd int, alt *syntax.RequiredLandmarkAlternative) (requiredLandmarkMatch, bool) {
	if c.exhausted || c.found && startAt <= c.core.CoreStart {
		return c.core, c.found && c.core.CoreStart < limit
	}
	c.found = false
	if c.pos < startAt {
		c.pos, c.matched = startAt, 0
	}
	if len(alt.Literal) > 0 {
		c.findLiteral(input, startAt, endAt, limit, readEnd, alt)
	} else {
		c.findSet(input, endAt, limit, readEnd, alt)
	}
	if !c.found && c.pos >= endAt {
		c.exhausted = true
	}
	return c.core, c.found
}

// undecided returns the first start in [startAt, limit) that the last call to
// find, which found no core, left for a later one, or a start at or past limit
// if there is none.
func (c *requiredLandmarkCursor) undecided(endAt int) int {
	if c.exhausted {
		return endAt
	}
	return c.pos - c.matched
}

func (c *requiredLandmarkCursor) findLiteral(input []rune, startAt, endAt, limit, readEnd int, alt *syntax.RequiredLandmarkAlternative) {
	literal := alt.Literal
	if c.prefix == nil {
		c.prefix = make([]int, len(literal))
		for i, k := 1, 0; i < len(literal); i++ {
			for k > 0 && literal[i] != literal[k] {
				k = c.prefix[k-1]
			}
			if literal[i] == literal[k] {
				k++
			}
			c.prefix[i] = k
		}
	}
	// A match that starts before startAt cannot be a core. Keep the shorter
	// ones within it that start at or after startAt.
	for c.matched > 0 && c.pos-c.matched < startAt {
		c.matched = c.prefix[c.matched-1]
	}
	// Stop once the matched prefix starts at or past limit.
	for c.pos < readEnd && c.pos-c.matched < limit {
		r := input[c.pos]
		for c.matched > 0 && r != literal[c.matched] {
			c.matched = c.prefix[c.matched-1]
		}
		if r == literal[c.matched] {
			c.matched++
		}
		c.pos++
		if c.matched < len(literal) {
			continue
		}
		start := c.pos - len(literal)
		c.matched = c.prefix[len(literal)-1]
		if (!alt.RequireWhitespaceBefore || start > 0 && alt.LeadingWhitespaceSet.Contains(input[start-1])) &&
			(!alt.RequireWhitespaceAfter || c.pos < endAt && alt.TrailingWhitespaceSet.Contains(input[c.pos])) {
			c.core, c.found = requiredLandmarkMatch{CoreStart: start, End: c.pos}, true
			return
		}
	}
	if c.pos-c.matched > endAt-len(literal) {
		// No start that is left fits.
		c.pos = endAt
	}
}

// findSet looks for the shortest core at each start. The VM can backtrack a
// longer one, so a longer core could pass the whitespace required after it or
// the start of the next landmark.
func (c *requiredLandmarkCursor) findSet(input []rune, endAt, limit, readEnd int, alt *syntax.RequiredLandmarkAlternative) {
	for c.pos < limit {
		start := c.pos
		// Test the fit first, so that the cursor stops even if no later start
		// follows whitespace. MinRepeat can be close to MaxInt32, so
		// start+MinRepeat can overflow int on 32-bit platforms.
		if alt.MinRepeat > endAt-start {
			c.pos = endAt
			return
		}
		if alt.RequireWhitespaceBefore && (start == 0 || !alt.LeadingWhitespaceSet.Contains(input[start-1])) {
			c.pos++
			continue
		}
		if c.setEnd < start {
			c.setEnd = start
		}
		end := start + alt.MinRepeat
		if alt.RequireWhitespaceAfter {
			end = max(end, c.trailingEnd)
			for end < readEnd && end-start <= alt.MaxRepeat && c.extendSet(input, end, alt) && !alt.TrailingWhitespaceSet.Contains(input[end]) {
				end++
			}
			c.trailingEnd = end
			if end-start > alt.MaxRepeat {
				// No start before this one can reach whitespace either.
				c.pos = end - alt.MaxRepeat
				continue
			}
			if end >= endAt {
				c.pos = endAt
				return
			}
		}
		if !c.extendSet(input, min(end, readEnd), alt) {
			// The set run ends too early for every start up to its end.
			c.pos = c.setEnd + 1
			continue
		}
		// Deciding this start needs the runes from readEnd on.
		if end > readEnd || end == readEnd && alt.RequireWhitespaceAfter {
			return
		}
		c.pos = start + 1
		c.core, c.found = requiredLandmarkMatch{CoreStart: start, End: end}, true
		return
	}
}

// extendSet reports whether the runes from the current start to end are in
// Set. It reads only as far as end.
func (c *requiredLandmarkCursor) extendSet(input []rune, end int, alt *syntax.RequiredLandmarkAlternative) bool {
	for c.setEnd < end {
		if !alt.Set.Contains(input[c.setEnd]) {
			break
		}
		c.setEnd++
	}
	return c.setEnd >= end
}

// reachable reports whether alt's core starts at pos or right after a run of
// alt's leading whitespace from pos.
func (c *requiredLandmarkCursor) reachable(input []rune, pos, endAt int, alt *syntax.RequiredLandmarkAlternative) bool {
	if c.leadingEnd <= pos {
		c.leadingEnd = pos
		for c.leadingEnd < endAt && alt.LeadingWhitespaceSet.Contains(input[c.leadingEnd]) {
			c.leadingEnd++
		}
	}
	_, ok := c.find(input, pos, endAt, min(c.leadingEnd+1, endAt), endAt, alt)
	return ok
}

// requiredLandmarkStart returns a position, not before Runtextpos, that is at
// or before the start of any match whose first landmark core starts at or
// after core, once LeadingLoopSet is rewound from it. An alternative's leading
// whitespace can start such a match before core only if the chain's anchors
// hold where the alternative starts, and the alternative's core starts at
// core or right after a run of that whitespace from core.
func requiredLandmarkStart(r *Runner, chain *syntax.RequiredLandmarkChain, core int, cursors []requiredLandmarkCursor) int {
	input, minStart := r.Runtext, r.Runtextpos
	landmark := chain.Landmarks[0]
	anchored := len(chain.Anchors) > 0
	start := core
	for j := range landmark.Alternatives {
		if start == minStart {
			break
		}
		alt := &landmark.Alternatives[j]
		ws := alt.LeadingWhitespaceSet
		if ws == nil {
			continue
		}
		// Rewind only to just before start at first. An alternative that
		// cannot get there needs no more work.
		altStart := core
		for altStart >= start && altStart > minStart && ws.Contains(input[altStart-1]) {
			altStart--
		}
		if altStart >= start {
			continue
		}
		// Without anchors, test reachable before the rest of the rewind, so
		// that a failure skips it. With anchors, rewind first: they can fail
		// at every start the rewind reaches, and then reachable need not read
		// ahead.
		if !anchored && !cursors[j].reachable(input, core, r.Runtextend, alt) {
			continue
		}
		for altStart > minStart && ws.Contains(input[altStart-1]) {
			altStart--
		}
		for altStart < start && !requiredLandmarkAnchorsHold(r, chain.Anchors, altStart) {
			altStart++
		}
		if altStart < start && (!anchored || cursors[j].reachable(input, core, r.Runtextend, alt)) {
			start = altStart
		}
	}
	return start
}

// requiredLandmarkAnchorsHold reports whether the VM would pass every anchor
// at pos.
func requiredLandmarkAnchorsHold(r *Runner, anchors []syntax.NodeType, pos int) bool {
	for _, anchor := range anchors {
		ok := true
		switch anchor {
		case syntax.NtBol:
			ok = pos == 0 || r.isLineBreak(r.Runtext[pos-1])
		case syntax.NtEol:
			ok = pos == r.Runtextend || r.isLineBreak(r.Runtext[pos])
		case syntax.NtBoundary:
			ok = r.IsBoundary(pos)
		case syntax.NtNonboundary:
			ok = !r.IsBoundary(pos)
		// Boundaries with the IgnoreCase|Unicode word characters are not
		// landmark anchors (see findRequiredLandmarkChain).
		case syntax.NtECMABoundary:
			ok = r.IsECMABoundaryWithOptions(pos, None)
		case syntax.NtNonECMABoundary:
			ok = !r.IsECMABoundaryWithOptions(pos, None)
		case syntax.NtBeginning:
			ok = pos == 0
		case syntax.NtStart:
			ok = pos == r.Runtextstart
		case syntax.NtEndZ:
			// RE2 and ECMAScript allow only the end. Otherwise a final '\n'
			// may follow.
			rest := r.Runtextend - pos
			ok = rest == 0 || rest == 1 && r.re.options&(RE2|ECMAScript) == 0 && r.Runtext[pos] == '\n'
		case syntax.NtEnd:
			ok = pos == r.Runtextend
		}
		if !ok {
			return false
		}
	}
	return true
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
	if len(set.Chars) > 5 && set.Set != nil {
		// The class already has an ASCII bitmap. Avoid constructing another
		// one each time a candidate search resumes, and keep Unicode handling
		// and negation in the class's membership test.
		for i, ch := range chars {
			if set.Set.Contains(ch) {
				return i
			}
		}
		return -1
	}
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
	if len(set.Chars) > 5 && set.Set != nil {
		return set.Set.Contains(ch)
	}
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
	return set.Set != nil && set.Set.Contains(ch)
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

func (r *Runner) stringMatchText(input string, runes []rune, runeOffset, byteOffset int) *matchText {
	if r.textInfo == nil {
		r.textInfo = new(matchText)
	}
	r.textInfo.resetString(input, runes, runeOffset, byteOffset)
	return r.textInfo
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

func (r *Runner) tidyMatch(mode scanMode) *Match {
	if mode == scanFullMatch {
		match := r.runmatch

		r.runmatch = nil

		// Rune input needs capture text metadata only when a match escapes the runner.
		if match.text == nil {
			match.text = newMatchText(r.Runtext)
		}
		match.tidy(r.Runtextpos)
		return match
	} else {
		// Borrowed results need only the whole-match bounds and continuation
		// position. MatchView reads capture ranges directly from the engine's
		// bookkeeping, including unresolved balancing references.
		m := r.runmatch
		if m == nil {
			return nil
		}
		m.textpos = r.Runtextpos
		if m.matchcount[0] > 0 {
			interval := m.matches[0]
			setCaptureFields(&m.Capture, interval[0], interval[1])
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

// TransferCapture removes the last capture from uncapnum and optionally
// captures its interval with [start, end) into capnum. Use -1 for capnum to
// only remove the capture. uncapnum must be matched, and both group numbers
// must already be mapped to dense capture slots. UncaptureUntil can undo it.
func (r *Runner) TransferCapture(capnum, uncapnum, start, end int) {
	r.transferCapture(capnum, uncapnum, start, end)
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

// isLineBreak reports whether ch ends a line for the multiline ^ and $ anchors.
func (r *Runner) isLineBreak(ch rune) bool {
	return ch == '\n' || r.re.options&ECMAScript != 0 && syntax.IsECMALineTerminator(ch)
}

// IsECMABoundary reports whether index is between a letter, nonspacing mark,
// decimal digit or connector punctuation and something else.
//
// Deprecated: IsECMABoundary is kept so that existing generated engines keep
// their behavior. It does not use the ECMAScript word characters; use
// IsECMABoundaryWithOptions.
func (r *Runner) IsECMABoundary(index int) bool {
	return (index > 0 && syntax.IsECMAWordChar(r.Runtext[index-1])) !=
		(index < r.Runtextend && syntax.IsECMAWordChar(r.Runtext[index]))
}

// IsECMABoundaryWithOptions reports whether index is an ECMAScript word
// boundary for a \b or \B with the given options: whether exactly one of the
// characters before and after index is in the WordCharacters, which are
// [A-Za-z0-9_], plus U+017F and U+212A with both IgnoreCase and Unicode.
func (r *Runner) IsECMABoundaryWithOptions(index int, options RegexOptions) bool {
	opts := syntax.RegexOptions(options)
	return (index > 0 && syntax.IsECMAWordCharWithOptions(opts, r.Runtext[index-1])) !=
		(index < r.Runtextend && syntax.IsECMAWordCharWithOptions(opts, r.Runtext[index]))
}

// ecmaBoundaryOptions returns the options of the current \b or \B operator
// that choose its word characters. The writer sets Ci on it only when its
// scoped options have both IgnoreCase and Unicode (see RegexNode.reduce).
func (r *Runner) ecmaBoundaryOptions() RegexOptions {
	if r.caseInsensitive {
		return IgnoreCase | Unicode
	}
	return None
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
	if r.textInfo != nil {
		// Release the input and offset cache, keeping the metadata object for reuse.
		*r.textInfo = matchText{}
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
