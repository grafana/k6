package ansi

import (
	"strconv"
	"strings"
)

// ModeSetting represents a mode setting.
type ModeSetting byte

// ModeSetting constants.
const (
	ModeNotRecognized ModeSetting = iota
	ModeSet
	ModeReset
	ModePermanentlySet
	ModePermanentlyReset
)

// IsNotRecognized returns true if the mode is not recognized.
func (m ModeSetting) IsNotRecognized() bool {
	return m == ModeNotRecognized
}

// IsSet returns true if the mode is set or permanently set.
func (m ModeSetting) IsSet() bool {
	return m == ModeSet || m == ModePermanentlySet
}

// IsReset returns true if the mode is reset or permanently reset.
func (m ModeSetting) IsReset() bool {
	return m == ModeReset || m == ModePermanentlyReset
}

// IsPermanentlySet returns true if the mode is permanently set.
func (m ModeSetting) IsPermanentlySet() bool {
	return m == ModePermanentlySet
}

// IsPermanentlyReset returns true if the mode is permanently reset.
func (m ModeSetting) IsPermanentlyReset() bool {
	return m == ModePermanentlyReset
}

// Mode represents an interface for terminal modes.
// Modes can be set, reset, and requested.
type Mode interface {
	Mode() int
}

// SetMode (SM) or (DECSET) returns a sequence to set a mode.
// The mode arguments are a list of modes to set.
//
// If one of the modes is a [DECMode], the function will returns two escape
// sequences.
//
// ANSI format:
//
//	CSI Pd ; ... ; Pd h
//
// DEC format:
//
//	CSI ? Pd ; ... ; Pd h
//
// See: https://vt100.net/docs/vt510-rm/SM.html
func SetMode(modes ...Mode) string {
	return setMode(false, modes...)
}

// SM is an alias for [SetMode].
func SM(modes ...Mode) string {
	return SetMode(modes...)
}

// DECSET is an alias for [SetMode].
func DECSET(modes ...Mode) string {
	return SetMode(modes...)
}

// ResetMode (RM) or (DECRST) returns a sequence to reset a mode.
// The mode arguments are a list of modes to reset.
//
// If one of the modes is a [DECMode], the function will returns two escape
// sequences.
//
// ANSI format:
//
//	CSI Pd ; ... ; Pd l
//
// DEC format:
//
//	CSI ? Pd ; ... ; Pd l
//
// See: https://vt100.net/docs/vt510-rm/RM.html
func ResetMode(modes ...Mode) string {
	return setMode(true, modes...)
}

// RM is an alias for [ResetMode].
func RM(modes ...Mode) string {
	return ResetMode(modes...)
}

// DECRST is an alias for [ResetMode].
func DECRST(modes ...Mode) string {
	return ResetMode(modes...)
}

func setMode(reset bool, modes ...Mode) (s string) {
	if len(modes) == 0 {
		return //nolint:nakedret
	}

	cmd := "h"
	if reset {
		cmd = "l"
	}

	seq := "\x1b["
	if len(modes) == 1 {
		switch modes[0].(type) {
		case DECMode:
			seq += "?"
		}
		return seq + strconv.Itoa(modes[0].Mode()) + cmd
	}

	dec := make([]string, 0, len(modes)/2)
	ansi := make([]string, 0, len(modes)/2)
	for _, m := range modes {
		switch m.(type) {
		case DECMode:
			dec = append(dec, strconv.Itoa(m.Mode()))
		case ANSIMode:
			ansi = append(ansi, strconv.Itoa(m.Mode()))
		}
	}

	if len(ansi) > 0 {
		s += seq + strings.Join(ansi, ";") + cmd
	}
	if len(dec) > 0 {
		s += seq + "?" + strings.Join(dec, ";") + cmd
	}
	return //nolint:nakedret
}

// RequestMode (DECRQM) returns a sequence to request a mode from the terminal.
// The terminal responds with a report mode function [DECRPM].
//
// ANSI format:
//
//	CSI Pa $ p
//
// DEC format:
//
//	CSI ? Pa $ p
//
// See: https://vt100.net/docs/vt510-rm/DECRQM.html
func RequestMode(m Mode) string {
	seq := "\x1b["
	switch m.(type) {
	case DECMode:
		seq += "?"
	}
	return seq + strconv.Itoa(m.Mode()) + "$p"
}

