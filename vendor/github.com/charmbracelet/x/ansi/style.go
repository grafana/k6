package ansi

import (
	"image/color"
	"strconv"
	"strings"
)

// ResetStyle is a SGR (Select Graphic Rendition) style sequence that resets
// all attributes.
// See: https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters
const ResetStyle = "\x1b[m"

// Attr is a SGR (Select Graphic Rendition) style attribute.
type Attr = int

// Style represents an ANSI SGR (Select Graphic Rendition) style.
type Style []string

// NewStyle returns a new style with the given attributes.
func NewStyle(attrs ...Attr) Style {
	if len(attrs) == 0 {
		return Style{}
	}
	s := make(Style, 0, len(attrs))
	for _, a := range attrs {
		attr, ok := attrStrings[a]
		if ok {
			s = append(s, attr)
		} else {
			if a < 0 {
				a = 0
			}
			s = append(s, strconv.Itoa(a))
		}
	}
	return s
}

// String returns the ANSI SGR (Select Graphic Rendition) style sequence for
// the given style.
func (s Style) String() string {
	if len(s) == 0 {
		return ResetStyle
	}
	return "\x1b[" + strings.Join(s, ";") + "m"
}

// Styled returns a styled string with the given style applied.
func (s Style) Styled(str string) string {
	if len(s) == 0 {
		return str
	}
	return s.String() + str + ResetStyle
}

// Reset appends the reset style attribute to the style.
func (s Style) Reset() Style {
	return append(s, resetAttr)
}

// Bold appends the bold style attribute to the style.
func (s Style) Bold() Style {
	return append(s, boldAttr)
}

// Faint appends the faint style attribute to the style.
func (s Style) Faint() Style {
	return append(s, faintAttr)
}

// Italic appends the italic style attribute to the style.
func (s Style) Italic() Style {
	return append(s, italicAttr)
}

// Underline appends the underline style attribute to the style.
func (s Style) Underline() Style {
	return append(s, underlineAttr)
}

// UnderlineStyle appends the underline style attribute to the style.
func (s Style) UnderlineStyle(u UnderlineStyle) Style {
	switch u {
	case NoUnderlineStyle:
		return s.NoUnderline()
	case SingleUnderlineStyle:
		return s.Underline()
	case DoubleUnderlineStyle:
		return append(s, doubleUnderlineStyle)
	case CurlyUnderlineStyle:
		return append(s, curlyUnderlineStyle)
	case DottedUnderlineStyle:
		return append(s, dottedUnderlineStyle)
	case DashedUnderlineStyle:
		return append(s, dashedUnderlineStyle)
	}
	return s
}

// DoubleUnderline appends the double underline style attribute to the style.
// This is a convenience method for UnderlineStyle(DoubleUnderlineStyle).
func (s Style) DoubleUnderline() Style {
	return s.UnderlineStyle(DoubleUnderlineStyle)
}

// CurlyUnderline appends the curly underline style attribute to the style.
// This is a convenience method for UnderlineStyle(CurlyUnderlineStyle).
func (s Style) CurlyUnderline() Style {
	return s.UnderlineStyle(CurlyUnderlineStyle)
}

// DottedUnderline appends the dotted underline style attribute to the style.
// This is a convenience method for UnderlineStyle(DottedUnderlineStyle).
func (s Style) DottedUnderline() Style {
	return s.UnderlineStyle(DottedUnderlineStyle)
}

// DashedUnderline appends the dashed underline style attribute to the style.
// This is a convenience method for UnderlineStyle(DashedUnderlineStyle).
func (s Style) DashedUnderline() Style {
	return s.UnderlineStyle(DashedUnderlineStyle)
}

// SlowBlink appends the slow blink style attribute to the style.
func (s Style) SlowBlink() Style {
	return append(s, slowBlinkAttr)
}

// RapidBlink appends the rapid blink style attribute to the style.
func (s Style) RapidBlink() Style {
	return append(s, rapidBlinkAttr)
}

// Reverse appends the reverse style attribute to the style.
func (s Style) Reverse() Style {
	return append(s, reverseAttr)
}

