package termenv

import (
	"fmt"
	"strings"
)

// Sequence definitions.
const (
	// Cursor positioning.
	CursorUpSeq              = "%dA"
	CursorDownSeq            = "%dB"
	CursorForwardSeq         = "%dC"
	CursorBackSeq            = "%dD"
	CursorNextLineSeq        = "%dE"
	CursorPreviousLineSeq    = "%dF"
	CursorHorizontalSeq      = "%dG"
	CursorPositionSeq        = "%d;%dH"
	EraseDisplaySeq          = "%dJ"
	EraseLineSeq             = "%dK"
	ScrollUpSeq              = "%dS"
	ScrollDownSeq            = "%dT"
	SaveCursorPositionSeq    = "s"
	RestoreCursorPositionSeq = "u"
	ChangeScrollingRegionSeq = "%d;%dr"
	InsertLineSeq            = "%dL"
	DeleteLineSeq            = "%dM"

	// Explicit values for EraseLineSeq.
	EraseLineRightSeq  = "0K"
	EraseLineLeftSeq   = "1K"
	EraseEntireLineSeq = "2K"

	// Mouse.
	EnableMousePressSeq         = "?9h" // press only (X10)
	DisableMousePressSeq        = "?9l"
	EnableMouseSeq              = "?1000h" // press, release, wheel
	DisableMouseSeq             = "?1000l"
	EnableMouseHiliteSeq        = "?1001h" // highlight
	DisableMouseHiliteSeq       = "?1001l"
	EnableMouseCellMotionSeq    = "?1002h" // press, release, move on pressed, wheel
	DisableMouseCellMotionSeq   = "?1002l"
	EnableMouseAllMotionSeq     = "?1003h" // press, release, move, wheel
	DisableMouseAllMotionSeq    = "?1003l"
	EnableMouseExtendedModeSeq  = "?1006h" // press, release, move, wheel, extended coordinates
	DisableMouseExtendedModeSeq = "?1006l"
	EnableMousePixelsModeSeq    = "?1016h" // press, release, move, wheel, extended pixel coordinates
	DisableMousePixelsModeSeq   = "?1016l"

	// Screen.
	RestoreScreenSeq = "?47l"
	SaveScreenSeq    = "?47h"
	AltScreenSeq     = "?1049h"
	ExitAltScreenSeq = "?1049l"

	// Bracketed paste.
	// https://en.wikipedia.org/wiki/Bracketed-paste
	EnableBracketedPasteSeq  = "?2004h"
	DisableBracketedPasteSeq = "?2004l"
	StartBracketedPasteSeq   = "200~"
	EndBracketedPasteSeq     = "201~"

	// Session.
	SetWindowTitleSeq     = "2;%s" + string(BEL)
	SetForegroundColorSeq = "10;%s" + string(BEL)
	SetBackgroundColorSeq = "11;%s" + string(BEL)
	SetCursorColorSeq     = "12;%s" + string(BEL)
	ShowCursorSeq         = "?25h"
	HideCursorSeq         = "?25l"
)

// Reset the terminal to its default style, removing any active styles.
func (o Output) Reset() {
	fmt.Fprint(o.w, CSI+ResetSeq+"m") //nolint:errcheck
}

// SetForegroundColor sets the default foreground color.
func (o Output) SetForegroundColor(color Color) {
	fmt.Fprintf(o.w, OSC+SetForegroundColorSeq, color) //nolint:errcheck
}

// SetBackgroundColor sets the default background color.
func (o Output) SetBackgroundColor(color Color) {
	fmt.Fprintf(o.w, OSC+SetBackgroundColorSeq, color) //nolint:errcheck
}

// SetCursorColor sets the cursor color.
func (o Output) SetCursorColor(color Color) {
	fmt.Fprintf(o.w, OSC+SetCursorColorSeq, color) //nolint:errcheck
}

// RestoreScreen restores a previously saved screen state.
func (o Output) RestoreScreen() {
	fmt.Fprint(o.w, CSI+RestoreScreenSeq) //nolint:errcheck
}

