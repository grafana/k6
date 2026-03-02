package cellbuf

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
)

// ErrInvalidDimensions is returned when the dimensions of a window are invalid
// for the operation.
var ErrInvalidDimensions = errors.New("invalid dimensions")

// notLocal returns whether the coordinates are not considered local movement
// using the defined thresholds.
// This takes the number of columns, and the coordinates of the current and
// target positions.
func notLocal(cols, fx, fy, tx, ty int) bool {
	// The typical distance for a [ansi.CUP] sequence. Anything less than this
	// is considered local movement.
	const longDist = 8 - 1
	return (tx > longDist) &&
		(tx < cols-1-longDist) &&
		(abs(ty-fy)+abs(tx-fx) > longDist)
}

// relativeCursorMove returns the relative cursor movement sequence using one or two
// of the following sequences [ansi.CUU], [ansi.CUD], [ansi.CUF], [ansi.CUB],
// [ansi.VPA], [ansi.HPA].
// When overwrite is true, this will try to optimize the sequence by using the
// screen cells values to move the cursor instead of using escape sequences.
func relativeCursorMove(s *Screen, fx, fy, tx, ty int, overwrite, useTabs, useBackspace bool) string {
	var seq strings.Builder

	width, height := s.newbuf.Width(), s.newbuf.Height()
	if ty != fy {
		var yseq string
		if s.xtermLike && !s.opts.RelativeCursor {
			yseq = ansi.VerticalPositionAbsolute(ty + 1)
		}

		// OPTIM: Use [ansi.LF] and [ansi.ReverseIndex] as optimizations.

		if ty > fy {
			n := ty - fy
			if cud := ansi.CursorDown(n); yseq == "" || len(cud) < len(yseq) {
				yseq = cud
			}
			shouldScroll := !s.opts.AltScreen && fy+n >= s.scrollHeight
			if lf := strings.Repeat("\n", n); shouldScroll || (fy+n < height && len(lf) < len(yseq)) {
				// TODO: Ensure we're not unintentionally scrolling the screen down.
				yseq = lf
				s.scrollHeight = max(s.scrollHeight, fy+n)
			}
		} else if ty < fy {
			n := fy - ty
			if cuu := ansi.CursorUp(n); yseq == "" || len(cuu) < len(yseq) {
				yseq = cuu
			}
			if n == 1 && fy-1 > 0 {
				// TODO: Ensure we're not unintentionally scrolling the screen up.
				yseq = ansi.ReverseIndex
			}
		}

		seq.WriteString(yseq)
	}

	if tx != fx {
		var xseq string
		if s.xtermLike && !s.opts.RelativeCursor {
			xseq = ansi.HorizontalPositionAbsolute(tx + 1)
		}

		if tx > fx {
			n := tx - fx
			if useTabs {
				var tabs int
				var col int
				for col = fx; s.tabs.Next(col) <= tx; col = s.tabs.Next(col) {
					tabs++
					if col == s.tabs.Next(col) || col >= width-1 {
						break
					}
				}

				if tabs > 0 {
					cht := ansi.CursorHorizontalForwardTab(tabs)
					tab := strings.Repeat("\t", tabs)
					if false && s.xtermLike && len(cht) < len(tab) {
						// TODO: The linux console and some terminals such as
						// Alacritty don't support [ansi.CHT]. Enable this when
						// we have a way to detect this, or after 5 years when
						// we're sure everyone has updated their terminals :P
						seq.WriteString(cht)
					} else {
						seq.WriteString(tab)
					}

					n = tx - col
					fx = col
				}
			}

			if cuf := ansi.CursorForward(n); xseq == "" || len(cuf) < len(xseq) {
				xseq = cuf
			}

			// If we have no attribute and style changes, overwrite is cheaper.
			var ovw string
			if overwrite && ty >= 0 {
				for i := 0; i < n; i++ {
					cell := s.newbuf.Cell(fx+i, ty)
					if cell != nil && cell.Width > 0 {
						i += cell.Width - 1
						if !cell.Style.Equal(&s.cur.Style) || !cell.Link.Equal(&s.cur.Link) {
							overwrite = false
							break
						}
					}
				}
			}

			if overwrite && ty >= 0 {
				for i := 0; i < n; i++ {
					cell := s.newbuf.Cell(fx+i, ty)
					if cell != nil && cell.Width > 0 {
						ovw += cell.String()
						i += cell.Width - 1
					} else {
						ovw += " "
					}
				}
			}

			if overwrite && len(ovw) < len(xseq) {
				xseq = ovw
			}
		} else if tx < fx {
			n := fx - tx
			if useTabs && s.xtermLike {
				// VT100 does not support backward tabs [ansi.CBT].

				col := fx

				var cbt int // cursor backward tabs count
				for s.tabs.Prev(col) >= tx {
					col = s.tabs.Prev(col)
					cbt++
					if col == s.tabs.Prev(col) || col <= 0 {
						break
					}
				}

				if cbt > 0 {
					seq.WriteString(ansi.CursorBackwardTab(cbt))
					n = col - tx
				}
			}

			if cub := ansi.CursorBackward(n); xseq == "" || len(cub) < len(xseq) {
				xseq = cub
			}

			if useBackspace && n < len(xseq) {
				xseq = strings.Repeat("\b", n)
			}
		}

		seq.WriteString(xseq)
	}

	return seq.String()
}

