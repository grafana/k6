package cellbuf

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// CellBuffer is a cell buffer that represents a set of cells in a screen or a
// grid.
type CellBuffer interface {
	// Cell returns the cell at the given position.
	Cell(x, y int) *Cell
	// SetCell sets the cell at the given position to the given cell. It
	// returns whether the cell was set successfully.
	SetCell(x, y int, c *Cell) bool
	// Bounds returns the bounds of the cell buffer.
	Bounds() Rectangle
}

// FillRect fills the rectangle within the cell buffer with the given cell.
// This will not fill cells outside the bounds of the cell buffer.
func FillRect(s CellBuffer, c *Cell, rect Rectangle) {
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			s.SetCell(x, y, c) //nolint:errcheck
		}
	}
}

// Fill fills the cell buffer with the given cell.
func Fill(s CellBuffer, c *Cell) {
	FillRect(s, c, s.Bounds())
}

// ClearRect clears the rectangle within the cell buffer with blank cells.
func ClearRect(s CellBuffer, rect Rectangle) {
	FillRect(s, nil, rect)
}

// Clear clears the cell buffer with blank cells.
func Clear(s CellBuffer) {
	Fill(s, nil)
}

// SetContentRect clears the rectangle within the cell buffer with blank cells,
// and sets the given string as its content. If the height or width of the
// string exceeds the height or width of the cell buffer, it will be truncated.
func SetContentRect(s CellBuffer, str string, rect Rectangle) {
	// Replace all "\n" with "\r\n" to ensure the cursor is reset to the start
	// of the line. Make sure we don't replace "\r\n" with "\r\r\n".
	str = strings.ReplaceAll(str, "\r\n", "\n")
	str = strings.ReplaceAll(str, "\n", "\r\n")
	ClearRect(s, rect)
	printString(s, ansi.GraphemeWidth, rect.Min.X, rect.Min.Y, rect, str, true, "")
}

// SetContent clears the cell buffer with blank cells, and sets the given string
// as its content. If the height or width of the string exceeds the height or
// width of the cell buffer, it will be truncated.
func SetContent(s CellBuffer, str string) {
	SetContentRect(s, str, s.Bounds())
}

// Render returns a string representation of the grid with ANSI escape sequences.
func Render(d CellBuffer) string {
	var buf bytes.Buffer
	height := d.Bounds().Dy()
	for y := 0; y < height; y++ {
		_, line := RenderLine(d, y)
		buf.WriteString(line)
		if y < height-1 {
			buf.WriteString("\r\n")
		}
	}
	return buf.String()
}

// RenderLine returns a string representation of the yth line of the grid along
// with the width of the line.
func RenderLine(d CellBuffer, n int) (w int, line string) {
	var pen Style
	var link Link
	var buf bytes.Buffer
	var pendingLine string
	var pendingWidth int // this ignores space cells until we hit a non-space cell

	writePending := func() {
		// If there's no pending line, we don't need to do anything.
		if len(pendingLine) == 0 {
			return
		}
		buf.WriteString(pendingLine)
		w += pendingWidth
		pendingWidth = 0
		pendingLine = ""
	}

	for x := 0; x < d.Bounds().Dx(); x++ {
		if cell := d.Cell(x, n); cell != nil && cell.Width > 0 {
			// Convert the cell's style and link to the given color profile.
			cellStyle := cell.Style
			cellLink := cell.Link
			if cellStyle.Empty() && !pen.Empty() {
				writePending()
				buf.WriteString(ansi.ResetStyle) //nolint:errcheck
				pen.Reset()
			}
			if !cellStyle.Equal(&pen) {
				writePending()
				seq := cellStyle.DiffSequence(pen)
				buf.WriteString(seq) // nolint:errcheck
				pen = cellStyle
			}

			// Write the URL escape sequence
			if cellLink != link && link.URL != "" {
				writePending()
				buf.WriteString(ansi.ResetHyperlink()) //nolint:errcheck
				link.Reset()
			}
			if cellLink != link {
				writePending()
				buf.WriteString(ansi.SetHyperlink(cellLink.URL, cellLink.Params)) //nolint:errcheck
				link = cellLink
			}

			// We only write the cell content if it's not empty. If it is, we
			// append it to the pending line and width to be evaluated later.
			if cell.Equal(&BlankCell) {
				pendingLine += cell.String()
				pendingWidth += cell.Width
			} else {
				writePending()
				buf.WriteString(cell.String())
				w += cell.Width
			}
		}
	}
	if link.URL != "" {
		buf.WriteString(ansi.ResetHyperlink()) //nolint:errcheck
	}
	if !pen.Empty() {
		buf.WriteString(ansi.ResetStyle) //nolint:errcheck
	}
	return w, strings.TrimRight(buf.String(), " ") // Trim trailing spaces
}

// ScreenWriter represents a writer that writes to a [Screen] parsing ANSI
// escape sequences and Unicode characters and converting them into cells that
// can be written to a cell [Buffer].
type ScreenWriter struct {
	*Screen
}

// NewScreenWriter creates a new ScreenWriter that writes to the given Screen.
// This is a convenience function for creating a ScreenWriter.
func NewScreenWriter(s *Screen) *ScreenWriter {
	return &ScreenWriter{s}
}

// Write writes the given bytes to the screen.
// This will recognize ANSI [ansi.SGR] style and [ansi.SetHyperlink] escape
// sequences.
func (s *ScreenWriter) Write(p []byte) (n int, err error) {
	printString(s.Screen, s.method,
		s.cur.X, s.cur.Y, s.Bounds(),
		p, false, "")
	return len(p), nil
}