// Conceal appends the conceal style attribute to the style.
func (s Style) Conceal() Style {
	return append(s, concealAttr)
}

// Strikethrough appends the strikethrough style attribute to the style.
func (s Style) Strikethrough() Style {
	return append(s, strikethroughAttr)
}

// NormalIntensity appends the normal intensity style attribute to the style.
func (s Style) NormalIntensity() Style {
	return append(s, normalIntensityAttr)
}

// NoItalic appends the no italic style attribute to the style.
func (s Style) NoItalic() Style {
	return append(s, noItalicAttr)
}

// NoUnderline appends the no underline style attribute to the style.
func (s Style) NoUnderline() Style {
	return append(s, noUnderlineAttr)
}

// NoBlink appends the no blink style attribute to the style.
func (s Style) NoBlink() Style {
	return append(s, noBlinkAttr)
}

// NoReverse appends the no reverse style attribute to the style.
func (s Style) NoReverse() Style {
	return append(s, noReverseAttr)
}

// NoConceal appends the no conceal style attribute to the style.
func (s Style) NoConceal() Style {
	return append(s, noConcealAttr)
}

// NoStrikethrough appends the no strikethrough style attribute to the style.
func (s Style) NoStrikethrough() Style {
	return append(s, noStrikethroughAttr)
}

// DefaultForegroundColor appends the default foreground color style attribute to the style.
func (s Style) DefaultForegroundColor() Style {
	return append(s, defaultForegroundColorAttr)
}

// DefaultBackgroundColor appends the default background color style attribute to the style.
func (s Style) DefaultBackgroundColor() Style {
	return append(s, defaultBackgroundColorAttr)
}

// DefaultUnderlineColor appends the default underline color style attribute to the style.
func (s Style) DefaultUnderlineColor() Style {
	return append(s, defaultUnderlineColorAttr)
}

// ForegroundColor appends the foreground color style attribute to the style.
func (s Style) ForegroundColor(c Color) Style {
	return append(s, foregroundColorString(c))
}

// BackgroundColor appends the background color style attribute to the style.
func (s Style) BackgroundColor(c Color) Style {
	return append(s, backgroundColorString(c))
}

// UnderlineColor appends the underline color style attribute to the style.
func (s Style) UnderlineColor(c Color) Style {
	return append(s, underlineColorString(c))
}

// UnderlineStyle represents an ANSI SGR (Select Graphic Rendition) underline
// style.
type UnderlineStyle = byte

const (
	doubleUnderlineStyle = "4:2"
	curlyUnderlineStyle  = "4:3"
	dottedUnderlineStyle = "4:4"
	dashedUnderlineStyle = "4:5"
)

const (
	// NoUnderlineStyle is the default underline style.
	NoUnderlineStyle UnderlineStyle = iota
	// SingleUnderlineStyle is a single underline style.
	SingleUnderlineStyle
	// DoubleUnderlineStyle is a double underline style.
	DoubleUnderlineStyle
	// CurlyUnderlineStyle is a curly underline style.
	CurlyUnderlineStyle
	// DottedUnderlineStyle is a dotted underline style.
	DottedUnderlineStyle
	// DashedUnderlineStyle is a dashed underline style.
	DashedUnderlineStyle
)