// DECRQM is an alias for [RequestMode].
func DECRQM(m Mode) string {
	return RequestMode(m)
}

// ReportMode (DECRPM) returns a sequence that the terminal sends to the host
// in response to a mode request [DECRQM].
//
// ANSI format:
//
//	CSI Pa ; Ps ; $ y
//
// DEC format:
//
//	CSI ? Pa ; Ps $ y
//
// Where Pa is the mode number, and Ps is the mode value.
//
//	0: Not recognized
//	1: Set
//	2: Reset
//	3: Permanent set
//	4: Permanent reset
//
// See: https://vt100.net/docs/vt510-rm/DECRPM.html
func ReportMode(mode Mode, value ModeSetting) string {
	if value > 4 {
		value = 0
	}
	switch mode.(type) {
	case DECMode:
		return "\x1b[?" + strconv.Itoa(mode.Mode()) + ";" + strconv.Itoa(int(value)) + "$y"
	}
	return "\x1b[" + strconv.Itoa(mode.Mode()) + ";" + strconv.Itoa(int(value)) + "$y"
}

// DECRPM is an alias for [ReportMode].
func DECRPM(mode Mode, value ModeSetting) string {
	return ReportMode(mode, value)
}

// ANSIMode represents an ANSI terminal mode.
type ANSIMode int //nolint:revive

// Mode returns the ANSI mode as an integer.
func (m ANSIMode) Mode() int {
	return int(m)
}

// DECMode represents a private DEC terminal mode.
type DECMode int

// Mode returns the DEC mode as an integer.
func (m DECMode) Mode() int {
	return int(m)
}

// Keyboard Action Mode (KAM) is a mode that controls locking of the keyboard.
// When the keyboard is locked, it cannot send data to the terminal.
//
// See: https://vt100.net/docs/vt510-rm/KAM.html
const (
	KeyboardActionMode = ANSIMode(2)
	KAM                = KeyboardActionMode

	SetKeyboardActionMode     = "\x1b[2h"
	ResetKeyboardActionMode   = "\x1b[2l"
	RequestKeyboardActionMode = "\x1b[2$p"
)

// Insert/Replace Mode (IRM) is a mode that determines whether characters are
// inserted or replaced when typed.
//
// When enabled, characters are inserted at the cursor position pushing the
// characters to the right. When disabled, characters replace the character at
// the cursor position.
//
// See: https://vt100.net/docs/vt510-rm/IRM.html
const (
	InsertReplaceMode = ANSIMode(4)
	IRM               = InsertReplaceMode

	SetInsertReplaceMode     = "\x1b[4h"
	ResetInsertReplaceMode   = "\x1b[4l"
	RequestInsertReplaceMode = "\x1b[4$p"
)

// BiDirectional Support Mode (BDSM) is a mode that determines whether the
// terminal supports bidirectional text. When enabled, the terminal supports
// bidirectional text and is set to implicit bidirectional mode. When disabled,
// the terminal does not support bidirectional text.
//
// See ECMA-48 7.2.1.
const (
	BiDirectionalSupportMode = ANSIMode(8)
	BDSM                     = BiDirectionalSupportMode

	SetBiDirectionalSupportMode     = "\x1b[8h"
	ResetBiDirectionalSupportMode   = "\x1b[8l"
	RequestBiDirectionalSupportMode = "\x1b[8$p"
)

// Send Receive Mode (SRM) or Local Echo Mode is a mode that determines whether
// the terminal echoes characters back to the host. When enabled, the terminal
// sends characters to the host as they are typed.
//
// See: https://vt100.net/docs/vt510-rm/SRM.html
const (
	SendReceiveMode = ANSIMode(12)
	LocalEchoMode   = SendReceiveMode
	SRM             = SendReceiveMode

	SetSendReceiveMode     = "\x1b[12h"
	ResetSendReceiveMode   = "\x1b[12l"
	RequestSendReceiveMode = "\x1b[12$p"

	SetLocalEchoMode     = "\x1b[12h"
	ResetLocalEchoMode   = "\x1b[12l"
	RequestLocalEchoMode = "\x1b[12$p"
)