// moveCursor moves and returns the cursor movement sequence to move the cursor
// to the specified position.
// When overwrite is true, this will try to optimize the sequence by using the
// screen cells values to move the cursor instead of using escape sequences.
func moveCursor(s *Screen, x, y int, overwrite bool) (seq string) {
	fx, fy := s.cur.X, s.cur.Y

	if !s.opts.RelativeCursor {
		// Method #0: Use [ansi.CUP] if the distance is long.
		seq = ansi.CursorPosition(x+1, y+1)
		if fx == -1 || fy == -1 || notLocal(s.newbuf.Width(), fx, fy, x, y) {
			return
		}
	}

	// Optimize based on options.
	trials := 0
	if s.opts.HardTabs {
		trials |= 2 // 0b10 in binary
	}
	if s.opts.Backspace {
		trials |= 1 // 0b01 in binary
	}

	// Try all possible combinations of hard tabs and backspace optimizations.
	for i := 0; i <= trials; i++ {
		// Skip combinations that are not enabled.
		if i & ^trials != 0 {
			continue
		}

		useHardTabs := i&2 != 0
		useBackspace := i&1 != 0

		// Method #1: Use local movement sequences.
		nseq := relativeCursorMove(s, fx, fy, x, y, overwrite, useHardTabs, useBackspace)
		if (i == 0 && len(seq) == 0) || len(nseq) < len(seq) {
			seq = nseq
		}

		// Method #2: Use [ansi.CR] and local movement sequences.
		nseq = "\r" + relativeCursorMove(s, 0, fy, x, y, overwrite, useHardTabs, useBackspace)
		if len(nseq) < len(seq) {
			seq = nseq
		}

		if !s.opts.RelativeCursor {
			// Method #3: Use [ansi.CursorHomePosition] and local movement sequences.
			nseq = ansi.CursorHomePosition + relativeCursorMove(s, 0, 0, x, y, overwrite, useHardTabs, useBackspace)
			if len(nseq) < len(seq) {
				seq = nseq
			}
		}
	}

	return
}

// moveCursor moves the cursor to the specified position.
func (s *Screen) moveCursor(x, y int, overwrite bool) {
	if !s.opts.AltScreen && s.cur.X == -1 && s.cur.Y == -1 {
		// First cursor movement in inline mode, move the cursor to the first
		// column before moving to the target position.
		s.buf.WriteByte('\r') //nolint:errcheck
		s.cur.X, s.cur.Y = 0, 0
	}
	s.buf.WriteString(moveCursor(s, x, y, overwrite)) //nolint:errcheck
	s.cur.X, s.cur.Y = x, y
}

func (s *Screen) move(x, y int) {
	// XXX: Make sure we use the max height and width of the buffer in case
	// we're in the middle of a resize operation.
	width := max(s.newbuf.Width(), s.curbuf.Width())
	height := max(s.newbuf.Height(), s.curbuf.Height())

	if width > 0 && x >= width {
		// Handle autowrap
		y += (x / width)
		x %= width
	}

	// XXX: Disable styles if there's any
	// Some move operations such as [ansi.LF] can apply styles to the new
	// cursor position, thus, we need to reset the styles before moving the
	// cursor.
	blank := s.clearBlank()
	resetPen := y != s.cur.Y && !blank.Equal(&BlankCell)
	if resetPen {
		s.updatePen(nil)
	}

	// Reset wrap around (phantom cursor) state
	if s.atPhantom {
		s.cur.X = 0
		s.buf.WriteByte('\r') //nolint:errcheck
		s.atPhantom = false   // reset phantom cell state
	}

	// TODO: Investigate if we need to handle this case and/or if we need the
	// following code.
	//
	// if width > 0 && s.cur.X >= width {
	// 	l := (s.cur.X + 1) / width
	//
	// 	s.cur.Y += l
	// 	if height > 0 && s.cur.Y >= height {
	// 		l -= s.cur.Y - height - 1
	// 	}
	//
	// 	if l > 0 {
	// 		s.cur.X = 0
	// 		s.buf.WriteString("\r" + strings.Repeat("\n", l)) //nolint:errcheck
	// 	}
	// }

	if height > 0 {
		if s.cur.Y > height-1 {
			s.cur.Y = height - 1
		}
		if y > height-1 {
			y = height - 1
		}
	}

	if x == s.cur.X && y == s.cur.Y {
		// We give up later because we need to run checks for the phantom cell
		// and others before we can determine if we can give up.
		return
	}

	// We set the new cursor in [Screen.moveCursor].
	s.moveCursor(x, y, true) // Overwrite cells if possible
}

// Cursor represents a terminal Cursor.
type Cursor struct {
	Style
	Link
	Position
}

// ScreenOptions are options for the screen.
type ScreenOptions struct {
	// Term is the terminal type to use when writing to the screen. When empty,
	// `$TERM` is used from [os.Getenv].
	Term string
	// Profile is the color profile to use when writing to the screen.
	Profile colorprofile.Profile
	// RelativeCursor is whether to use relative cursor movements. This is
	// useful when alt-screen is not used or when using inline mode.
	RelativeCursor bool
	// AltScreen is whether to use the alternate screen buffer.
	AltScreen bool
	// ShowCursor is whether to show the cursor.
	ShowCursor bool
	// HardTabs is whether to use hard tabs to optimize cursor movements.
	HardTabs bool
	// Backspace is whether to use backspace characters to move the cursor.
	Backspace bool
}

// lineData represents the metadata for a line.
type lineData struct {
	// first and last changed cell indices
	firstCell, lastCell int
	// old index used for scrolling
	oldIndex int //nolint:unused
}

