/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package pb

import (
	"fmt"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/sirupsen/logrus"
)

const defaultWidth = 40
const defaultBarColor = color.Faint

// Status of the progress bar
type Status string

// Progress bar status symbols
const (
	Running     Status = " "
	Waiting     Status = "•"
	Stopping    Status = "↓"
	Interrupted Status = "✗"
	Done        Status = "✓"
)

//nolint:gochecknoglobals
var statusColors = map[Status]*color.Color{
	Interrupted: color.New(color.FgRed),
	Done:        color.New(color.FgGreen),
}

// ProgressBar is a simple thread-safe progressbar implementation with
// callbacks.
type ProgressBar struct {
	mutex  sync.RWMutex
	width  int
	color  *color.Color
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
		width: defaultWidth,
		color: color.New(defaultBarColor),
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
// progress bar UI.
type ProgressBarRender struct {
	Left, Status, Progress, Hijack string
	Right                          []string
}

func (pbr ProgressBarRender) String() string {
	if pbr.Hijack != "" {
		return pbr.Hijack
	}
	var right string
	if len(pbr.Right) > 0 {
		right = " " + strings.Join(pbr.Right, "  ")
	}
	return fmt.Sprintf("%s %-1s %s%s",
		pbr.Left, pbr.Status, pbr.Progress, right)
}

// Render locks the progressbar struct for reading and calls all of
// its methods to return the final output. A struct is returned over a
// plain string to allow dynamic padding and positioning of elements
// depending on other elements on the screen.
// - leftMax defines the maximum character length of the left-side
//   text. Characters exceeding this length will be replaced with a
//   single ellipsis. Passing <=0 disables this.
func (pb *ProgressBar) Render(leftMax int) ProgressBarRender {
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

	padding := ""
	if space > filled {
		padding = pb.color.Sprint(strings.Repeat("-", space-filled))
	}

	out.Left = pb.renderLeft(leftMax)
	status := string(pb.status)
	if c, ok := statusColors[pb.status]; ok {
		status = c.Sprint(pb.status)
	}
	out.Status = fmt.Sprintf("%-1s", status)
	out.Progress = fmt.Sprintf("[%s%s%s]", filling, caret, padding)

	return out
}