// Line Feed/New Line Mode (LNM) is a mode that determines whether the terminal
// interprets the line feed character as a new line.
//
// When enabled, the terminal interprets the line feed character as a new line.
// When disabled, the terminal interprets the line feed character as a line feed.
//
// A new line moves the cursor to the first position of the next line.
// A line feed moves the cursor down one line without changing the column
// scrolling the screen if necessary.
//
// See: https://vt100.net/docs/vt510-rm/LNM.html
const (
	LineFeedNewLineMode = ANSIMode(20)
	LNM                 = LineFeedNewLineMode

	SetLineFeedNewLineMode     = "\x1b[20h"
	ResetLineFeedNewLineMode   = "\x1b[20l"
	RequestLineFeedNewLineMode = "\x1b[20$p"
)

// Cursor Keys Mode (DECCKM) is a mode that determines whether the cursor keys
// send ANSI cursor sequences or application sequences.
//
// See: https://vt100.net/docs/vt510-rm/DECCKM.html
const (
	CursorKeysMode = DECMode(1)
	DECCKM         = CursorKeysMode

	SetCursorKeysMode     = "\x1b[?1h"
	ResetCursorKeysMode   = "\x1b[?1l"
	RequestCursorKeysMode = "\x1b[?1$p"
)

// Deprecated: use [SetCursorKeysMode] and [ResetCursorKeysMode] instead.
const (
	EnableCursorKeys  = "\x1b[?1h" //nolint:revive // grouped constants
	DisableCursorKeys = "\x1b[?1l"
)

// Origin Mode (DECOM) is a mode that determines whether the cursor moves to the
// home position or the margin position.
//
// See: https://vt100.net/docs/vt510-rm/DECOM.html
const (
	OriginMode = DECMode(6)
	DECOM      = OriginMode

	SetOriginMode     = "\x1b[?6h"
	ResetOriginMode   = "\x1b[?6l"
	RequestOriginMode = "\x1b[?6$p"
)

// Auto Wrap Mode (DECAWM) is a mode that determines whether the cursor wraps
// to the next line when it reaches the right margin.
//
// See: https://vt100.net/docs/vt510-rm/DECAWM.html
const (
	AutoWrapMode = DECMode(7)
	DECAWM       = AutoWrapMode

	SetAutoWrapMode     = "\x1b[?7h"
	ResetAutoWrapMode   = "\x1b[?7l"
	RequestAutoWrapMode = "\x1b[?7$p"
)

// X10 Mouse Mode is a mode that determines whether the mouse reports on button
// presses.
//
// The terminal responds with the following encoding:
//
//	CSI M CbCxCy
//
// Where Cb is the button-1, where it can be 1, 2, or 3.
// Cx and Cy are the x and y coordinates of the mouse event.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	X10MouseMode = DECMode(9)

	SetX10MouseMode     = "\x1b[?9h"
	ResetX10MouseMode   = "\x1b[?9l"
	RequestX10MouseMode = "\x1b[?9$p"
)

// Text Cursor Enable Mode (DECTCEM) is a mode that shows/hides the cursor.
//
// See: https://vt100.net/docs/vt510-rm/DECTCEM.html
const (
	TextCursorEnableMode = DECMode(25)
	DECTCEM              = TextCursorEnableMode

	SetTextCursorEnableMode     = "\x1b[?25h"
	ResetTextCursorEnableMode   = "\x1b[?25l"
	RequestTextCursorEnableMode = "\x1b[?25$p"
)

// These are aliases for [SetTextCursorEnableMode] and [ResetTextCursorEnableMode].
const (
	ShowCursor = SetTextCursorEnableMode
	HideCursor = ResetTextCursorEnableMode
)

// Text Cursor Enable Mode (DECTCEM) is a mode that shows/hides the cursor.
//
// See: https://vt100.net/docs/vt510-rm/DECTCEM.html
//
// Deprecated: use [SetTextCursorEnableMode] and [ResetTextCursorEnableMode] instead.
const (
	CursorEnableMode        = DECMode(25)
	RequestCursorVisibility = "\x1b[?25$p"
)