// Screen represents the terminal screen.
type Screen struct {
	w                io.Writer
	buf              *bytes.Buffer // buffer for writing to the screen
	curbuf           *Buffer       // the current buffer
	newbuf           *Buffer       // the new buffer
	tabs             *TabStops
	touch            map[int]lineData
	queueAbove       []string  // the queue of strings to write above the screen
	oldhash, newhash []uint64  // the old and new hash values for each line
	hashtab          []hashmap // the hashmap table
	oldnum           []int     // old indices from previous hash
	cur, saved       Cursor    // the current and saved cursors
	opts             ScreenOptions
	mu               sync.Mutex
	method           ansi.Method
	scrollHeight     int  // keeps track of how many lines we've scrolled down (inline mode)
	altScreenMode    bool // whether alternate screen mode is enabled
	cursorHidden     bool // whether text cursor mode is enabled
	clear            bool // whether to force clear the screen
	xtermLike        bool // whether to use xterm-like optimizations, otherwise, it uses vt100 only
	queuedText       bool // whether we have queued non-zero width text queued up
	atPhantom        bool // whether the cursor is out of bounds and at a phantom cell
}

// SetMethod sets the method used to calculate the width of cells.
func (s *Screen) SetMethod(method ansi.Method) {
	s.method = method
}

// UseBackspaces sets whether to use backspace characters to move the cursor.
func (s *Screen) UseBackspaces(v bool) {
	s.opts.Backspace = v
}

// UseHardTabs sets whether to use hard tabs to optimize cursor movements.
func (s *Screen) UseHardTabs(v bool) {
	s.opts.HardTabs = v
}

// SetColorProfile sets the color profile to use when writing to the screen.
func (s *Screen) SetColorProfile(p colorprofile.Profile) {
	s.opts.Profile = p
}

// SetRelativeCursor sets whether to use relative cursor movements.
func (s *Screen) SetRelativeCursor(v bool) {
	s.opts.RelativeCursor = v
}

// EnterAltScreen enters the alternate screen buffer.
func (s *Screen) EnterAltScreen() {
	s.opts.AltScreen = true
	s.clear = true
	s.saved = s.cur
}

// ExitAltScreen exits the alternate screen buffer.
func (s *Screen) ExitAltScreen() {
	s.opts.AltScreen = false
	s.clear = true
	s.cur = s.saved
}

// ShowCursor shows the cursor.
func (s *Screen) ShowCursor() {
	s.opts.ShowCursor = true
}

// HideCursor hides the cursor.
func (s *Screen) HideCursor() {
	s.opts.ShowCursor = false
}

// Bounds implements Window.
func (s *Screen) Bounds() Rectangle {
	// Always return the new buffer bounds.
	return s.newbuf.Bounds()
}

// Cell implements Window.
func (s *Screen) Cell(x int, y int) *Cell {
	return s.newbuf.Cell(x, y)
}

// Redraw forces a full redraw of the screen.
func (s *Screen) Redraw() {
	s.mu.Lock()
	s.clear = true
	s.mu.Unlock()
}

// Clear clears the screen with blank cells. This is a convenience method for
// [Screen.Fill] with a nil cell.
func (s *Screen) Clear() bool {
	return s.ClearRect(s.newbuf.Bounds())
}

// ClearRect clears the given rectangle with blank cells. This is a convenience
// method for [Screen.FillRect] with a nil cell.
func (s *Screen) ClearRect(r Rectangle) bool {
	return s.FillRect(nil, r)
}

// SetCell implements Window.
func (s *Screen) SetCell(x int, y int, cell *Cell) (v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cellWidth := 1
	if cell != nil {
		cellWidth = cell.Width
	}
	if prev := s.curbuf.Cell(x, y); !cellEqual(prev, cell) {
		chg, ok := s.touch[y]
		if !ok {
			chg = lineData{firstCell: x, lastCell: x + cellWidth}
		} else {
			chg.firstCell = min(chg.firstCell, x)
			chg.lastCell = max(chg.lastCell, x+cellWidth)
		}
		s.touch[y] = chg
	}

	return s.newbuf.SetCell(x, y, cell)
}

// Fill implements Window.
func (s *Screen) Fill(cell *Cell) bool {
	return s.FillRect(cell, s.newbuf.Bounds())
}

// FillRect implements Window.
func (s *Screen) FillRect(cell *Cell, r Rectangle) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.newbuf.FillRect(cell, r)
	for i := r.Min.Y; i < r.Max.Y; i++ {
		s.touch[i] = lineData{firstCell: r.Min.X, lastCell: r.Max.X}
	}
	return true
}

// isXtermLike returns whether the terminal is xterm-like. This means that the
// terminal supports ECMA-48 and ANSI X3.64 escape sequences.
// TODO: Should this be a lookup table into each $TERM terminfo database? Like
// we could keep a map of ANSI escape sequence to terminfo capability name and
// check if the database supports the escape sequence. Instead of keeping a
// list of terminal names here.
func isXtermLike(termtype string) (v bool) {
	parts := strings.Split(termtype, "-")
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case
		"alacritty",
		"contour",
		"foot",
		"ghostty",
		"kitty",
		"linux",
		"rio",
		"screen",
		"st",
		"tmux",
		"wezterm",
		"xterm":
		v = true
	}

	return
}

// NewScreen creates a new Screen.
func NewScreen(w io.Writer, width, height int, opts *ScreenOptions) (s *Screen) {
	s = new(Screen)
	s.w = w
	if opts != nil {
		s.opts = *opts
	}

	if s.opts.Term == "" {
		s.opts.Term = os.Getenv("TERM")
	}

	if width <= 0 || height <= 0 {
		if f, ok := w.(term.File); ok {
			width, height, _ = term.GetSize(f.Fd())
		}
	}
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}

	s.buf = new(bytes.Buffer)
	s.xtermLike = isXtermLike(s.opts.Term)
	s.curbuf = NewBuffer(width, height)
	s.newbuf = NewBuffer(width, height)
	s.cur = Cursor{Position: Pos(-1, -1)} // start at -1 to force a move
	s.saved = s.cur
	s.reset()

	return
}

// Width returns the width of the screen.
func (s *Screen) Width() int {
	return s.newbuf.Width()
}

