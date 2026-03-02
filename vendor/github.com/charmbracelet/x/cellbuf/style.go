package cellbuf

import (
	"github.com/charmbracelet/colorprofile"
)

// Convert converts a style to respect the given color profile.
func ConvertStyle(s Style, p colorprofile.Profile) Style {
	switch p {
	case colorprofile.TrueColor:
		return s
	case colorprofile.Ascii:
		s.Fg = nil
		s.Bg = nil
		s.Ul = nil
	case colorprofile.NoTTY:
		return Style{}
	}

	if s.Fg != nil {
		s.Fg = p.Convert(s.Fg)
	}
	if s.Bg != nil {
		s.Bg = p.Convert(s.Bg)
	}
	if s.Ul != nil {
		s.Ul = p.Convert(s.Ul)
	}

	return s
}