// Numeric Keypad Mode (DECNKM) is a mode that determines whether the keypad
// sends application sequences or numeric sequences.
//
// This works like [DECKPAM] and [DECKPNM], but uses different sequences.
//
// See: https://vt100.net/docs/vt510-rm/DECNKM.html
const (
	NumericKeypadMode = DECMode(66)
	DECNKM            = NumericKeypadMode

	SetNumericKeypadMode     = "\x1b[?66h"
	ResetNumericKeypadMode   = "\x1b[?66l"
	RequestNumericKeypadMode = "\x1b[?66$p"
)

// Backarrow Key Mode (DECBKM) is a mode that determines whether the backspace
// key sends a backspace or delete character. Disabled by default.
//
// See: https://vt100.net/docs/vt510-rm/DECBKM.html
const (
	BackarrowKeyMode = DECMode(67)
	DECBKM           = BackarrowKeyMode

	SetBackarrowKeyMode     = "\x1b[?67h"
	ResetBackarrowKeyMode   = "\x1b[?67l"
	RequestBackarrowKeyMode = "\x1b[?67$p"
)

// Left Right Margin Mode (DECLRMM) is a mode that determines whether the left
// and right margins can be set with [DECSLRM].
//
// See: https://vt100.net/docs/vt510-rm/DECLRMM.html
const (
	LeftRightMarginMode = DECMode(69)
	DECLRMM             = LeftRightMarginMode

	SetLeftRightMarginMode     = "\x1b[?69h"
	ResetLeftRightMarginMode   = "\x1b[?69l"
	RequestLeftRightMarginMode = "\x1b[?69$p"
)

// Normal Mouse Mode is a mode that determines whether the mouse reports on
// button presses and releases. It will also report modifier keys, wheel
// events, and extra buttons.
//
// It uses the same encoding as [X10MouseMode] with a few differences:
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	NormalMouseMode = DECMode(1000)

	SetNormalMouseMode     = "\x1b[?1000h"
	ResetNormalMouseMode   = "\x1b[?1000l"
	RequestNormalMouseMode = "\x1b[?1000$p"
)

// VT Mouse Tracking is a mode that determines whether the mouse reports on
// button press and release.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
//
// Deprecated: use [NormalMouseMode] instead.
const (
	MouseMode = DECMode(1000)

	EnableMouse  = "\x1b[?1000h"
	DisableMouse = "\x1b[?1000l"
	RequestMouse = "\x1b[?1000$p"
)

// Highlight Mouse Tracking is a mode that determines whether the mouse reports
// on button presses, releases, and highlighted cells.
//
// It uses the same encoding as [NormalMouseMode] with a few differences:
//
// On highlight events, the terminal responds with the following encoding:
//
//	CSI t CxCy
//	CSI T CxCyCxCyCxCy
//
// Where the parameters are startx, starty, endx, endy, mousex, and mousey.
const (
	HighlightMouseMode = DECMode(1001)

	SetHighlightMouseMode     = "\x1b[?1001h"
	ResetHighlightMouseMode   = "\x1b[?1001l"
	RequestHighlightMouseMode = "\x1b[?1001$p"
)

// VT Hilite Mouse Tracking is a mode that determines whether the mouse reports on
// button presses, releases, and highlighted cells.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
//
// Deprecated: use [HighlightMouseMode] instead.
const (
	MouseHiliteMode = DECMode(1001)

	EnableMouseHilite  = "\x1b[?1001h"
	DisableMouseHilite = "\x1b[?1001l"
	RequestMouseHilite = "\x1b[?1001$p"
)

// Button Event Mouse Tracking is essentially the same as [NormalMouseMode],
// but it also reports button-motion events when a button is pressed.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	ButtonEventMouseMode = DECMode(1002)

	SetButtonEventMouseMode     = "\x1b[?1002h"
	ResetButtonEventMouseMode   = "\x1b[?1002l"
	RequestButtonEventMouseMode = "\x1b[?1002$p"
)