// SGR (Select Graphic Rendition) style attributes.
// See: https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters
const (
	ResetAttr                        Attr = 0
	BoldAttr                         Attr = 1
	FaintAttr                        Attr = 2
	ItalicAttr                       Attr = 3
	UnderlineAttr                    Attr = 4
	SlowBlinkAttr                    Attr = 5
	RapidBlinkAttr                   Attr = 6
	ReverseAttr                      Attr = 7
	ConcealAttr                      Attr = 8
	StrikethroughAttr                Attr = 9
	NormalIntensityAttr              Attr = 22
	NoItalicAttr                     Attr = 23
	NoUnderlineAttr                  Attr = 24
	NoBlinkAttr                      Attr = 25
	NoReverseAttr                    Attr = 27
	NoConcealAttr                    Attr = 28
	NoStrikethroughAttr              Attr = 29
	BlackForegroundColorAttr         Attr = 30
	RedForegroundColorAttr           Attr = 31
	GreenForegroundColorAttr         Attr = 32
	YellowForegroundColorAttr        Attr = 33
	BlueForegroundColorAttr          Attr = 34
	MagentaForegroundColorAttr       Attr = 35
	CyanForegroundColorAttr          Attr = 36
	WhiteForegroundColorAttr         Attr = 37
	ExtendedForegroundColorAttr      Attr = 38
	DefaultForegroundColorAttr       Attr = 39
	BlackBackgroundColorAttr         Attr = 40
	RedBackgroundColorAttr           Attr = 41
	GreenBackgroundColorAttr         Attr = 42
	YellowBackgroundColorAttr        Attr = 43
	BlueBackgroundColorAttr          Attr = 44
	MagentaBackgroundColorAttr       Attr = 45
	CyanBackgroundColorAttr          Attr = 46
	WhiteBackgroundColorAttr         Attr = 47
	ExtendedBackgroundColorAttr      Attr = 48
	DefaultBackgroundColorAttr       Attr = 49
	ExtendedUnderlineColorAttr       Attr = 58
	DefaultUnderlineColorAttr        Attr = 59
	BrightBlackForegroundColorAttr   Attr = 90
	BrightRedForegroundColorAttr     Attr = 91
	BrightGreenForegroundColorAttr   Attr = 92
	BrightYellowForegroundColorAttr  Attr = 93
	BrightBlueForegroundColorAttr    Attr = 94
	BrightMagentaForegroundColorAttr Attr = 95
	BrightCyanForegroundColorAttr    Attr = 96
	BrightWhiteForegroundColorAttr   Attr = 97
	BrightBlackBackgroundColorAttr   Attr = 100
	BrightRedBackgroundColorAttr     Attr = 101
	BrightGreenBackgroundColorAttr   Attr = 102
	BrightYellowBackgroundColorAttr  Attr = 103
	BrightBlueBackgroundColorAttr    Attr = 104
	BrightMagentaBackgroundColorAttr Attr = 105
	BrightCyanBackgroundColorAttr    Attr = 106
	BrightWhiteBackgroundColorAttr   Attr = 107

	RGBColorIntroducerAttr      Attr = 2
	ExtendedColorIntroducerAttr Attr = 5
)

const (
	resetAttr                        = "0"
	boldAttr                         = "1"
	faintAttr                        = "2"
	italicAttr                       = "3"
	underlineAttr                    = "4"
	slowBlinkAttr                    = "5"
	rapidBlinkAttr                   = "6"
	reverseAttr                      = "7"
	concealAttr                      = "8"
	strikethroughAttr                = "9"
	normalIntensityAttr              = "22"
	noItalicAttr                     = "23"
	noUnderlineAttr                  = "24"
	noBlinkAttr                      = "25"
	noReverseAttr                    = "27"
	noConcealAttr                    = "28"
	noStrikethroughAttr              = "29"
	blackForegroundColorAttr         = "30"
	redForegroundColorAttr           = "31"
	greenForegroundColorAttr         = "32"
	yellowForegroundColorAttr        = "33"
	blueForegroundColorAttr          = "34"
	magentaForegroundColorAttr       = "35"
	cyanForegroundColorAttr          = "36"
	whiteForegroundColorAttr         = "37"
	extendedForegroundColorAttr      = "38"
	defaultForegroundColorAttr       = "39"
	blackBackgroundColorAttr         = "40"
	redBackgroundColorAttr           = "41"
	greenBackgroundColorAttr         = "42"
	yellowBackgroundColorAttr        = "43"
	blueBackgroundColorAttr          = "44"
	magentaBackgroundColorAttr       = "45"
	cyanBackgroundColorAttr          = "46"
	whiteBackgroundColorAttr         = "47"
	extendedBackgroundColorAttr      = "48"
	defaultBackgroundColorAttr       = "49"
	extendedUnderlineColorAttr       = "58"
	defaultUnderlineColorAttr        = "59"
	brightBlackForegroundColorAttr   = "90"
	brightRedForegroundColorAttr     = "91"
	brightGreenForegroundColorAttr   = "92"
	brightYellowForegroundColorAttr  = "93"
	brightBlueForegroundColorAttr    = "94"
	brightMagentaForegroundColorAttr = "95"
	brightCyanForegroundColorAttr    = "96"
	brightWhiteForegroundColorAttr   = "97"
	brightBlackBackgroundColorAttr   = "100"
	brightRedBackgroundColorAttr     = "101"
	brightGreenBackgroundColorAttr   = "102"
	brightYellowBackgroundColorAttr  = "103"
	brightBlueBackgroundColorAttr    = "104"
	brightMagentaBackgroundColorAttr = "105"
	brightCyanBackgroundColorAttr    = "106"
	brightWhiteBackgroundColorAttr   = "107"
)

