package pb

import (
	"fmt"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

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
	// DefaultWidth of the progress bar
	DefaultWidth = 40
	// threshold below which progress should be rendered as
	// percentages instead of filling bars
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

// ProgressBar is a simple thread-safe progressbar implementation with
// callbacks.
type ProgressBar struct {
	mutex  sync.RWMutex
	width  int
	logger *logrus.Entry
	status Status

	left     func() string
	progress func() (progress float64, right []string)
	hijack   func() string
}

// ProgressBarOption is used for helper functions that modify the progressbar
// parameters, either in the constructor or via the Modify() method.
type ProgressBarOption func(*ProgressBar)

// WithLeft modifies the function that returns the left progressbar value.
func WithLeft(left func() string) ProgressBarOption {
	return func(pb *ProgressBar) { pb.left = left }
}

// WithConstLeft sets the left progressbar value to the supplied const.
func WithConstLeft(left string) ProgressBarOption {
	return func(pb *ProgressBar) {
		pb.left = func() string { return left }
	}
}

// WithLogger modifies the logger instance
func WithLogger(logger *logrus.Entry) ProgressBarOption {
	return func(pb *ProgressBar) { pb.logger = logger }
}

// WithProgress modifies the progress calculation function.
func WithProgress(progress func() (float64, []string)) ProgressBarOption {
	return func(pb *ProgressBar) { pb.progress = progress }
}

// WithStatus modifies the progressbar status
func WithStatus(status Status) ProgressBarOption {
	return func(pb *ProgressBar) { pb.status = status }
}

// WithConstProgress sets the progress and right values to the supplied consts.
func WithConstProgress(progress float64, right ...string) ProgressBarOption {
	return func(pb *ProgressBar) {
		pb.progress = func() (float64, []string) { return progress, right }
	}
}

// WithHijack replaces the progressbar Render function with the argument.
func WithHijack(hijack func() string) ProgressBarOption {
	return func(pb *ProgressBar) { pb.hijack = hijack }
}

// New creates and initializes a new ProgressBar struct, calling all of the
// supplied options
func New(options ...ProgressBarOption) *ProgressBar {
	pb := &ProgressBar{
		mutex: sync.RWMutex{},
		width: DefaultWidth,
	}
	pb.Modify(options...)
	return pb
}

// Left returns the left part of the progressbar in a thread-safe way.
func (pb *ProgressBar) Left() string {
	pb.mutex.RLock()
	defer pb.mutex.RUnlock()

	return pb.renderLeft(0)
}

// renderLeft renders the left part of the progressbar, replacing text
// exceeding maxLen with an ellipsis.
func (pb *ProgressBar) renderLeft(maxLen int) string {
	var left string
	if pb.left != nil {
		l := pb.left()
		if maxLen > 0 && len(l) > maxLen {
			l = l[:maxLen-3] + "..."
		}
		left = l
	}
	return left
}

// Modify changes the progressbar options in a thread-safe way.
func (pb *ProgressBar) Modify(options ...ProgressBarOption) {
	pb.mutex.Lock()
	defer pb.mutex.Unlock()
	for _, option := range options {
		option(pb)
	}
}

// ProgressBarRender stores the different rendered parts of the
// progress bar UI to allow dynamic positioning and padding of
// elements in the terminal output (e.g. for responsive progress
// bars).
type ProgressBarRender struct {
	Right                                   []string
	progress, progressFill, progressPadding string
	Left, Hijack                            string
	status                                  Status
	Color                                   bool
}

// Status returns an optionally colorized status string
func (pbr *ProgressBarRender) Status() string {
	status := " "

	if pbr.status > 0 {
		status = string(pbr.status)
		if c, ok := statusColors[pbr.status]; pbr.Color && ok {
			status = c.Sprint(status)
		}
	}

	return status
}

// Progress returns an assembled and optionally colorized progress string
func (pbr *ProgressBarRender) Progress() string {
	var body string
	if pbr.progress != "" {
		body = fmt.Sprintf(" %s ", pbr.progress)
	} else {
		padding := pbr.progressPadding
		if pbr.Color {
			padding = colorFaint.Sprint(pbr.progressPadding)
		}
		body = pbr.progressFill + padding
	}
	return fmt.Sprintf("[%s]", body)
}

func (pbr ProgressBarRender) String() string {
	if pbr.Hijack != "" {
		return pbr.Hijack
	}
	var right string
	if len(pbr.Right) > 0 {
		right = " " + strings.Join(pbr.Right, "  ")
	}
	return pbr.Left + " " + pbr.Status() + " " + pbr.Progress() + right
}

// Render locks the progressbar struct for reading and calls all of
// its methods to return the final output. A struct is returned over a
// plain string to allow dynamic padding and positioning of elements
// depending on other elements on the screen.
//   - maxLeft defines the maximum character length of the left-side
//     text. Characters exceeding this length will be replaced with a
//     single ellipsis. Passing <=0 disables this.
//   - widthDelta changes the progress bar width the specified amount of
//     characters. E.g. passing -2 would shorten the width by 2 chars.
//     If the resulting width is lower than minWidth, progress will be
//     rendered as a percentage instead of a filling bar.
func (pb *ProgressBar) Render(maxLeft, widthDelta int) ProgressBarRender {
	pb.mutex.RLock()
	defer pb.mutex.RUnlock()

	var out ProgressBarRender
	if pb.hijack != nil {
		out.Hijack = pb.hijack()
		return out
	}

	var progress float64
	if pb.progress != nil {
		progress, out.Right = pb.progress()
		progressClamped := Clampf(progress, 0, 1)
		if progress != progressClamped {
			progress = progressClamped
			if pb.logger != nil {
				pb.logger.Warnf("progress value %.2f exceeds valid range, clamped between 0 and 1", progress)
			}
		}
	}

	width := Clampf(float64(pb.width+widthDelta), minWidth, DefaultWidth)
	pb.width = int(width)

	if pb.width > minWidth { //nolint:nestif
		space := pb.width - 2
		filled := int(float64(space) * progress)

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

		out.progressPadding = ""
		if space > filled {
			out.progressPadding = strings.Repeat("-", space-filled)
		}

		out.progressFill = filling + caret
	} else {
		out.progress = fmt.Sprintf("%3.f%%", progress*100)
	}

	out.Left = pb.renderLeft(maxLeft)
	out.status = pb.status

	return out
}