// Cell Motion Mouse Tracking is a mode that determines whether the mouse
// reports on button press, release, and motion events.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
//
// Deprecated: use [ButtonEventMouseMode] instead.
const (
	MouseCellMotionMode = DECMode(1002)

	EnableMouseCellMotion  = "\x1b[?1002h"
	DisableMouseCellMotion = "\x1b[?1002l"
	RequestMouseCellMotion = "\x1b[?1002$p"
)

// Any Event Mouse Tracking is the same as [ButtonEventMouseMode], except that
// all motion events are reported even if no mouse buttons are pressed.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	AnyEventMouseMode = DECMode(1003)

	SetAnyEventMouseMode     = "\x1b[?1003h"
	ResetAnyEventMouseMode   = "\x1b[?1003l"
	RequestAnyEventMouseMode = "\x1b[?1003$p"
)

// All Mouse Tracking is a mode that determines whether the mouse reports on
// button press, release, motion, and highlight events.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
//
// Deprecated: use [AnyEventMouseMode] instead.
const (
	MouseAllMotionMode = DECMode(1003)

	EnableMouseAllMotion  = "\x1b[?1003h"
	DisableMouseAllMotion = "\x1b[?1003l"
	RequestMouseAllMotion = "\x1b[?1003$p"
)

// Focus Event Mode is a mode that determines whether the terminal reports focus
// and blur events.
//
// The terminal sends the following encoding:
//
//	CSI I // Focus In
//	CSI O // Focus Out
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Focus-Tracking
const (
	FocusEventMode = DECMode(1004)

	SetFocusEventMode     = "\x1b[?1004h"
	ResetFocusEventMode   = "\x1b[?1004l"
	RequestFocusEventMode = "\x1b[?1004$p"
)

// Deprecated: use [SetFocusEventMode], [ResetFocusEventMode], and
// [RequestFocusEventMode] instead.
// Focus reporting mode constants.
const (
	ReportFocusMode = DECMode(1004) //nolint:revive // grouped constants

	EnableReportFocus  = "\x1b[?1004h"
	DisableReportFocus = "\x1b[?1004l"
	RequestReportFocus = "\x1b[?1004$p"
)

// SGR Extended Mouse Mode is a mode that changes the mouse tracking encoding
// to use SGR parameters.
//
// The terminal responds with the following encoding:
//
//	CSI < Cb ; Cx ; Cy M
//
// Where Cb is the same as [NormalMouseMode], and Cx and Cy are the x and y.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	SgrExtMouseMode = DECMode(1006)

	SetSgrExtMouseMode     = "\x1b[?1006h"
	ResetSgrExtMouseMode   = "\x1b[?1006l"
	RequestSgrExtMouseMode = "\x1b[?1006$p"
)

// Deprecated: use [SgrExtMouseMode] [SetSgrExtMouseMode],
// [ResetSgrExtMouseMode], and [RequestSgrExtMouseMode] instead.
const (
	MouseSgrExtMode    = DECMode(1006) //nolint:revive // grouped constants
	EnableMouseSgrExt  = "\x1b[?1006h"
	DisableMouseSgrExt = "\x1b[?1006l"
	RequestMouseSgrExt = "\x1b[?1006$p"
)

// UTF-8 Extended Mouse Mode is a mode that changes the mouse tracking encoding
// to use UTF-8 parameters.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	Utf8ExtMouseMode = DECMode(1005)

	SetUtf8ExtMouseMode     = "\x1b[?1005h"
	ResetUtf8ExtMouseMode   = "\x1b[?1005l"
	RequestUtf8ExtMouseMode = "\x1b[?1005$p"
)

// URXVT Extended Mouse Mode is a mode that changes the mouse tracking encoding
// to use an alternate encoding.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	UrxvtExtMouseMode = DECMode(1015)

	SetUrxvtExtMouseMode     = "\x1b[?1015h"
	ResetUrxvtExtMouseMode   = "\x1b[?1015l"
	RequestUrxvtExtMouseMode = "\x1b[?1015$p"
)

