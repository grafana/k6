package cellbuf

import (
	"github.com/charmbracelet/x/ansi"
)

var (
	// BlankCell is a cell with a single space, width of 1, and no style or link.
	BlankCell = Cell{Rune: ' ', Width: 1}

	// EmptyCell is just an empty cell used for comparisons and as a placeholder
	// for wide cells.
	EmptyCell = Cell{}
)

// Cell represents a single cell in the terminal screen.
type Cell struct {
	// The style of the cell. Nil style means no style. Zero value prints a
	// reset sequence.
	Style Style

	// Link is the hyperlink of the cell.
	Link Link

	// Comb is the combining runes of the cell. This is nil if the cell is a
	// single rune or if it's a zero width cell that is part of a wider cell.
	Comb []rune

	// Width is the mono-space width of the grapheme cluster.
	Width int

	// Rune is the main rune of the cell. This is zero if the cell is part of a
	// wider cell.
	Rune rune
}

// Append appends runes to the cell without changing the width. This is useful
// when we want to use the cell to store escape sequences or other runes that
// don't affect the width of the cell.
func (c *Cell) Append(r ...rune) {
	for i, r := range r {
		if i == 0 && c.Rune == 0 {
			c.Rune = r
			continue
		}
		c.Comb = append(c.Comb, r)
	}
}

// String returns the string content of the cell excluding any styles, links,
// and escape sequences.
func (c Cell) String() string {
	if c.Rune == 0 {
		return ""
	}
	if len(c.Comb) == 0 {
		return string(c.Rune)
	}
	return string(append([]rune{c.Rune}, c.Comb...))
}

// Equal returns whether the cell is equal to the other cell.
func (c *Cell) Equal(o *Cell) bool {
	return o != nil &&
		c.Width == o.Width &&
		c.Rune == o.Rune &&
		runesEqual(c.Comb, o.Comb) &&
		c.Style.Equal(&o.Style) &&
		c.Link.Equal(&o.Link)
}

// Empty returns whether the cell is an empty cell. An empty cell is a cell
// with a width of 0, a rune of 0, and no combining runes.
func (c Cell) Empty() bool {
	return c.Width == 0 &&
		c.Rune == 0 &&
		len(c.Comb) == 0
}

// Reset resets the cell to the default state zero value.
func (c *Cell) Reset() {
	c.Rune = 0
	c.Comb = nil
	c.Width = 0
	c.Style.Reset()
	c.Link.Reset()
}

// Clear returns whether the cell consists of only attributes that don't
// affect appearance of a space character.
func (c *Cell) Clear() bool {
	return c.Rune == ' ' && len(c.Comb) == 0 && c.Width == 1 && c.Style.Clear() && c.Link.Empty()
}

// Clone returns a copy of the cell.
func (c *Cell) Clone() (n *Cell) {
	n = new(Cell)
	*n = *c
	return
}

// Blank makes the cell a blank cell by setting the rune to a space, comb to
// nil, and the width to 1.
func (c *Cell) Blank() *Cell {
	c.Rune = ' '
	c.Comb = nil
	c.Width = 1
	return c
}

// Link represents a hyperlink in the terminal screen.
type Link struct {
	URL    string
	Params string
}

// String returns a string representation of the hyperlink.
func (h Link) String() string {
	return h.URL
}

// Reset resets the hyperlink to the default state zero value.
func (h *Link) Reset() {
	h.URL = ""
	h.Params = ""
}

// Equal returns whether the hyperlink is equal to the other hyperlink.
func (h *Link) Equal(o *Link) bool {
	return o != nil && h.URL == o.URL && h.Params == o.Params
}

// Empty returns whether the hyperlink is empty.
func (h Link) Empty() bool {
	return h.URL == "" && h.Params == ""
}

// AttrMask is a bitmask for text attributes that can change the look of text.
// These attributes can be combined to create different styles.
type AttrMask uint8

// These are the available text attributes that can be combined to create
// different styles.
const (
	BoldAttr AttrMask = 1 << iota
	FaintAttr
	ItalicAttr
	SlowBlinkAttr
	RapidBlinkAttr
	ReverseAttr
	ConcealAttr
	StrikethroughAttr

	ResetAttr AttrMask = 0
)

// UnderlineStyle is the style of underline to use for text.
type UnderlineStyle = ansi.UnderlineStyle

// These are the available underline styles.
const (
	NoUnderline     = ansi.NoUnderlineStyle
	SingleUnderline = ansi.SingleUnderlineStyle
	DoubleUnderline = ansi.DoubleUnderlineStyle
	CurlyUnderline  = ansi.CurlyUnderlineStyle
	DottedUnderline = ansi.DottedUnderlineStyle
	DashedUnderline = ansi.DashedUnderlineStyle
)

