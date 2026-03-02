package cellbuf

import (
	"bytes"
	"image/color"

	"github.com/charmbracelet/x/ansi"
)

// ReadStyle reads a Select Graphic Rendition (SGR) escape sequences from a
// list of parameters.
func ReadStyle(params ansi.Params, pen *Style) {
	if len(params) == 0 {
		pen.Reset()
		return
	}

	for i := 0; i < len(params); i++ {
		param, hasMore, _ := params.Param(i, 0)
		switch param {
		case 0: // Reset
			pen.Reset()
		case 1: // Bold
			pen.Bold(true)
		case 2: // Dim/Faint
			pen.Faint(true)
		case 3: // Italic
			pen.Italic(true)
		case 4: // Underline
			nextParam, _, ok := params.Param(i+1, 0)
			if hasMore && ok { // Only accept subparameters i.e. separated by ":"
				switch nextParam {
				case 0, 1, 2, 3, 4, 5:
					i++
					switch nextParam {
					case 0: // No Underline
						pen.UnderlineStyle(NoUnderline)
					case 1: // Single Underline
						pen.UnderlineStyle(SingleUnderline)
					case 2: // Double Underline
						pen.UnderlineStyle(DoubleUnderline)
					case 3: // Curly Underline
						pen.UnderlineStyle(CurlyUnderline)
					case 4: // Dotted Underline
						pen.UnderlineStyle(DottedUnderline)
					case 5: // Dashed Underline
						pen.UnderlineStyle(DashedUnderline)
					}
				}
			} else {
				// Single Underline
				pen.Underline(true)
			}
		case 5: // Slow Blink
			pen.SlowBlink(true)
		case 6: // Rapid Blink
			pen.RapidBlink(true)
		case 7: // Reverse
			pen.Reverse(true)
		case 8: // Conceal
			pen.Conceal(true)
		case 9: // Crossed-out/Strikethrough
			pen.Strikethrough(true)
		case 22: // Normal Intensity (not bold or faint)
			pen.Bold(false).Faint(false)
		case 23: // Not italic, not Fraktur
			pen.Italic(false)
		case 24: // Not underlined
			pen.Underline(false)
		case 25: // Blink off
			pen.SlowBlink(false).RapidBlink(false)
		case 27: // Positive (not reverse)
			pen.Reverse(false)
		case 28: // Reveal
			pen.Conceal(false)
		case 29: // Not crossed out
			pen.Strikethrough(false)
		case 30, 31, 32, 33, 34, 35, 36, 37: // Set foreground
			pen.Foreground(ansi.Black + ansi.BasicColor(param-30)) //nolint:gosec
		case 38: // Set foreground 256 or truecolor
			var c color.Color
			n := ReadStyleColor(params[i:], &c)
			if n > 0 {
				pen.Foreground(c)
				i += n - 1
			}
		case 39: // Default foreground
			pen.Foreground(nil)
		case 40, 41, 42, 43, 44, 45, 46, 47: // Set background
			pen.Background(ansi.Black + ansi.BasicColor(param-40)) //nolint:gosec
		case 48: // Set background 256 or truecolor
			var c color.Color
			n := ReadStyleColor(params[i:], &c)
			if n > 0 {
				pen.Background(c)
				i += n - 1
			}
		case 49: // Default Background
			pen.Background(nil)
		case 58: // Set underline color
			var c color.Color
			n := ReadStyleColor(params[i:], &c)
			if n > 0 {
				pen.UnderlineColor(c)
				i += n - 1
			}
		case 59: // Default underline color
			pen.UnderlineColor(nil)
		case 90, 91, 92, 93, 94, 95, 96, 97: // Set bright foreground
			pen.Foreground(ansi.BrightBlack + ansi.BasicColor(param-90)) //nolint:gosec
		case 100, 101, 102, 103, 104, 105, 106, 107: // Set bright background
			pen.Background(ansi.BrightBlack + ansi.BasicColor(param-100)) //nolint:gosec
		}
	}
}

// ReadLink reads a hyperlink escape sequence from a data buffer.
func ReadLink(p []byte, link *Link) {
	params := bytes.Split(p, []byte{';'})
	if len(params) != 3 {
		return
	}
	link.Params = string(params[1])
	link.URL = string(params[2])
}

// ReadStyleColor reads a color from a list of parameters.
// See [ansi.ReadStyleColor] for more information.
func ReadStyleColor(params ansi.Params, c *color.Color) int {
	return ansi.ReadStyleColor(params, c)
}