// SGR Pixel Extended Mouse Mode is a mode that changes the mouse tracking
// encoding to use SGR parameters with pixel coordinates.
//
// This is similar to [SgrExtMouseMode], but also reports pixel coordinates.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Mouse-Tracking
const (
	SgrPixelExtMouseMode = DECMode(1016)

	SetSgrPixelExtMouseMode     = "\x1b[?1016h"
	ResetSgrPixelExtMouseMode   = "\x1b[?1016l"
	RequestSgrPixelExtMouseMode = "\x1b[?1016$p"
)

// Alternate Screen Mode is a mode that determines whether the alternate screen
// buffer is active. When this mode is enabled, the alternate screen buffer is
// cleared.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-The-Alternate-Screen-Buffer
const (
	AltScreenMode = DECMode(1047)

	SetAltScreenMode     = "\x1b[?1047h"
	ResetAltScreenMode   = "\x1b[?1047l"
	RequestAltScreenMode = "\x1b[?1047$p"
)

// Save Cursor Mode is a mode that saves the cursor position.
// This is equivalent to [SaveCursor] and [RestoreCursor].
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-The-Alternate-Screen-Buffer
const (
	SaveCursorMode = DECMode(1048)

	SetSaveCursorMode     = "\x1b[?1048h"
	ResetSaveCursorMode   = "\x1b[?1048l"
	RequestSaveCursorMode = "\x1b[?1048$p"
)

// Alternate Screen Save Cursor Mode is a mode that saves the cursor position as in
// [SaveCursorMode], switches to the alternate screen buffer as in [AltScreenMode],
// and clears the screen on switch.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-The-Alternate-Screen-Buffer
const (
	AltScreenSaveCursorMode = DECMode(1049)

	SetAltScreenSaveCursorMode     = "\x1b[?1049h"
	ResetAltScreenSaveCursorMode   = "\x1b[?1049l"
	RequestAltScreenSaveCursorMode = "\x1b[?1049$p"
)

// Alternate Screen Buffer is a mode that determines whether the alternate screen
// buffer is active.
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-The-Alternate-Screen-Buffer
//
// Deprecated: use [AltScreenSaveCursorMode] instead.
const (
	AltScreenBufferMode = DECMode(1049)

	SetAltScreenBufferMode     = "\x1b[?1049h"
	ResetAltScreenBufferMode   = "\x1b[?1049l"
	RequestAltScreenBufferMode = "\x1b[?1049$p"

	EnableAltScreenBuffer  = "\x1b[?1049h"
	DisableAltScreenBuffer = "\x1b[?1049l"
	RequestAltScreenBuffer = "\x1b[?1049$p"
)

// Bracketed Paste Mode is a mode that determines whether pasted text is
// bracketed with escape sequences.
//
// See: https://cirw.in/blog/bracketed-paste
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h2-Bracketed-Paste-Mode
const (
	BracketedPasteMode = DECMode(2004)

	SetBracketedPasteMode     = "\x1b[?2004h"
	ResetBracketedPasteMode   = "\x1b[?2004l"
	RequestBracketedPasteMode = "\x1b[?2004$p"
)

// Deprecated: use [SetBracketedPasteMode], [ResetBracketedPasteMode], and
// [RequestBracketedPasteMode] instead.
const (
	EnableBracketedPaste  = "\x1b[?2004h" //nolint:revive // grouped constants
	DisableBracketedPaste = "\x1b[?2004l"
	RequestBracketedPaste = "\x1b[?2004$p"
)

// Synchronized Output Mode is a mode that determines whether output is
// synchronized with the terminal.
//
// See: https://gist.github.com/christianparpart/d8a62cc1ab659194337d73e399004036
const (
	SynchronizedOutputMode = DECMode(2026)

	SetSynchronizedOutputMode     = "\x1b[?2026h"
	ResetSynchronizedOutputMode   = "\x1b[?2026l"
	RequestSynchronizedOutputMode = "\x1b[?2026$p"
)

// Synchronized Output Mode. See [SynchronizedOutputMode].
//
// Deprecated: use [SynchronizedOutputMode], [SetSynchronizedOutputMode], and
// [ResetSynchronizedOutputMode], and [RequestSynchronizedOutputMode] instead.
const (
	SyncdOutputMode = DECMode(2026)

	EnableSyncdOutput  = "\x1b[?2026h"
	DisableSyncdOutput = "\x1b[?2026l"
	RequestSyncdOutput = "\x1b[?2026$p"
)