// foregroundColorString returns the style SGR attribute for the given
// foreground color.
// See: https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters
func foregroundColorString(c Color) string {
	switch c := c.(type) {
	case BasicColor:
		// 3-bit or 4-bit ANSI foreground
		// "3<n>" or "9<n>" where n is the color number from 0 to 7
		switch c {
		case Black:
			return blackForegroundColorAttr
		case Red:
			return redForegroundColorAttr
		case Green:
			return greenForegroundColorAttr
		case Yellow:
			return yellowForegroundColorAttr
		case Blue:
			return blueForegroundColorAttr
		case Magenta:
			return magentaForegroundColorAttr
		case Cyan:
			return cyanForegroundColorAttr
		case White:
			return whiteForegroundColorAttr
		case BrightBlack:
			return brightBlackForegroundColorAttr
		case BrightRed:
			return brightRedForegroundColorAttr
		case BrightGreen:
			return brightGreenForegroundColorAttr
		case BrightYellow:
			return brightYellowForegroundColorAttr
		case BrightBlue:
			return brightBlueForegroundColorAttr
		case BrightMagenta:
			return brightMagentaForegroundColorAttr
		case BrightCyan:
			return brightCyanForegroundColorAttr
		case BrightWhite:
			return brightWhiteForegroundColorAttr
		}
	case ExtendedColor:
		// 256-color ANSI foreground
		// "38;5;<n>"
		return "38;5;" + strconv.FormatUint(uint64(c), 10)
	case TrueColor, color.Color:
		// 24-bit "true color" foreground
		// "38;2;<r>;<g>;<b>"
		r, g, b, _ := c.RGBA()
		return "38;2;" +
			strconv.FormatUint(uint64(shift(r)), 10) + ";" +
			strconv.FormatUint(uint64(shift(g)), 10) + ";" +
			strconv.FormatUint(uint64(shift(b)), 10)
	}
	return defaultForegroundColorAttr
}

// backgroundColorString returns the style SGR attribute for the given
// background color.
// See: https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters
func backgroundColorString(c Color) string {
	switch c := c.(type) {
	case BasicColor:
		// 3-bit or 4-bit ANSI foreground
		// "4<n>" or "10<n>" where n is the color number from 0 to 7
		switch c {
		case Black:
			return blackBackgroundColorAttr
		case Red:
			return redBackgroundColorAttr
		case Green:
			return greenBackgroundColorAttr
		case Yellow:
			return yellowBackgroundColorAttr
		case Blue:
			return blueBackgroundColorAttr
		case Magenta:
			return magentaBackgroundColorAttr
		case Cyan:
			return cyanBackgroundColorAttr
		case White:
			return whiteBackgroundColorAttr
		case BrightBlack:
			return brightBlackBackgroundColorAttr
		case BrightRed:
			return brightRedBackgroundColorAttr
		case BrightGreen:
			return brightGreenBackgroundColorAttr
		case BrightYellow:
			return brightYellowBackgroundColorAttr
		case BrightBlue:
			return brightBlueBackgroundColorAttr
		case BrightMagenta:
			return brightMagentaBackgroundColorAttr
		case BrightCyan:
			return brightCyanBackgroundColorAttr
		case BrightWhite:
			return brightWhiteBackgroundColorAttr
		}
	case ExtendedColor:
		// 256-color ANSI foreground
		// "48;5;<n>"
		return "48;5;" + strconv.FormatUint(uint64(c), 10)
	case TrueColor, color.Color:
		// 24-bit "true color" foreground
		// "38;2;<r>;<g>;<b>"
		r, g, b, _ := c.RGBA()
		return "48;2;" +
			strconv.FormatUint(uint64(shift(r)), 10) + ";" +
			strconv.FormatUint(uint64(shift(g)), 10) + ";" +
			strconv.FormatUint(uint64(shift(b)), 10)
	}
	return defaultBackgroundColorAttr
}

