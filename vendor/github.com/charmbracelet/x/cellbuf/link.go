package cellbuf

import (
	"github.com/charmbracelet/colorprofile"
)

// Convert converts a hyperlink to respect the given color profile.
func ConvertLink(h Link, p colorprofile.Profile) Link {
	if p == colorprofile.NoTTY {
		return Link{}
	}

	return h
}
