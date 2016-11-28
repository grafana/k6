package ui

import (
	"fmt"
	"strings"
)

type ProgressBar struct {
	Width    int
	Progress float64
}

func (b *ProgressBar) String() string {
	space := b.Width - 2
	filled := int(float64(space) * b.Progress)

	filling := ""
	caret := ""
	if filled > 0 {
		if filled < space {
			filling = strings.Repeat("=", filled-1)
			caret = ">"
		} else {
			filling = strings.Repeat("=", filled)
		}
	}

	padding := ""
	if space > filled {
		padding = strings.Repeat(" ", space-filled)
	}

	return fmt.Sprintf("[%s%s%s]", filling, caret, padding)
}