// SaveScreen saves the screen state.
func (o Output) SaveScreen() {
	fmt.Fprint(o.w, CSI+SaveScreenSeq) //nolint:errcheck
}

// AltScreen switches to the alternate screen buffer. The former view can be
// restored with ExitAltScreen().
func (o Output) AltScreen() {
	fmt.Fprint(o.w, CSI+AltScreenSeq) //nolint:errcheck
}

// ExitAltScreen exits the alternate screen buffer and returns to the former
// terminal view.
func (o Output) ExitAltScreen() {
	fmt.Fprint(o.w, CSI+ExitAltScreenSeq) //nolint:errcheck
}

// ClearScreen clears the visible portion of the terminal.
func (o Output) ClearScreen() {
	fmt.Fprintf(o.w, CSI+EraseDisplaySeq, 2) //nolint:errcheck,mnd
	o.MoveCursor(1, 1)
}

// MoveCursor moves the cursor to a given position.
func (o Output) MoveCursor(row int, column int) {
	fmt.Fprintf(o.w, CSI+CursorPositionSeq, row, column) //nolint:errcheck
}

// HideCursor hides the cursor.
func (o Output) HideCursor() {
	fmt.Fprint(o.w, CSI+HideCursorSeq) //nolint:errcheck
}

// ShowCursor shows the cursor.
func (o Output) ShowCursor() {
	fmt.Fprint(o.w, CSI+ShowCursorSeq) //nolint:errcheck
}

// SaveCursorPosition saves the cursor position.
func (o Output) SaveCursorPosition() {
	fmt.Fprint(o.w, CSI+SaveCursorPositionSeq) //nolint:errcheck
}

// RestoreCursorPosition restores a saved cursor position.
func (o Output) RestoreCursorPosition() {
	fmt.Fprint(o.w, CSI+RestoreCursorPositionSeq) //nolint:errcheck
}

// CursorUp moves the cursor up a given number of lines.
func (o Output) CursorUp(n int) {
	fmt.Fprintf(o.w, CSI+CursorUpSeq, n) //nolint:errcheck
}

// CursorDown moves the cursor down a given number of lines.
func (o Output) CursorDown(n int) {
	fmt.Fprintf(o.w, CSI+CursorDownSeq, n) //nolint:errcheck
}

// CursorForward moves the cursor up a given number of lines.
func (o Output) CursorForward(n int) {
	fmt.Fprintf(o.w, CSI+CursorForwardSeq, n) //nolint:errcheck
}

// CursorBack moves the cursor backwards a given number of cells.
func (o Output) CursorBack(n int) {
	fmt.Fprintf(o.w, CSI+CursorBackSeq, n) //nolint:errcheck
}

// CursorNextLine moves the cursor down a given number of lines and places it at
// the beginning of the line.
func (o Output) CursorNextLine(n int) {
	fmt.Fprintf(o.w, CSI+CursorNextLineSeq, n) //nolint:errcheck
}

// CursorPrevLine moves the cursor up a given number of lines and places it at
// the beginning of the line.
func (o Output) CursorPrevLine(n int) {
	fmt.Fprintf(o.w, CSI+CursorPreviousLineSeq, n) //nolint:errcheck
}

// ClearLine clears the current line.
func (o Output) ClearLine() {
	fmt.Fprint(o.w, CSI+EraseEntireLineSeq) //nolint:errcheck
}

// ClearLineLeft clears the line to the left of the cursor.
func (o Output) ClearLineLeft() {
	fmt.Fprint(o.w, CSI+EraseLineLeftSeq) //nolint:errcheck
}

// ClearLineRight clears the line to the right of the cursor.
func (o Output) ClearLineRight() {
	fmt.Fprint(o.w, CSI+EraseLineRightSeq) //nolint:errcheck
}

// ClearLines clears a given number of lines.
func (o Output) ClearLines(n int) {
	clearLine := fmt.Sprintf(CSI+EraseLineSeq, 2) //nolint:mnd
	cursorUp := fmt.Sprintf(CSI+CursorUpSeq, 1)
	fmt.Fprint(o.w, clearLine+strings.Repeat(cursorUp+clearLine, n)) //nolint:errcheck
}