// Height returns the height of the screen.
func (s *Screen) Height() int {
	return s.newbuf.Height()
}

// cellEqual returns whether the two cells are equal. A nil cell is considered
// a [BlankCell].
func cellEqual(a, b *Cell) bool {
	if a == b {
		return true
	}
	if a == nil {
		a = &BlankCell
	}
	if b == nil {
		b = &BlankCell
	}
	return a.Equal(b)
}

// putCell draws a cell at the current cursor position.
func (s *Screen) putCell(cell *Cell) {
	width, height := s.newbuf.Width(), s.newbuf.Height()
	if s.opts.AltScreen && s.cur.X == width-1 && s.cur.Y == height-1 {
		s.putCellLR(cell)
	} else {
		s.putAttrCell(cell)
	}
}

// wrapCursor wraps the cursor to the next line.
//
//nolint:unused
func (s *Screen) wrapCursor() {
	const autoRightMargin = true
	if autoRightMargin {
		// Assume we have auto wrap mode enabled.
		s.cur.X = 0
		s.cur.Y++
	} else {
		s.cur.X--
	}
}

func (s *Screen) putAttrCell(cell *Cell) {
	if cell != nil && cell.Empty() {
		// XXX: Zero width cells are special and should not be written to the
		// screen no matter what other attributes they have.
		// Zero width cells are used for wide characters that are split into
		// multiple cells.
		return
	}

	if cell == nil {
		cell = s.clearBlank()
	}

	// We're at pending wrap state (phantom cell), incoming cell should
	// wrap.
	if s.atPhantom {
		s.wrapCursor()
		s.atPhantom = false
	}

	s.updatePen(cell)
	s.buf.WriteRune(cell.Rune) //nolint:errcheck
	for _, c := range cell.Comb {
		s.buf.WriteRune(c) //nolint:errcheck
	}

	s.cur.X += cell.Width

	if cell.Width > 0 {
		s.queuedText = true
	}

	if s.cur.X >= s.newbuf.Width() {
		s.atPhantom = true
	}
}

// putCellLR draws a cell at the lower right corner of the screen.
func (s *Screen) putCellLR(cell *Cell) {
	// Optimize for the lower right corner cell.
	curX := s.cur.X
	if cell == nil || !cell.Empty() {
		s.buf.WriteString(ansi.ResetAutoWrapMode) //nolint:errcheck
		s.putAttrCell(cell)
		// Writing to lower-right corner cell should not wrap.
		s.atPhantom = false
		s.cur.X = curX
		s.buf.WriteString(ansi.SetAutoWrapMode) //nolint:errcheck
	}
}

// updatePen updates the cursor pen styles.
func (s *Screen) updatePen(cell *Cell) {
	if cell == nil {
		cell = &BlankCell
	}

	if s.opts.Profile != 0 {
		// Downsample colors to the given color profile.
		cell.Style = ConvertStyle(cell.Style, s.opts.Profile)
		cell.Link = ConvertLink(cell.Link, s.opts.Profile)
	}

	if !cell.Style.Equal(&s.cur.Style) {
		seq := cell.Style.DiffSequence(s.cur.Style)
		if cell.Style.Empty() && len(seq) > len(ansi.ResetStyle) {
			seq = ansi.ResetStyle
		}
		s.buf.WriteString(seq) //nolint:errcheck
		s.cur.Style = cell.Style
	}
	if !cell.Link.Equal(&s.cur.Link) {
		s.buf.WriteString(ansi.SetHyperlink(cell.Link.URL, cell.Link.Params)) //nolint:errcheck
		s.cur.Link = cell.Link
	}
}

// emitRange emits a range of cells to the buffer. It it equivalent to calling
// [Screen.putCell] for each cell in the range. This is optimized to use
// [ansi.ECH] and [ansi.REP].
// Returns whether the cursor is at the end of interval or somewhere in the
// middle.
func (s *Screen) emitRange(line Line, n int) (eoi bool) {
	for n > 0 {
		var count int
		for n > 1 && !cellEqual(line.At(0), line.At(1)) {
			s.putCell(line.At(0))
			line = line[1:]
			n--
		}

		cell0 := line[0]
		if n == 1 {
			s.putCell(cell0)
			return false
		}

		count = 2
		for count < n && cellEqual(line.At(count), cell0) {
			count++
		}

		ech := ansi.EraseCharacter(count)
		cup := ansi.CursorPosition(s.cur.X+count, s.cur.Y)
		rep := ansi.RepeatPreviousCharacter(count)
		if s.xtermLike && count > len(ech)+len(cup) && cell0 != nil && cell0.Clear() {
			s.updatePen(cell0)
			s.buf.WriteString(ech) //nolint:errcheck

			// If this is the last cell, we don't need to move the cursor.
			if count < n {
				s.move(s.cur.X+count, s.cur.Y)
			} else {
				return true // cursor in the middle
			}
		} else if s.xtermLike && count > len(rep) &&
			(cell0 == nil || (len(cell0.Comb) == 0 && cell0.Rune < 256)) {
			// We only support ASCII characters. Most terminals will handle
			// non-ASCII characters correctly, but some might not, ahem xterm.
			//
			// NOTE: [ansi.REP] only repeats the last rune and won't work
			// if the last cell contains multiple runes.

			wrapPossible := s.cur.X+count >= s.newbuf.Width()
			repCount := count
			if wrapPossible {
				repCount--
			}

			s.updatePen(cell0)
			s.putCell(cell0)
			repCount-- // cell0 is a single width cell ASCII character

			s.buf.WriteString(ansi.RepeatPreviousCharacter(repCount)) //nolint:errcheck
			s.cur.X += repCount
			if wrapPossible {
				s.putCell(cell0)
			}
		} else {
			for i := 0; i < count; i++ {
				s.putCell(line.At(i))
			}
		}

		line = line[clamp(count, 0, len(line)):]
		n -= count
	}

	return
}

