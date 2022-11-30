package pb

import "github.com/fatih/color"

//nolint:gochecknoglobals
var (
	colorFaint   = color.New(color.Faint)
	statusColors = map[Status]*color.Color{
		Interrupted: color.New(color.FgRed),
		Done:        color.New(color.FgGreen),
		Waiting:     colorFaint,
	}
)

const (
	// DefaultWidth of the progress bar.
	DefaultWidth = 40
	// Threshold below which progress should be rendered as
	// percentages instead of filling bars.
	minWidth = 8
)

// Status of the progress bar
type Status rune

// Progress bar status symbols
const (
	Running     Status = ' '
	Waiting     Status = '•'
	Stopping    Status = '↓'
	Interrupted Status = '✗'
	Done        Status = '✓'
)
