package cellbuf

import (
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

// NewCell returns a new cell. This is a convenience function that initializes a
// new cell with the given content. The cell's width is determined by the
// content using [runewidth.RuneWidth].
// This will only account for the first combined rune in the content. If the
// content is empty, it will return an empty cell with a width of 0.
func NewCell(r rune, comb ...rune) (c *Cell) {
	c = new(Cell)
	c.Rune = r
	c.Width = runewidth.RuneWidth(r)
	for _, r := range comb {
		if runewidth.RuneWidth(r) > 0 {
			break
		}
		c.Comb = append(c.Comb, r)
	}
	c.Comb = comb
	c.Width = runewidth.StringWidth(string(append([]rune{r}, comb...)))
	return
}

// NewCellString returns a new cell with the given string content. This is a
// convenience function that initializes a new cell with the given content. The
// cell's width is determined by the content using [runewidth.StringWidth].
// This will only use the first combined rune in the string. If the string is
// empty, it will return an empty cell with a width of 0.
func NewCellString(s string) (c *Cell) {
	c = new(Cell)
	for i, r := range s {
		if i == 0 {
			c.Rune = r
			// We only care about the first rune's width
			c.Width = runewidth.RuneWidth(r)
		} else {
			if runewidth.RuneWidth(r) > 0 {
				break
			}
			c.Comb = append(c.Comb, r)
		}
	}
	return
}

// NewGraphemeCell returns a new cell. This is a convenience function that
// initializes a new cell with the given content. The cell's width is determined
// by the content using [uniseg.FirstGraphemeClusterInString].
// This is used when the content is a grapheme cluster i.e. a sequence of runes
// that form a single visual unit.
// This will only return the first grapheme cluster in the string. If the
// string is empty, it will return an empty cell with a width of 0.
func NewGraphemeCell(s string) (c *Cell) {
	g, _, w, _ := uniseg.FirstGraphemeClusterInString(s, -1)
	return newGraphemeCell(g, w)
}

func newGraphemeCell(s string, w int) (c *Cell) {
	c = new(Cell)
	c.Width = w
	for i, r := range s {
		if i == 0 {
			c.Rune = r
		} else {
			c.Comb = append(c.Comb, r)
		}
	}
	return
}

// Line represents a line in the terminal.
// A nil cell represents an blank cell, a cell with a space character and a
// width of 1.
// If a cell has no content and a width of 0, it is a placeholder for a wide
// cell.
type Line []*Cell

// Width returns the width of the line.
func (l Line) Width() int {
	return len(l)
}

// Len returns the length of the line.
func (l Line) Len() int {
	return len(l)
}

// String returns the string representation of the line. Any trailing spaces
// are removed.
func (l Line) String() (s string) {
	for _, c := range l {
		if c == nil {
			s += " "
		} else if c.Empty() {
			continue
		} else {
			s += c.String()
		}
	}
	s = strings.TrimRight(s, " ")
	return
}

// At returns the cell at the given x position.
// If the cell does not exist, it returns nil.
func (l Line) At(x int) *Cell {
	if x < 0 || x >= len(l) {
		return nil
	}

	c := l[x]
	if c == nil {
		newCell := BlankCell
		return &newCell
	}

	return c
}

// Set sets the cell at the given x position. If a wide cell is given, it will
// set the cell and the following cells to [EmptyCell]. It returns true if the
// cell was set.
func (l Line) Set(x int, c *Cell) bool {
	return l.set(x, c, true)
}

func (l Line) set(x int, c *Cell, clone bool) bool {
	width := l.Width()
	if x < 0 || x >= width {
		return false
	}

	// When a wide cell is partially overwritten, we need
	// to fill the rest of the cell with space cells to
	// avoid rendering issues.
	prev := l.At(x)
	if prev != nil && prev.Width > 1 {
		// Writing to the first wide cell
		for j := 0; j < prev.Width && x+j < l.Width(); j++ {
			l[x+j] = prev.Clone().Blank()
		}
	} else if prev != nil && prev.Width == 0 {
		// Writing to wide cell placeholders
		for j := 1; j < maxCellWidth && x-j >= 0; j++ {
			wide := l.At(x - j)
			if wide != nil && wide.Width > 1 && j < wide.Width {
				for k := 0; k < wide.Width; k++ {
					l[x-j+k] = wide.Clone().Blank()
				}
				break
			}
		}
	}

	if clone && c != nil {
		// Clone the cell if not nil.
		c = c.Clone()
	}

	if c != nil && x+c.Width > width {
		// If the cell is too wide, we write blanks with the same style.
		for i := 0; i < c.Width && x+i < width; i++ {
			l[x+i] = c.Clone().Blank()
		}
	} else {
		l[x] = c

		// Mark wide cells with an empty cell zero width
		// We set the wide cell down below
		if c != nil && c.Width > 1 {
			for j := 1; j < c.Width && x+j < l.Width(); j++ {
				var wide Cell
				l[x+j] = &wide
			}
		}
	}

	return true
}

// Buffer is a 2D grid of cells representing a screen or terminal.
type Buffer struct {
	// Lines holds the lines of the buffer.
	Lines []Line
}

// NewBuffer creates a new buffer with the given width and height.
// This is a convenience function that initializes a new buffer and resizes it.
func NewBuffer(width int, height int) *Buffer {
	b := new(Buffer)
	b.Resize(width, height)
	return b
}

// String returns the string representation of the buffer.
func (b *Buffer) String() (s string) {
	for i, l := range b.Lines {
		s += l.String()
		if i < len(b.Lines)-1 {
			s += "\r\n"
		}
	}
	return
}

// Line returns a pointer to the line at the given y position.
// If the line does not exist, it returns nil.
func (b *Buffer) Line(y int) Line {
	if y < 0 || y >= len(b.Lines) {
		return nil
	}
	return b.Lines[y]
}

// Cell implements Screen.
func (b *Buffer) Cell(x int, y int) *Cell {
	if y < 0 || y >= len(b.Lines) {
		return nil
	}
	return b.Lines[y].At(x)
}

// maxCellWidth is the maximum width a terminal cell can get.
const maxCellWidth = 4

// SetCell sets the cell at the given x, y position.
func (b *Buffer) SetCell(x, y int, c *Cell) bool {
	return b.setCell(x, y, c, true)
}

// setCell sets the cell at the given x, y position. This will always clone and
// allocates a new cell if c is not nil.
func (b *Buffer) setCell(x, y int, c *Cell, clone bool) bool {
	if y < 0 || y >= len(b.Lines) {
		return false
	}
	return b.Lines[y].set(x, c, clone)
}

// Height implements Screen.
func (b *Buffer) Height() int {
	return len(b.Lines)
}

// Width implements Screen.
func (b *Buffer) Width() int {
	if len(b.Lines) == 0 {
		return 0
	}
	return b.Lines[0].Width()
}

// Bounds returns the bounds of the buffer.
func (b *Buffer) Bounds() Rectangle {
	return Rect(0, 0, b.Width(), b.Height())
}

// Resize resizes the buffer to the given width and height.
func (b *Buffer) Resize(width int, height int) {
	if width == 0 || height == 0 {
		b.Lines = nil
		return
	}

	if width > b.Width() {
		line := make(Line, width-b.Width())
		for i := range b.Lines {
			b.Lines[i] = append(b.Lines[i], line...)
		}
	} else if width < b.Width() {
		for i := range b.Lines {
			b.Lines[i] = b.Lines[i][:width]
		}
	}

	if height > len(b.Lines) {
		for i := len(b.Lines); i < height; i++ {
			b.Lines = append(b.Lines, make(Line, width))
		}
	} else if height < len(b.Lines) {
		b.Lines = b.Lines[:height]
	}
}

// FillRect fills the buffer with the given cell and rectangle.
func (b *Buffer) FillRect(c *Cell, rect Rectangle) {
	cellWidth := 1
	if c != nil && c.Width > 1 {
		cellWidth = c.Width
	}
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x += cellWidth {
			b.setCell(x, y, c, false) //nolint:errcheck
		}
	}
}