// putRange puts a range of cells from the old line to the new line.
// Returns whether the cursor is at the end of interval or somewhere in the
// middle.
func (s *Screen) putRange(oldLine, newLine Line, y, start, end int) (eoi bool) {
	inline := min(len(ansi.CursorPosition(start+1, y+1)),
		min(len(ansi.HorizontalPositionAbsolute(start+1)),
			len(ansi.CursorForward(start+1))))
	if (end - start + 1) > inline {
		var j, same int
		for j, same = start, 0; j <= end; j++ {
			oldCell, newCell := oldLine.At(j), newLine.At(j)
			if same == 0 && oldCell != nil && oldCell.Empty() {
				continue
			}
			if cellEqual(oldCell, newCell) {
				same++
			} else {
				if same > end-start {
					s.emitRange(newLine[start:], j-same-start)
					s.move(j, y)
					start = j
				}
				same = 0
			}
		}

		i := s.emitRange(newLine[start:], j-same-start)

		// Always return 1 for the next [Screen.move] after a [Screen.putRange] if
		// we found identical characters at end of interval.
		if same == 0 {
			return i
		}
		return true
	}

	return s.emitRange(newLine[start:], end-start+1)
}

// clearToEnd clears the screen from the current cursor position to the end of
// line.
func (s *Screen) clearToEnd(blank *Cell, force bool) { //nolint:unparam
	if s.cur.Y >= 0 {
		curline := s.curbuf.Line(s.cur.Y)
		for j := s.cur.X; j < s.curbuf.Width(); j++ {
			if j >= 0 {
				c := curline.At(j)
				if !cellEqual(c, blank) {
					curline.Set(j, blank)
					force = true
				}
			}
		}
	}

	if force {
		s.updatePen(blank)
		count := s.newbuf.Width() - s.cur.X
		if s.el0Cost() <= count {
			s.buf.WriteString(ansi.EraseLineRight) //nolint:errcheck
		} else {
			for i := 0; i < count; i++ {
				s.putCell(blank)
			}
		}
	}
}

// clearBlank returns a blank cell based on the current cursor background color.
func (s *Screen) clearBlank() *Cell {
	c := BlankCell
	if !s.cur.Style.Empty() || !s.cur.Link.Empty() {
		c.Style = s.cur.Style
		c.Link = s.cur.Link
	}
	return &c
}

// insertCells inserts the count cells pointed by the given line at the current
// cursor position.
func (s *Screen) insertCells(line Line, count int) {
	if s.xtermLike {
		// Use [ansi.ICH] as an optimization.
		s.buf.WriteString(ansi.InsertCharacter(count)) //nolint:errcheck
	} else {
		// Otherwise, use [ansi.IRM] mode.
		s.buf.WriteString(ansi.SetInsertReplaceMode) //nolint:errcheck
	}

	for i := 0; count > 0; i++ {
		s.putAttrCell(line[i])
		count--
	}

	if !s.xtermLike {
		s.buf.WriteString(ansi.ResetInsertReplaceMode) //nolint:errcheck
	}
}

// el0Cost returns the cost of using [ansi.EL] 0 i.e. [ansi.EraseLineRight]. If
// this terminal supports background color erase, it can be cheaper to use
// [ansi.EL] 0 i.e. [ansi.EraseLineRight] to clear
// trailing spaces.
func (s *Screen) el0Cost() int {
	if s.xtermLike {
		return 0
	}
	return len(ansi.EraseLineRight)
}