// Style represents the Style of a cell.
type Style struct {
	Fg      ansi.Color
	Bg      ansi.Color
	Ul      ansi.Color
	Attrs   AttrMask
	UlStyle UnderlineStyle
}

// Sequence returns the ANSI sequence that sets the style.
func (s Style) Sequence() string {
	if s.Empty() {
		return ansi.ResetStyle
	}

	var b ansi.Style

	if s.Attrs != 0 {
		if s.Attrs&BoldAttr != 0 {
			b = b.Bold()
		}
		if s.Attrs&FaintAttr != 0 {
			b = b.Faint()
		}
		if s.Attrs&ItalicAttr != 0 {
			b = b.Italic()
		}
		if s.Attrs&SlowBlinkAttr != 0 {
			b = b.SlowBlink()
		}
		if s.Attrs&RapidBlinkAttr != 0 {
			b = b.RapidBlink()
		}
		if s.Attrs&ReverseAttr != 0 {
			b = b.Reverse()
		}
		if s.Attrs&ConcealAttr != 0 {
			b = b.Conceal()
		}
		if s.Attrs&StrikethroughAttr != 0 {
			b = b.Strikethrough()
		}
	}
	if s.UlStyle != NoUnderline {
		switch s.UlStyle {
		case SingleUnderline:
			b = b.Underline()
		case DoubleUnderline:
			b = b.DoubleUnderline()
		case CurlyUnderline:
			b = b.CurlyUnderline()
		case DottedUnderline:
			b = b.DottedUnderline()
		case DashedUnderline:
			b = b.DashedUnderline()
		}
	}
	if s.Fg != nil {
		b = b.ForegroundColor(s.Fg)
	}
	if s.Bg != nil {
		b = b.BackgroundColor(s.Bg)
	}
	if s.Ul != nil {
		b = b.UnderlineColor(s.Ul)
	}

	return b.String()
}

// DiffSequence returns the ANSI sequence that sets the style as a diff from
// another style.
func (s Style) DiffSequence(o Style) string {
	if o.Empty() {
		return s.Sequence()
	}

	var b ansi.Style

	if !colorEqual(s.Fg, o.Fg) {
		b = b.ForegroundColor(s.Fg)
	}

	if !colorEqual(s.Bg, o.Bg) {
		b = b.BackgroundColor(s.Bg)
	}

	if !colorEqual(s.Ul, o.Ul) {
		b = b.UnderlineColor(s.Ul)
	}

	var (
		noBlink  bool
		isNormal bool
	)

	if s.Attrs != o.Attrs {
		if s.Attrs&BoldAttr != o.Attrs&BoldAttr {
			if s.Attrs&BoldAttr != 0 {
				b = b.Bold()
			} else if !isNormal {
				isNormal = true
				b = b.NormalIntensity()
			}
		}
		if s.Attrs&FaintAttr != o.Attrs&FaintAttr {
			if s.Attrs&FaintAttr != 0 {
				b = b.Faint()
			} else if !isNormal {
				b = b.NormalIntensity()
			}
		}
		if s.Attrs&ItalicAttr != o.Attrs&ItalicAttr {
			if s.Attrs&ItalicAttr != 0 {
				b = b.Italic()
			} else {
				b = b.NoItalic()
			}
		}
		if s.Attrs&SlowBlinkAttr != o.Attrs&SlowBlinkAttr {
			if s.Attrs&SlowBlinkAttr != 0 {
				b = b.SlowBlink()
			} else if !noBlink {
				noBlink = true
				b = b.NoBlink()
			}
		}
		if s.Attrs&RapidBlinkAttr != o.Attrs&RapidBlinkAttr {
			if s.Attrs&RapidBlinkAttr != 0 {
				b = b.RapidBlink()
			} else if !noBlink {
				b = b.NoBlink()
			}
		}
		if s.Attrs&ReverseAttr != o.Attrs&ReverseAttr {
			if s.Attrs&ReverseAttr != 0 {
				b = b.Reverse()
			} else {
				b = b.NoReverse()
			}
		}
		if s.Attrs&ConcealAttr != o.Attrs&ConcealAttr {
			if s.Attrs&ConcealAttr != 0 {
				b = b.Conceal()
			} else {
				b = b.NoConceal()
			}
		}
		if s.Attrs&StrikethroughAttr != o.Attrs&StrikethroughAttr {
			if s.Attrs&StrikethroughAttr != 0 {
				b = b.Strikethrough()
			} else {
				b = b.NoStrikethrough()
			}
		}
	}

	if s.UlStyle != o.UlStyle {
		b = b.UnderlineStyle(s.UlStyle)
	}

	return b.String()
}