// Fill fills the buffer with the given cell and rectangle.
func (b *Buffer) Fill(c *Cell) {
	b.FillRect(c, b.Bounds())
}

// Clear clears the buffer with space cells and rectangle.
func (b *Buffer) Clear() {
	b.ClearRect(b.Bounds())
}

// ClearRect clears the buffer with space cells within the specified
// rectangles. Only cells within the rectangle's bounds are affected.
func (b *Buffer) ClearRect(rect Rectangle) {
	b.FillRect(nil, rect)
}

// InsertLine inserts n lines at the given line position, with the given
// optional cell, within the specified rectangles. If no rectangles are
// specified, it inserts lines in the entire buffer. Only cells within the
// rectangle's horizontal bounds are affected. Lines are pushed out of the
// rectangle bounds and lost. This follows terminal [ansi.IL] behavior.
// It returns the pushed out lines.
func (b *Buffer) InsertLine(y, n int, c *Cell) {
	b.InsertLineRect(y, n, c, b.Bounds())
}

// InsertLineRect inserts new lines at the given line position, with the
// given optional cell, within the rectangle bounds. Only cells within the
// rectangle's horizontal bounds are affected. Lines are pushed out of the
// rectangle bounds and lost. This follows terminal [ansi.IL] behavior.
func (b *Buffer) InsertLineRect(y, n int, c *Cell, rect Rectangle) {
	if n <= 0 || y < rect.Min.Y || y >= rect.Max.Y || y >= b.Height() {
		return
	}

	// Limit number of lines to insert to available space
	if y+n > rect.Max.Y {
		n = rect.Max.Y - y
	}

	// Move existing lines down within the bounds
	for i := rect.Max.Y - 1; i >= y+n; i-- {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			// We don't need to clone c here because we're just moving lines down.
			b.setCell(x, i, b.Lines[i-n][x], false)
		}
	}

	// Clear the newly inserted lines within bounds
	for i := y; i < y+n; i++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			b.setCell(x, i, c, true)
		}
	}
}