// underlineColorString returns the style SGR attribute for the given underline
// color.
// See: https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_(Select_Graphic_Rendition)_parameters
func underlineColorString(c Color) string {
	switch c := c.(type) {
	// NOTE: we can't use 3-bit and 4-bit ANSI color codes with underline
	// color, use 256-color instead.
	//
	// 256-color ANSI underline color
	// "58;5;<n>"
	case BasicColor:
		return "58;5;" + strconv.FormatUint(uint64(c), 10)
	case ExtendedColor:
		return "58;5;" + strconv.FormatUint(uint64(c), 10)
	case TrueColor, color.Color:
		// 24-bit "true color" foreground
		// "38;2;<r>;<g>;<b>"
		r, g, b, _ := c.RGBA()
		return "58;2;" +
			strconv.FormatUint(uint64(shift(r)), 10) + ";" +
			strconv.FormatUint(uint64(shift(g)), 10) + ";" +
			strconv.FormatUint(uint64(shift(b)), 10)
	}
	return defaultUnderlineColorAttr
}

// ReadStyleColor decodes a color from a slice of parameters. It returns the
// number of parameters read and the color. This function is used to read SGR
// color parameters following the ITU T.416 standard.
//
// It supports reading the following color types:
//   - 0: implementation defined
//   - 1: transparent
//   - 2: RGB direct color
//   - 3: CMY direct color
//   - 4: CMYK direct color
//   - 5: indexed color
//   - 6: RGBA direct color (WezTerm extension)
//
// The parameters can be separated by semicolons (;) or colons (:). Mixing
// separators is not allowed.
//
// The specs supports defining a color space id, a color tolerance value, and a
// tolerance color space id. However, these values have no effect on the
// returned color and will be ignored.
//
// This implementation includes a few modifications to the specs:
//  1. Support for legacy color values separated by semicolons (;) with respect to RGB, and indexed colors
//  2. Support ignoring and omitting the color space id (second parameter) with respect to RGB colors
//  3. Support ignoring and omitting the 6th parameter with respect to RGB and CMY colors
//  4. Support reading RGBA colors
func ReadStyleColor(params Params, co *color.Color) (n int) {
	if len(params) < 2 { // Need at least SGR type and color type
		return 0
	}

	// First parameter indicates one of 38, 48, or 58 (foreground, background, or underline)
	s := params[0]
	p := params[1]
	colorType := p.Param(0)
	n = 2

	paramsfn := func() (p1, p2, p3, p4 int) {
		// Where should we start reading the color?
		switch {
		case s.HasMore() && p.HasMore() && len(params) > 8 && params[2].HasMore() && params[3].HasMore() && params[4].HasMore() && params[5].HasMore() && params[6].HasMore() && params[7].HasMore():
			// We have color space id, a 6th parameter, a tolerance value, and a tolerance color space
			n += 7
			return params[3].Param(0), params[4].Param(0), params[5].Param(0), params[6].Param(0)
		case s.HasMore() && p.HasMore() && len(params) > 7 && params[2].HasMore() && params[3].HasMore() && params[4].HasMore() && params[5].HasMore() && params[6].HasMore():
			// We have color space id, a 6th parameter, and a tolerance value
			n += 6
			return params[3].Param(0), params[4].Param(0), params[5].Param(0), params[6].Param(0)
		case s.HasMore() && p.HasMore() && len(params) > 6 && params[2].HasMore() && params[3].HasMore() && params[4].HasMore() && params[5].HasMore():
			// We have color space id and a 6th parameter
			// 48 : 4 : : 1 : 2 : 3 :4
			n += 5
			return params[3].Param(0), params[4].Param(0), params[5].Param(0), params[6].Param(0)
		case s.HasMore() && p.HasMore() && len(params) > 5 && params[2].HasMore() && params[3].HasMore() && params[4].HasMore() && !params[5].HasMore():
			// We have color space
			// 48 : 3 : : 1 : 2 : 3
			n += 4
			return params[3].Param(0), params[4].Param(0), params[5].Param(0), -1
		case s.HasMore() && p.HasMore() && p.Param(0) == 2 && params[2].HasMore() && params[3].HasMore() && !params[4].HasMore():
			// We have color values separated by colons (:)
			// 48 : 2 : 1 : 2 : 3
			fallthrough
		case !s.HasMore() && !p.HasMore() && p.Param(0) == 2 && !params[2].HasMore() && !params[3].HasMore() && !params[4].HasMore():
			// Support legacy color values separated by semicolons (;)
			// 48 ; 2 ; 1 ; 2 ; 3
			n += 3
			return params[2].Param(0), params[3].Param(0), params[4].Param(0), -1
		}
		// Ambiguous SGR color
		return -1, -1, -1, -1
	}

	switch colorType {
	case 0: // implementation defined
		return 2
	case 1: // transparent
		*co = color.Transparent
		return 2
	case 2: // RGB direct color
		if len(params) < 5 {
			return 0
		}

		r, g, b, _ := paramsfn()
		if r == -1 || g == -1 || b == -1 {
			return 0
		}

		*co = color.RGBA{
			R: uint8(r), //nolint:gosec
			G: uint8(g), //nolint:gosec
			B: uint8(b), //nolint:gosec
			A: 0xff,
		}
		return //nolint:nakedret

	case 3: // CMY direct color
		if len(params) < 5 {
			return 0
		}

		c, m, y, _ := paramsfn()
		if c == -1 || m == -1 || y == -1 {
			return 0
		}

		*co = color.CMYK{
			C: uint8(c), //nolint:gosec
			M: uint8(m), //nolint:gosec
			Y: uint8(y), //nolint:gosec
			K: 0,
		}
		return //nolint:nakedret

	case 4: // CMYK direct color
		if len(params) < 6 {
			return 0
		}

		c, m, y, k := paramsfn()
		if c == -1 || m == -1 || y == -1 || k == -1 {
			return 0
		}

		*co = color.CMYK{
			C: uint8(c), //nolint:gosec
			M: uint8(m), //nolint:gosec
			Y: uint8(y), //nolint:gosec
			K: uint8(k), //nolint:gosec
		}
		return //nolint:nakedret

	case 5: // indexed color
		if len(params) < 3 {
			return 0
		}
		switch {
		case s.HasMore() && p.HasMore() && !params[2].HasMore():
			// Colon separated indexed color
			// 38 : 5 : 234
		case !s.HasMore() && !p.HasMore() && !params[2].HasMore():
			// Legacy semicolon indexed color
			// 38 ; 5 ; 234
		default:
			return 0
		}
		*co = ExtendedColor(params[2].Param(0)) //nolint:gosec
		return 3

	case 6: // RGBA direct color
		if len(params) < 6 {
			return 0
		}

		r, g, b, a := paramsfn()
		if r == -1 || g == -1 || b == -1 || a == -1 {
			return 0
		}

		*co = color.RGBA{
			R: uint8(r), //nolint:gosec
			G: uint8(g), //nolint:gosec
			B: uint8(b), //nolint:gosec
			A: uint8(a), //nolint:gosec
		}
		return //nolint:nakedret

	default:
		return 0
	}
}