// transformLine transforms the given line in the current window to the
// corresponding line in the new window. It uses [ansi.ICH] and [ansi.DCH] to
// insert or delete characters.
func (s *Screen) transformLine(y int) {
	var firstCell, oLastCell, nLastCell int // first, old last, new last index
	oldLine := s.curbuf.Line(y)
	newLine := s.newbuf.Line(y)

	// Find the first changed cell in the line
	var lineChanged bool
	for i := 0; i < s.newbuf.Width(); i++ {
		if !cellEqual(newLine.At(i), oldLine.At(i)) {
			lineChanged = true
			break
		}
	}

	const ceolStandoutGlitch = false
	if ceolStandoutGlitch && lineChanged {
		s.move(0, y)
		s.clearToEnd(nil, false)
		s.putRange(oldLine, newLine, y, 0, s.newbuf.Width()-1)
	} else {
		blank := newLine.At(0)

		// It might be cheaper to clear leading spaces with [ansi.EL] 1 i.e.
		// [ansi.EraseLineLeft].
		if blank == nil || blank.Clear() {
			var oFirstCell, nFirstCell int
			for oFirstCell = 0; oFirstCell < s.curbuf.Width(); oFirstCell++ {
				if !cellEqual(oldLine.At(oFirstCell), blank) {
					break
				}
			}
			for nFirstCell = 0; nFirstCell < s.newbuf.Width(); nFirstCell++ {
				if !cellEqual(newLine.At(nFirstCell), blank) {
					break
				}
			}

			if nFirstCell == oFirstCell {
				firstCell = nFirstCell

				// Find the first differing cell
				for firstCell < s.newbuf.Width() &&
					cellEqual(oldLine.At(firstCell), newLine.At(firstCell)) {
					firstCell++
				}
			} else if oFirstCell > nFirstCell {
				firstCell = nFirstCell
			} else if oFirstCell < nFirstCell {
				firstCell = oFirstCell
				el1Cost := len(ansi.EraseLineLeft)
				if el1Cost < nFirstCell-oFirstCell {
					if nFirstCell >= s.newbuf.Width() {
						s.move(0, y)
						s.updatePen(blank)
						s.buf.WriteString(ansi.EraseLineRight) //nolint:errcheck
					} else {
						s.move(nFirstCell-1, y)
						s.updatePen(blank)
						s.buf.WriteString(ansi.EraseLineLeft) //nolint:errcheck
					}

					for firstCell < nFirstCell {
						oldLine.Set(firstCell, blank)
						firstCell++
					}
				}
			}
		} else {
			// Find the first differing cell
			for firstCell < s.newbuf.Width() && cellEqual(newLine.At(firstCell), oldLine.At(firstCell)) {
				firstCell++
			}
		}

		// If we didn't find one, we're done
		if firstCell >= s.newbuf.Width() {
			return
		}

		blank = newLine.At(s.newbuf.Width() - 1)
		if blank != nil && !blank.Clear() {
			// Find the last differing cell
			nLastCell = s.newbuf.Width() - 1
			for nLastCell > firstCell && cellEqual(newLine.At(nLastCell), oldLine.At(nLastCell)) {
				nLastCell--
			}

			if nLastCell >= firstCell {
				s.move(firstCell, y)
				s.putRange(oldLine, newLine, y, firstCell, nLastCell)
				if firstCell < len(oldLine) && firstCell < len(newLine) {
					copy(oldLine[firstCell:], newLine[firstCell:])
				} else {
					copy(oldLine, newLine)
				}
			}

			return
		}

		// Find last non-blank cell in the old line.
		oLastCell = s.curbuf.Width() - 1
		for oLastCell > firstCell && cellEqual(oldLine.At(oLastCell), blank) {
			oLastCell--
		}

		// Find last non-blank cell in the new line.
		nLastCell = s.newbuf.Width() - 1
		for nLastCell > firstCell && cellEqual(newLine.At(nLastCell), blank) {
			nLastCell--
		}

		if nLastCell == firstCell && s.el0Cost() < oLastCell-nLastCell {
			s.move(firstCell, y)
			if !cellEqual(newLine.At(firstCell), blank) {
				s.putCell(newLine.At(firstCell))
			}
			s.clearToEnd(blank, false)
		} else if nLastCell != oLastCell &&
			!cellEqual(newLine.At(nLastCell), oldLine.At(oLastCell)) {
			s.move(firstCell, y)
			if oLastCell-nLastCell > s.el0Cost() {
				if s.putRange(oldLine, newLine, y, firstCell, nLastCell) {
					s.move(nLastCell+1, y)
				}
				s.clearToEnd(blank, false)
			} else {
				n := max(nLastCell, oLastCell)
				s.putRange(oldLine, newLine, y, firstCell, n)
			}
		} else {
			nLastNonBlank := nLastCell
			oLastNonBlank := oLastCell

			// Find the last cells that really differ.
			// Can be -1 if no cells differ.
			for cellEqual(newLine.At(nLastCell), oldLine.At(oLastCell)) {
				if !cellEqual(newLine.At(nLastCell-1), oldLine.At(oLastCell-1)) {
					break
				}
				nLastCell--
				oLastCell--
				if nLastCell == -1 || oLastCell == -1 {
					break
				}
			}

			n := min(oLastCell, nLastCell)
			if n >= firstCell {
				s.move(firstCell, y)
				s.putRange(oldLine, newLine, y, firstCell, n)
			}

			if oLastCell < nLastCell {
				m := max(nLastNonBlank, oLastNonBlank)
				if n != 0 {
					for n > 0 {
						wide := newLine.At(n + 1)
						if wide == nil || !wide.Empty() {
							break
						}
						n--
						oLastCell--
					}
				} else if n >= firstCell && newLine.At(n) != nil && newLine.At(n).Width > 1 {
					next := newLine.At(n + 1)
					for next != nil && next.Empty() {
						n++
						oLastCell++
					}
				}

				s.move(n+1, y)
				ichCost := 3 + nLastCell - oLastCell
				if s.xtermLike && (nLastCell < nLastNonBlank || ichCost > (m-n)) {
					s.putRange(oldLine, newLine, y, n+1, m)
				} else {
					s.insertCells(newLine[n+1:], nLastCell-oLastCell)
				}
			} else if oLastCell > nLastCell {
				s.move(n+1, y)
				dchCost := 3 + oLastCell - nLastCell
				if dchCost > len(ansi.EraseLineRight)+nLastNonBlank-(n+1) {
					if s.putRange(oldLine, newLine, y, n+1, nLastNonBlank) {
						s.move(nLastNonBlank+1, y)
					}
					s.clearToEnd(blank, false)
				} else {
					s.updatePen(blank)
					s.deleteCells(oLastCell - nLastCell)
				}
			}
		}
	}

	// Update the old line with the new line
	if firstCell < len(oldLine) && firstCell < len(newLine) {
		copy(oldLine[firstCell:], newLine[firstCell:])
	} else {
		copy(oldLine, newLine)
	}
}

// deleteCells deletes the count cells at the current cursor position and moves
// the rest of the line to the left. This is equivalent to [ansi.DCH].
func (s *Screen) deleteCells(count int) {
	// [ansi.DCH] will shift in cells from the right margin so we need to
	// ensure that they are the right style.
	s.buf.WriteString(ansi.DeleteCharacter(count)) //nolint:errcheck
}

// clearToBottom clears the screen from the current cursor position to the end
// of the screen.
func (s *Screen) clearToBottom(blank *Cell) {
	row, col := s.cur.Y, s.cur.X
	if row < 0 {
		row = 0
	}

	s.updatePen(blank)
	s.buf.WriteString(ansi.EraseScreenBelow) //nolint:errcheck
	// Clear the rest of the current line
	s.curbuf.ClearRect(Rect(col, row, s.curbuf.Width()-col, 1))
	// Clear everything below the current line
	s.curbuf.ClearRect(Rect(0, row+1, s.curbuf.Width(), s.curbuf.Height()-row-1))
}