// DeleteLineRect deletes lines at the given line position, with the given
// optional cell, within the rectangle bounds. Only cells within the
// rectangle's bounds are affected. Lines are shifted up within the bounds and
// new blank lines are created at the bottom. This follows terminal [ansi.DL]
// behavior.
func (b *Buffer) DeleteLineRect(y, n int, c *Cell, rect Rectangle) {
	if n <= 0 || y < rect.Min.Y || y >= rect.Max.Y || y >= b.Height() {
		return
	}

	// Limit deletion count to available space in scroll region
	if n > rect.Max.Y-y {
		n = rect.Max.Y - y
	}

	// Shift cells up within the bounds
	for dst := y; dst < rect.Max.Y-n; dst++ {
		src := dst + n
		for x := rect.Min.X; x < rect.Max.X; x++ {
			// We don't need to clone c here because we're just moving cells up.
			// b.lines[dst][x] = b.lines[src][x]
			b.setCell(x, dst, b.Lines[src][x], false)
		}
	}

	// Fill the bottom n lines with blank cells
	for i := rect.Max.Y - n; i < rect.Max.Y; i++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			b.setCell(x, i, c, true)
		}
	}
}

// DeleteLine deletes n lines at the given line position, with the given
// optional cell, within the specified rectangles. If no rectangles are
// specified, it deletes lines in the entire buffer.
func (b *Buffer) DeleteLine(y, n int, c *Cell) {
	b.DeleteLineRect(y, n, c, b.Bounds())
}

// InsertCell inserts new cells at the given position, with the given optional
// cell, within the specified rectangles. If no rectangles are specified, it
// inserts cells in the entire buffer. This follows terminal [ansi.ICH]
// behavior.
func (b *Buffer) InsertCell(x, y, n int, c *Cell) {
	b.InsertCellRect(x, y, n, c, b.Bounds())
}

// InsertCellRect inserts new cells at the given position, with the given
// optional cell, within the rectangle bounds. Only cells within the
// rectangle's bounds are affected, following terminal [ansi.ICH] behavior.
func (b *Buffer) InsertCellRect(x, y, n int, c *Cell, rect Rectangle) {
	if n <= 0 || y < rect.Min.Y || y >= rect.Max.Y || y >= b.Height() ||
		x < rect.Min.X || x >= rect.Max.X || x >= b.Width() {
		return
	}

	// Limit number of cells to insert to available space
	if x+n > rect.Max.X {
		n = rect.Max.X - x
	}

	// Move existing cells within rectangle bounds to the right
	for i := rect.Max.X - 1; i >= x+n && i-n >= rect.Min.X; i-- {
		// We don't need to clone c here because we're just moving cells to the
		// right.
		// b.lines[y][i] = b.lines[y][i-n]
		b.setCell(i, y, b.Lines[y][i-n], false)
	}

	// Clear the newly inserted cells within rectangle bounds
	for i := x; i < x+n && i < rect.Max.X; i++ {
		b.setCell(i, y, c, true)
	}
}

// DeleteCell deletes cells at the given position, with the given optional
// cell, within the specified rectangles. If no rectangles are specified, it
// deletes cells in the entire buffer. This follows terminal [ansi.DCH]
// behavior.
func (b *Buffer) DeleteCell(x, y, n int, c *Cell) {
	b.DeleteCellRect(x, y, n, c, b.Bounds())
}

// DeleteCellRect deletes cells at the given position, with the given
// optional cell, within the rectangle bounds. Only cells within the
// rectangle's bounds are affected, following terminal [ansi.DCH] behavior.
func (b *Buffer) DeleteCellRect(x, y, n int, c *Cell, rect Rectangle) {
	if n <= 0 || y < rect.Min.Y || y >= rect.Max.Y || y >= b.Height() ||
		x < rect.Min.X || x >= rect.Max.X || x >= b.Width() {
		return
	}

	// Calculate how many positions we can actually delete
	remainingCells := rect.Max.X - x
	if n > remainingCells {
		n = remainingCells
	}

	// Shift the remaining cells to the left
	for i := x; i < rect.Max.X-n; i++ {
		if i+n < rect.Max.X {
			// We don't need to clone c here because we're just moving cells to
			// the left.
			// b.lines[y][i] = b.lines[y][i+n]
			b.setCell(i, y, b.Lines[y][i+n], false)
		}
	}

	// Fill the vacated positions with the given cell
	for i := rect.Max.X - n; i < rect.Max.X; i++ {
		b.setCell(i, y, c, true)
	}
}