// ChangeScrollingRegion sets the scrolling region of the terminal.
func (o Output) ChangeScrollingRegion(top, bottom int) {
	fmt.Fprintf(o.w, CSI+ChangeScrollingRegionSeq, top, bottom) //nolint:errcheck
}

// InsertLines inserts the given number of lines at the top of the scrollable
// region, pushing lines below down.
func (o Output) InsertLines(n int) {
	fmt.Fprintf(o.w, CSI+InsertLineSeq, n) //nolint:errcheck
}

// DeleteLines deletes the given number of lines, pulling any lines in
// the scrollable region below up.
func (o Output) DeleteLines(n int) {
	fmt.Fprintf(o.w, CSI+DeleteLineSeq, n) //nolint:errcheck
}

// EnableMousePress enables X10 mouse mode. Button press events are sent only.
func (o Output) EnableMousePress() {
	fmt.Fprint(o.w, CSI+EnableMousePressSeq) //nolint:errcheck
}

// DisableMousePress disables X10 mouse mode.
func (o Output) DisableMousePress() {
	fmt.Fprint(o.w, CSI+DisableMousePressSeq) //nolint:errcheck
}

// EnableMouse enables Mouse Tracking mode.
func (o Output) EnableMouse() {
	fmt.Fprint(o.w, CSI+EnableMouseSeq) //nolint:errcheck
}

// DisableMouse disables Mouse Tracking mode.
func (o Output) DisableMouse() {
	fmt.Fprint(o.w, CSI+DisableMouseSeq) //nolint:errcheck
}

// EnableMouseHilite enables Hilite Mouse Tracking mode.
func (o Output) EnableMouseHilite() {
	fmt.Fprint(o.w, CSI+EnableMouseHiliteSeq) //nolint:errcheck
}

// DisableMouseHilite disables Hilite Mouse Tracking mode.
func (o Output) DisableMouseHilite() {
	fmt.Fprint(o.w, CSI+DisableMouseHiliteSeq) //nolint:errcheck
}

// EnableMouseCellMotion enables Cell Motion Mouse Tracking mode.
func (o Output) EnableMouseCellMotion() {
	fmt.Fprint(o.w, CSI+EnableMouseCellMotionSeq) //nolint:errcheck
}

// DisableMouseCellMotion disables Cell Motion Mouse Tracking mode.
func (o Output) DisableMouseCellMotion() {
	fmt.Fprint(o.w, CSI+DisableMouseCellMotionSeq) //nolint:errcheck
}

// EnableMouseAllMotion enables All Motion Mouse mode.
func (o Output) EnableMouseAllMotion() {
	fmt.Fprint(o.w, CSI+EnableMouseAllMotionSeq) //nolint:errcheck
}

// DisableMouseAllMotion disables All Motion Mouse mode.
func (o Output) DisableMouseAllMotion() {
	fmt.Fprint(o.w, CSI+DisableMouseAllMotionSeq) //nolint:errcheck
}

// EnableMouseExtendedMotion enables Extended Mouse mode (SGR). This should be
// enabled in conjunction with EnableMouseCellMotion, and EnableMouseAllMotion.
func (o Output) EnableMouseExtendedMode() {
	fmt.Fprint(o.w, CSI+EnableMouseExtendedModeSeq) //nolint:errcheck
}

// DisableMouseExtendedMotion disables Extended Mouse mode (SGR).
func (o Output) DisableMouseExtendedMode() {
	fmt.Fprint(o.w, CSI+DisableMouseExtendedModeSeq) //nolint:errcheck
}

// EnableMousePixelsMotion enables Pixel Motion Mouse mode (SGR-Pixels). This
// should be enabled in conjunction with EnableMouseCellMotion, and
// EnableMouseAllMotion.
func (o Output) EnableMousePixelsMode() {
	fmt.Fprint(o.w, CSI+EnableMousePixelsModeSeq) //nolint:errcheck
}