// clearBottom tests if clearing the end of the screen would satisfy part of
// the screen update. Scan backwards through lines in the screen checking if
// each is blank and one or more are changed.
// It returns the top line.
func (s *Screen) clearBottom(total int) (top int) {
	if total <= 0 {
		return
	}

	top = total
	last := s.newbuf.Width()
	blank := s.clearBlank()
	canClearWithBlank := blank == nil || blank.Clear()

	if canClearWithBlank {
		var row int
		for row = total - 1; row >= 0; row-- {
			oldLine := s.curbuf.Line(row)
			newLine := s.newbuf.Line(row)

			var col int
			ok := true
			for col = 0; ok && col < last; col++ {
				ok = cellEqual(newLine.At(col), blank)
			}
			if !ok {
				break
			}

			for col = 0; ok && col < last; col++ {
				ok = len(oldLine) == last && cellEqual(oldLine.At(col), blank)
			}
			if !ok {
				top = row
			}
		}

		if top < total {
			s.move(0, top-1) // top is 1-based
			s.clearToBottom(blank)
			if s.oldhash != nil && s.newhash != nil &&
				row < len(s.oldhash) && row < len(s.newhash) {
				for row := top; row < s.newbuf.Height(); row++ {
					s.oldhash[row] = s.newhash[row]
				}
			}
		}
	}

	return
}

// clearScreen clears the screen and put cursor at home.
func (s *Screen) clearScreen(blank *Cell) {
	s.updatePen(blank)
	s.buf.WriteString(ansi.CursorHomePosition) //nolint:errcheck
	s.buf.WriteString(ansi.EraseEntireScreen)  //nolint:errcheck
	s.cur.X, s.cur.Y = 0, 0
	s.curbuf.Fill(blank)
}

// clearBelow clears everything below and including the row.
func (s *Screen) clearBelow(blank *Cell, row int) {
	s.move(0, row)
	s.clearToBottom(blank)
}

// clearUpdate forces a screen redraw.
func (s *Screen) clearUpdate() {
	blank := s.clearBlank()
	var nonEmpty int
	if s.opts.AltScreen {
		// XXX: We're using the maximum height of the two buffers to ensure
		// we write newly added lines to the screen in [Screen.transformLine].
		nonEmpty = max(s.curbuf.Height(), s.newbuf.Height())
		s.clearScreen(blank)
	} else {
		nonEmpty = s.newbuf.Height()
		s.clearBelow(blank, 0)
	}
	nonEmpty = s.clearBottom(nonEmpty)
	for i := 0; i < nonEmpty; i++ {
		s.transformLine(i)
	}
}

// Flush flushes the buffer to the screen.
func (s *Screen) Flush() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flush()
}

func (s *Screen) flush() (err error) {
	// Write the buffer
	if s.buf.Len() > 0 {
		_, err = s.w.Write(s.buf.Bytes()) //nolint:errcheck
		if err == nil {
			s.buf.Reset()
		}
	}

	return
}

// Render renders changes of the screen to the internal buffer. Call
// [Screen.Flush] to flush pending changes to the screen.
func (s *Screen) Render() {
	s.mu.Lock()
	s.render()
	s.mu.Unlock()
}