// Equal returns true if the style is equal to the other style.
func (s *Style) Equal(o *Style) bool {
	return s.Attrs == o.Attrs &&
		s.UlStyle == o.UlStyle &&
		colorEqual(s.Fg, o.Fg) &&
		colorEqual(s.Bg, o.Bg) &&
		colorEqual(s.Ul, o.Ul)
}

func colorEqual(c, o ansi.Color) bool {
	if c == nil && o == nil {
		return true
	}
	if c == nil || o == nil {
		return false
	}
	cr, cg, cb, ca := c.RGBA()
	or, og, ob, oa := o.RGBA()
	return cr == or && cg == og && cb == ob && ca == oa
}

// Bold sets the bold attribute.
func (s *Style) Bold(v bool) *Style {
	if v {
		s.Attrs |= BoldAttr
	} else {
		s.Attrs &^= BoldAttr
	}
	return s
}

// Faint sets the faint attribute.
func (s *Style) Faint(v bool) *Style {
	if v {
		s.Attrs |= FaintAttr
	} else {
		s.Attrs &^= FaintAttr
	}
	return s
}

// Italic sets the italic attribute.
func (s *Style) Italic(v bool) *Style {
	if v {
		s.Attrs |= ItalicAttr
	} else {
		s.Attrs &^= ItalicAttr
	}
	return s
}

// SlowBlink sets the slow blink attribute.
func (s *Style) SlowBlink(v bool) *Style {
	if v {
		s.Attrs |= SlowBlinkAttr
	} else {
		s.Attrs &^= SlowBlinkAttr
	}
	return s
}

// RapidBlink sets the rapid blink attribute.
func (s *Style) RapidBlink(v bool) *Style {
	if v {
		s.Attrs |= RapidBlinkAttr
	} else {
		s.Attrs &^= RapidBlinkAttr
	}
	return s
}

// Reverse sets the reverse attribute.
func (s *Style) Reverse(v bool) *Style {
	if v {
		s.Attrs |= ReverseAttr
	} else {
		s.Attrs &^= ReverseAttr
	}
	return s
}

// Conceal sets the conceal attribute.
func (s *Style) Conceal(v bool) *Style {
	if v {
		s.Attrs |= ConcealAttr
	} else {
		s.Attrs &^= ConcealAttr
	}
	return s
}

// Strikethrough sets the strikethrough attribute.
func (s *Style) Strikethrough(v bool) *Style {
	if v {
		s.Attrs |= StrikethroughAttr
	} else {
		s.Attrs &^= StrikethroughAttr
	}
	return s
}

// UnderlineStyle sets the underline style.
func (s *Style) UnderlineStyle(style UnderlineStyle) *Style {
	s.UlStyle = style
	return s
}

// Underline sets the underline attribute.
// This is a syntactic sugar for [UnderlineStyle].
func (s *Style) Underline(v bool) *Style {
	if v {
		return s.UnderlineStyle(SingleUnderline)
	}
	return s.UnderlineStyle(NoUnderline)
}

// Foreground sets the foreground color.
func (s *Style) Foreground(c ansi.Color) *Style {
	s.Fg = c
	return s
}

// Background sets the background color.
func (s *Style) Background(c ansi.Color) *Style {
	s.Bg = c
	return s
}

// UnderlineColor sets the underline color.
func (s *Style) UnderlineColor(c ansi.Color) *Style {
	s.Ul = c
	return s
}

// Reset resets the style to default.
func (s *Style) Reset() *Style {
	s.Fg = nil
	s.Bg = nil
	s.Ul = nil
	s.Attrs = ResetAttr
	s.UlStyle = NoUnderline
	return s
}

// Empty returns true if the style is empty.
func (s *Style) Empty() bool {
	return s.Fg == nil && s.Bg == nil && s.Ul == nil && s.Attrs == ResetAttr && s.UlStyle == NoUnderline
}

// Clear returns whether the style consists of only attributes that don't
// affect appearance of a space character.
func (s *Style) Clear() bool {
	return s.UlStyle == NoUnderline &&
		s.Attrs&^(BoldAttr|FaintAttr|ItalicAttr|SlowBlinkAttr|RapidBlinkAttr) == 0 &&
		s.Fg == nil &&
		s.Bg == nil &&
		s.Ul == nil
}

func runesEqual(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}
	for i, r := range a {
		if r != b[i] {
			return false
		}
	}
	return true
}