// DisableMousePixelsMotion disables Pixel Motion Mouse mode (SGR-Pixels).
func (o Output) DisableMousePixelsMode() {
	fmt.Fprint(o.w, CSI+DisableMousePixelsModeSeq) //nolint:errcheck
}

// SetWindowTitle sets the terminal window title.
func (o Output) SetWindowTitle(title string) {
	fmt.Fprintf(o.w, OSC+SetWindowTitleSeq, title) //nolint:errcheck
}

// EnableBracketedPaste enables bracketed paste.
func (o Output) EnableBracketedPaste() {
	fmt.Fprintf(o.w, CSI+EnableBracketedPasteSeq) //nolint:errcheck
}

// DisableBracketedPaste disables bracketed paste.
func (o Output) DisableBracketedPaste() {
	fmt.Fprintf(o.w, CSI+DisableBracketedPasteSeq) //nolint:errcheck
}

// Legacy functions.

// Reset the terminal to its default style, removing any active styles.
//
// Deprecated: please use termenv.Output instead.
func Reset() {
	output.Reset()
}

// SetForegroundColor sets the default foreground color.
//
// Deprecated: please use termenv.Output instead.
func SetForegroundColor(color Color) {
	output.SetForegroundColor(color)
}

// SetBackgroundColor sets the default background color.
//
// Deprecated: please use termenv.Output instead.
func SetBackgroundColor(color Color) {
	output.SetBackgroundColor(color)
}

// SetCursorColor sets the cursor color.
//
// Deprecated: please use termenv.Output instead.
func SetCursorColor(color Color) {
	output.SetCursorColor(color)
}

// RestoreScreen restores a previously saved screen state.
//
// Deprecated: please use termenv.Output instead.
func RestoreScreen() {
	output.RestoreScreen()
}

// SaveScreen saves the screen state.
//
// Deprecated: please use termenv.Output instead.
func SaveScreen() {
	output.SaveScreen()
}

// AltScreen switches to the alternate screen buffer. The former view can be
// restored with ExitAltScreen().
//
// Deprecated: please use termenv.Output instead.
func AltScreen() {
	output.AltScreen()
}

// ExitAltScreen exits the alternate screen buffer and returns to the former
// terminal view.
//
// Deprecated: please use termenv.Output instead.
func ExitAltScreen() {
	output.ExitAltScreen()
}

// ClearScreen clears the visible portion of the terminal.
//
// Deprecated: please use termenv.Output instead.
func ClearScreen() {
	output.ClearScreen()
}

// MoveCursor moves the cursor to a given position.
//
// Deprecated: please use termenv.Output instead.
func MoveCursor(row int, column int) {
	output.MoveCursor(row, column)
}

// HideCursor hides the cursor.
//
// Deprecated: please use termenv.Output instead.
func HideCursor() {
	output.HideCursor()
}

// ShowCursor shows the cursor.
//
// Deprecated: please use termenv.Output instead.
func ShowCursor() {
	output.ShowCursor()
}

// SaveCursorPosition saves the cursor position.
//
// Deprecated: please use termenv.Output instead.
func SaveCursorPosition() {
	output.SaveCursorPosition()
}

// RestoreCursorPosition restores a saved cursor position.
//
// Deprecated: please use termenv.Output instead.
func RestoreCursorPosition() {
	output.RestoreCursorPosition()
}

// CursorUp moves the cursor up a given number of lines.
//
// Deprecated: please use termenv.Output instead.
func CursorUp(n int) {
	output.CursorUp(n)
}

// CursorDown moves the cursor down a given number of lines.
//
// Deprecated: please use termenv.Output instead.
func CursorDown(n int) {
	output.CursorDown(n)
}

// CursorForward moves the cursor up a given number of lines.
//
// Deprecated: please use termenv.Output instead.
func CursorForward(n int) {
	output.CursorForward(n)
}

// CursorBack moves the cursor backwards a given number of cells.
//
// Deprecated: please use termenv.Output instead.
func CursorBack(n int) {
	output.CursorBack(n)
}

// CursorNextLine moves the cursor down a given number of lines and places it at
// the beginning of the line.
//
// Deprecated: please use termenv.Output instead.
func CursorNextLine(n int) {
	output.CursorNextLine(n)
}