// Unicode Core Mode is a mode that determines whether the terminal should use
// Unicode grapheme clustering to calculate the width of glyphs for each
// terminal cell.
//
// See: https://github.com/contour-terminal/terminal-unicode-core
const (
	UnicodeCoreMode = DECMode(2027)

	SetUnicodeCoreMode     = "\x1b[?2027h"
	ResetUnicodeCoreMode   = "\x1b[?2027l"
	RequestUnicodeCoreMode = "\x1b[?2027$p"
)

// Grapheme Clustering Mode is a mode that determines whether the terminal
// should look for grapheme clusters instead of single runes in the rendered
// text. This makes the terminal properly render combining characters such as
// emojis.
//
// See: https://github.com/contour-terminal/terminal-unicode-core
//
// Deprecated: use [GraphemeClusteringMode], [SetUnicodeCoreMode],
// [ResetUnicodeCoreMode], and [RequestUnicodeCoreMode] instead.
const (
	GraphemeClusteringMode = DECMode(2027)

	SetGraphemeClusteringMode     = "\x1b[?2027h"
	ResetGraphemeClusteringMode   = "\x1b[?2027l"
	RequestGraphemeClusteringMode = "\x1b[?2027$p"
)

// Grapheme Clustering Mode. See [GraphemeClusteringMode].
//
// Deprecated: use [SetUnicodeCoreMode], [ResetUnicodeCoreMode], and
// [RequestUnicodeCoreMode] instead.
const (
	EnableGraphemeClustering  = "\x1b[?2027h"
	DisableGraphemeClustering = "\x1b[?2027l"
	RequestGraphemeClustering = "\x1b[?2027$p"
)

// LightDarkMode is a mode that enables reporting the operating system's color
// scheme (light or dark) preference. It reports the color scheme as a [DSR]
// and [LightDarkReport] escape sequences encoded as follows:
//
//	CSI ? 997 ; 1 n   for dark mode
//	CSI ? 997 ; 2 n   for light mode
//
// The color preference can also be requested via the following [DSR] and
// [RequestLightDarkReport] escape sequences:
//
//	CSI ? 996 n
//
// See: https://contour-terminal.org/vt-extensions/color-palette-update-notifications/
const (
	LightDarkMode = DECMode(2031)

	SetLightDarkMode     = "\x1b[?2031h"
	ResetLightDarkMode   = "\x1b[?2031l"
	RequestLightDarkMode = "\x1b[?2031$p"
)

// InBandResizeMode is a mode that reports terminal resize events as escape
// sequences. This is useful for systems that do not support [SIGWINCH] like
// Windows.
//
// The terminal then sends the following encoding:
//
//	CSI 48 ; cellsHeight ; cellsWidth ; pixelHeight ; pixelWidth t
//
// See: https://gist.github.com/rockorager/e695fb2924d36b2bcf1fff4a3704bd83
const (
	InBandResizeMode = DECMode(2048)

	SetInBandResizeMode     = "\x1b[?2048h"
	ResetInBandResizeMode   = "\x1b[?2048l"
	RequestInBandResizeMode = "\x1b[?2048$p"
)

// Win32Input is a mode that determines whether input is processed by the
// Win32 console and Conpty.
//
// See: https://github.com/microsoft/terminal/blob/main/doc/specs/%234999%20-%20Improved%20keyboard%20handling%20in%20Conpty.md
const (
	Win32InputMode = DECMode(9001)

	SetWin32InputMode     = "\x1b[?9001h"
	ResetWin32InputMode   = "\x1b[?9001l"
	RequestWin32InputMode = "\x1b[?9001$p"
)

// Deprecated: use [SetWin32InputMode], [ResetWin32InputMode], and
// [RequestWin32InputMode] instead.
const (
	EnableWin32Input  = "\x1b[?9001h" //nolint:revive // grouped constants
	DisableWin32Input = "\x1b[?9001l"
	RequestWin32Input = "\x1b[?9001$p"
)