// SetContent clears the screen with blank cells, and sets the given string as
// its content. If the height or width of the string exceeds the height or
// width of the screen, it will be truncated.
//
// This will recognize ANSI [ansi.SGR] style and [ansi.SetHyperlink] escape sequences.
func (s *ScreenWriter) SetContent(str string) {
	s.SetContentRect(str, s.Bounds())
}

// SetContentRect clears the rectangle within the screen with blank cells, and
// sets the given string as its content. If the height or width of the string
// exceeds the height or width of the screen, it will be truncated.
//
// This will recognize ANSI [ansi.SGR] style and [ansi.SetHyperlink] escape
// sequences.
func (s *ScreenWriter) SetContentRect(str string, rect Rectangle) {
	// Replace all "\n" with "\r\n" to ensure the cursor is reset to the start
	// of the line. Make sure we don't replace "\r\n" with "\r\r\n".
	str = strings.ReplaceAll(str, "\r\n", "\n")
	str = strings.ReplaceAll(str, "\n", "\r\n")
	s.ClearRect(rect)
	printString(s.Screen, s.method,
		rect.Min.X, rect.Min.Y, rect,
		str, true, "")
}

// Print prints the string at the current cursor position. It will wrap the
// string to the width of the screen if it exceeds the width of the screen.
// This will recognize ANSI [ansi.SGR] style and [ansi.SetHyperlink] escape
// sequences.
func (s *ScreenWriter) Print(str string, v ...interface{}) {
	if len(v) > 0 {
		str = fmt.Sprintf(str, v...)
	}
	printString(s.Screen, s.method,
		s.cur.X, s.cur.Y, s.Bounds(),
		str, false, "")
}

// PrintAt prints the string at the given position. It will wrap the string to
// the width of the screen if it exceeds the width of the screen.
// This will recognize ANSI [ansi.SGR] style and [ansi.SetHyperlink] escape
// sequences.
func (s *ScreenWriter) PrintAt(x, y int, str string, v ...interface{}) {
	if len(v) > 0 {
		str = fmt.Sprintf(str, v...)
	}
	printString(s.Screen, s.method,
		x, y, s.Bounds(),
		str, false, "")
}

// PrintCrop prints the string at the current cursor position and truncates the
// text if it exceeds the width of the screen. Use tail to specify a string to
// append if the string is truncated.
// This will recognize ANSI [ansi.SGR] style and [ansi.SetHyperlink] escape
// sequences.
func (s *ScreenWriter) PrintCrop(str string, tail string) {
	printString(s.Screen, s.method,
		s.cur.X, s.cur.Y, s.Bounds(),
		str, true, tail)
}

// PrintCropAt prints the string at the given position and truncates the text
// if it exceeds the width of the screen. Use tail to specify a string to append
// if the string is truncated.
// This will recognize ANSI [ansi.SGR] style and [ansi.SetHyperlink] escape
// sequences.
func (s *ScreenWriter) PrintCropAt(x, y int, str string, tail string) {
	printString(s.Screen, s.method,
		x, y, s.Bounds(),
		str, true, tail)
}

// printString draws a string starting at the given position.
func printString[T []byte | string](
	s CellBuffer,
	m ansi.Method,
	x, y int,
	bounds Rectangle, str T,
	truncate bool, tail string,
) {
	p := ansi.GetParser()
	defer ansi.PutParser(p)

	var tailc Cell
	if truncate && len(tail) > 0 {
		if m == ansi.WcWidth {
			tailc = *NewCellString(tail)
		} else {
			tailc = *NewGraphemeCell(tail)
		}
	}

	decoder := ansi.DecodeSequenceWc[T]
	if m == ansi.GraphemeWidth {
		decoder = ansi.DecodeSequence[T]
	}

	var cell Cell
	var style Style
	var link Link
	var state byte
	for len(str) > 0 {
		seq, width, n, newState := decoder(str, state, p)

		switch width {
		case 1, 2, 3, 4: // wide cells can go up to 4 cells wide
			cell.Width += width
			cell.Append([]rune(string(seq))...)

			if !truncate && x+cell.Width > bounds.Max.X && y+1 < bounds.Max.Y {
				// Wrap the string to the width of the window
				x = bounds.Min.X
				y++
			}
			if Pos(x, y).In(bounds) {
				if truncate && tailc.Width > 0 && x+cell.Width > bounds.Max.X-tailc.Width {
					// Truncate the string and append the tail if any.
					cell := tailc
					cell.Style = style
					cell.Link = link
					s.SetCell(x, y, &cell)
					x += tailc.Width
				} else {
					// Print the cell to the screen
					cell.Style = style
					cell.Link = link
					s.SetCell(x, y, &cell) //nolint:errcheck
					x += width
				}
			}

			// String is too long for the line, truncate it.
			// Make sure we reset the cell for the next iteration.
			cell.Reset()
		default:
			// Valid sequences always have a non-zero Cmd.
			// TODO: Handle cursor movement and other sequences
			switch {
			case ansi.HasCsiPrefix(seq) && p.Command() == 'm':
				// SGR - Select Graphic Rendition
				ReadStyle(p.Params(), &style)
			case ansi.HasOscPrefix(seq) && p.Command() == 8:
				// Hyperlinks
				ReadLink(p.Data(), &link)
			case ansi.Equal(seq, T("\n")):
				y++
			case ansi.Equal(seq, T("\r")):
				x = bounds.Min.X
			default:
				cell.Append([]rune(string(seq))...)
			}
		}

		// Advance the state and data
		state = newState
		str = str[n:]
	}

	// Make sure to set the last cell if it's not empty.
	if !cell.Empty() {
		s.SetCell(x, y, &cell) //nolint:errcheck
		cell.Reset()
	}
}