// CursorPrevLine moves the cursor up a given number of lines and places it at
// the beginning of the line.
//
// Deprecated: please use termenv.Output instead.
func CursorPrevLine(n int) {
	output.CursorPrevLine(n)
}

// ClearLine clears the current line.
//
// Deprecated: please use termenv.Output instead.
func ClearLine() {
	output.ClearLine()
}

// ClearLineLeft clears the line to the left of the cursor.
//
// Deprecated: please use termenv.Output instead.
func ClearLineLeft() {
	output.ClearLineLeft()
}

// ClearLineRight clears the line to the right of the cursor.
//
// Deprecated: please use termenv.Output instead.
func ClearLineRight() {
	output.ClearLineRight()
}

// ClearLines clears a given number of lines.
//
// Deprecated: please use termenv.Output instead.
func ClearLines(n int) {
	output.ClearLines(n)
}

// ChangeScrollingRegion sets the scrolling region of the terminal.
//
// Deprecated: please use termenv.Output instead.
func ChangeScrollingRegion(top, bottom int) {
	output.ChangeScrollingRegion(top, bottom)
}

// InsertLines inserts the given number of lines at the top of the scrollable
// region, pushing lines below down.
//
// Deprecated: please use termenv.Output instead.
func InsertLines(n int) {
	output.InsertLines(n)
}

// DeleteLines deletes the given number of lines, pulling any lines in
// the scrollable region below up.
//
// Deprecated: please use termenv.Output instead.
func DeleteLines(n int) {
	output.DeleteLines(n)
}

// EnableMousePress enables X10 mouse mode. Button press events are sent only.
//
// Deprecated: please use termenv.Output instead.
func EnableMousePress() {
	output.EnableMousePress()
}

// DisableMousePress disables X10 mouse mode.
//
// Deprecated: please use termenv.Output instead.
func DisableMousePress() {
	output.DisableMousePress()
}

// EnableMouse enables Mouse Tracking mode.
//
// Deprecated: please use termenv.Output instead.
func EnableMouse() {
	output.EnableMouse()
}

// DisableMouse disables Mouse Tracking mode.
//
// Deprecated: please use termenv.Output instead.
func DisableMouse() {
	output.DisableMouse()
}

// EnableMouseHilite enables Hilite Mouse Tracking mode.
//
// Deprecated: please use termenv.Output instead.
func EnableMouseHilite() {
	output.EnableMouseHilite()
}

// DisableMouseHilite disables Hilite Mouse Tracking mode.
//
// Deprecated: please use termenv.Output instead.
func DisableMouseHilite() {
	output.DisableMouseHilite()
}

// EnableMouseCellMotion enables Cell Motion Mouse Tracking mode.
//
// Deprecated: please use termenv.Output instead.
func EnableMouseCellMotion() {
	output.EnableMouseCellMotion()
}

// DisableMouseCellMotion disables Cell Motion Mouse Tracking mode.
//
// Deprecated: please use termenv.Output instead.
func DisableMouseCellMotion() {
	output.DisableMouseCellMotion()
}

// EnableMouseAllMotion enables All Motion Mouse mode.
//
// Deprecated: please use termenv.Output instead.
func EnableMouseAllMotion() {
	output.EnableMouseAllMotion()
}

// DisableMouseAllMotion disables All Motion Mouse mode.
//
// Deprecated: please use termenv.Output instead.
func DisableMouseAllMotion() {
	output.DisableMouseAllMotion()
}

// SetWindowTitle sets the terminal window title.
//
// Deprecated: please use termenv.Output instead.
func SetWindowTitle(title string) {
	output.SetWindowTitle(title)
}

// EnableBracketedPaste enables bracketed paste.
//
// Deprecated: please use termenv.Output instead.
func EnableBracketedPaste() {
	output.EnableBracketedPaste()
}

// DisableBracketedPaste disables bracketed paste.
//
// Deprecated: please use termenv.Output instead.
func DisableBracketedPaste() {
	output.DisableBracketedPaste()
}