func (s *Screen) render() {
	// Do we need to render anything?
	if s.opts.AltScreen == s.altScreenMode &&
		!s.opts.ShowCursor == s.cursorHidden &&
		!s.clear &&
		len(s.touch) == 0 &&
		len(s.queueAbove) == 0 {
		return
	}

	// TODO: Investigate whether this is necessary. Theoretically, terminals
	// can add/remove tab stops and we should be able to handle that. We could
	// use [ansi.DECTABSR] to read the tab stops, but that's not implemented in
	// most terminals :/
	// // Are we using hard tabs? If so, ensure tabs are using the
	// // default interval using [ansi.DECST8C].
	// if s.opts.HardTabs && !s.initTabs {
	// 	s.buf.WriteString(ansi.SetTabEvery8Columns)
	// 	s.initTabs = true
	// }

	// Do we need alt-screen mode?
	if s.opts.AltScreen != s.altScreenMode {
		if s.opts.AltScreen {
			s.buf.WriteString(ansi.SetAltScreenSaveCursorMode)
		} else {
			s.buf.WriteString(ansi.ResetAltScreenSaveCursorMode)
		}
		s.altScreenMode = s.opts.AltScreen
	}

	// Do we need text cursor mode?
	if !s.opts.ShowCursor != s.cursorHidden {
		s.cursorHidden = !s.opts.ShowCursor
		if s.cursorHidden {
			s.buf.WriteString(ansi.HideCursor)
		}
	}

	// Do we have queued strings to write above the screen?
	if len(s.queueAbove) > 0 {
		// TODO: Use scrolling region if available.
		// TODO: Use [Screen.Write] [io.Writer] interface.

		// We need to scroll the screen up by the number of lines in the queue.
		// We can't use [ansi.SU] because we want the cursor to move down until
		// it reaches the bottom of the screen.
		s.move(0, s.newbuf.Height()-1)
		s.buf.WriteString(strings.Repeat("\n", len(s.queueAbove)))
		s.cur.Y += len(s.queueAbove)
		// XXX: Now go to the top of the screen, insert new lines, and write
		// the queued strings. It is important to use [Screen.moveCursor]
		// instead of [Screen.move] because we don't want to perform any checks
		// on the cursor position.
		s.moveCursor(0, 0, false)
		s.buf.WriteString(ansi.InsertLine(len(s.queueAbove)))
		for _, line := range s.queueAbove {
			s.buf.WriteString(line + "\r\n")
		}

		// Clear the queue
		s.queueAbove = s.queueAbove[:0]
	}

	var nonEmpty int

	// XXX: In inline mode, after a screen resize, we need to clear the extra
	// lines at the bottom of the screen. This is because in inline mode, we
	// don't use the full screen height and the current buffer size might be
	// larger than the new buffer size.
	partialClear := !s.opts.AltScreen && s.cur.X != -1 && s.cur.Y != -1 &&
		s.curbuf.Width() == s.newbuf.Width() &&
		s.curbuf.Height() > 0 &&
		s.curbuf.Height() > s.newbuf.Height()

	if !s.clear && partialClear {
		s.clearBelow(nil, s.newbuf.Height()-1)
	}

	if s.clear {
		s.clearUpdate()
		s.clear = false
	} else if len(s.touch) > 0 {
		if s.opts.AltScreen {
			// Optimize scrolling for the alternate screen buffer.
			// TODO: Should we optimize for inline mode as well? If so, we need
			// to know the actual cursor position to use [ansi.DECSTBM].
			s.scrollOptimize()
		}

		var changedLines int
		var i int

		if s.opts.AltScreen {
			nonEmpty = min(s.curbuf.Height(), s.newbuf.Height())
		} else {
			nonEmpty = s.newbuf.Height()
		}

		nonEmpty = s.clearBottom(nonEmpty)
		for i = 0; i < nonEmpty; i++ {
			_, ok := s.touch[i]
			if ok {
				s.transformLine(i)
				changedLines++
			}
		}
	}

	// Sync windows and screen
	s.touch = make(map[int]lineData, s.newbuf.Height())

	if s.curbuf.Width() != s.newbuf.Width() || s.curbuf.Height() != s.newbuf.Height() {
		// Resize the old buffer to match the new buffer.
		_, oldh := s.curbuf.Width(), s.curbuf.Height()
		s.curbuf.Resize(s.newbuf.Width(), s.newbuf.Height())
		// Sync new lines to old lines
		for i := oldh - 1; i < s.newbuf.Height(); i++ {
			copy(s.curbuf.Line(i), s.newbuf.Line(i))
		}
	}

	s.updatePen(nil) // nil indicates a blank cell with no styles

	// Do we have enough changes to justify toggling the cursor?
	if s.buf.Len() > 1 && s.opts.ShowCursor && !s.cursorHidden && s.queuedText {
		nb := new(bytes.Buffer)
		nb.Grow(s.buf.Len() + len(ansi.HideCursor) + len(ansi.ShowCursor))
		nb.WriteString(ansi.HideCursor)
		nb.Write(s.buf.Bytes())
		nb.WriteString(ansi.ShowCursor)
		*s.buf = *nb
	}

	s.queuedText = false
}

// Close writes the final screen update and resets the screen.
func (s *Screen) Close() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.render()
	s.updatePen(nil)
	// Go to the bottom of the screen
	s.move(0, s.newbuf.Height()-1)

	if s.altScreenMode {
		s.buf.WriteString(ansi.ResetAltScreenSaveCursorMode)
		s.altScreenMode = false
	}

	if s.cursorHidden {
		s.buf.WriteString(ansi.ShowCursor)
		s.cursorHidden = false
	}

	// Write the buffer
	err = s.flush()
	if err != nil {
		return
	}

	s.reset()
	return
}

// reset resets the screen to its initial state.
func (s *Screen) reset() {
	s.scrollHeight = 0
	s.cursorHidden = false
	s.altScreenMode = false
	s.touch = make(map[int]lineData, s.newbuf.Height())
	if s.curbuf != nil {
		s.curbuf.Clear()
	}
	if s.newbuf != nil {
		s.newbuf.Clear()
	}
	s.buf.Reset()
	s.tabs = DefaultTabStops(s.newbuf.Width())
	s.oldhash, s.newhash = nil, nil

	// We always disable HardTabs when termtype is "linux".
	if strings.HasPrefix(s.opts.Term, "linux") {
		s.opts.HardTabs = false
	}
}

// Resize resizes the screen.
func (s *Screen) Resize(width, height int) bool {
	oldw := s.newbuf.Width()
	oldh := s.newbuf.Height()

	if s.opts.AltScreen || width != oldw {
		// We only clear the whole screen if the width changes. Adding/removing
		// rows is handled by the [Screen.render] and [Screen.transformLine]
		// methods.
		s.clear = true
	}

	// Clear new columns and lines
	if width > oldh {
		s.ClearRect(Rect(max(oldw-1, 0), 0, width-oldw, height))
	} else if width < oldw {
		s.ClearRect(Rect(max(width-1, 0), 0, oldw-width, height))
	}

	if height > oldh {
		s.ClearRect(Rect(0, max(oldh-1, 0), width, height-oldh))
	} else if height < oldh {
		s.ClearRect(Rect(0, max(height-1, 0), width, oldh-height))
	}

	s.mu.Lock()
	s.newbuf.Resize(width, height)
	s.tabs.Resize(width)
	s.oldhash, s.newhash = nil, nil
	s.scrollHeight = 0 // reset scroll lines
	s.mu.Unlock()

	return true
}

// MoveTo moves the cursor to the given position.
func (s *Screen) MoveTo(x, y int) {
	s.mu.Lock()
	s.move(x, y)
	s.mu.Unlock()
}

// InsertAbove inserts string above the screen. The inserted string is not
// managed by the screen. This does nothing when alternate screen mode is
// enabled.
func (s *Screen) InsertAbove(str string) {
	if s.opts.AltScreen {
		return
	}
	s.mu.Lock()
	for _, line := range strings.Split(str, "\n") {
		s.queueAbove = append(s.queueAbove, s.method.Truncate(line, s.Width(), ""))
	}
	s.mu.Unlock()
}
