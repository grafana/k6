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
)

const defaultWidth = 40
const defaultBarColor = color.Faint

// ProgressBar is just a simple thread-safe progressbar implementation with
// callbacks.
type ProgressBar struct {
	mutex sync.RWMutex
	width int
	color *color.Color

	left     func() string
	progress func() (progress float64, right string)
	hijack   func() string
}

// ProgressBarOption is used for helper functions that modify the progressbar
// parameters, either in the constructor or via the Modify() method.
type ProgressBarOption func(*ProgressBar)

// WithLeft modifies the function that returns the left progressbar padding.
func WithLeft(left func() string) ProgressBarOption {
	return func(pb *ProgressBar) { pb.left = left }
}

// WithConstLeft sets the left progressbar padding to the supplied const.
func WithConstLeft(left string) ProgressBarOption {
	return func(pb *ProgressBar) {
		pb.left = func() string { return left }
	}
}

// WithProgress modifies the progress calculation function.
func WithProgress(progress func() (float64, string)) ProgressBarOption {
	return func(pb *ProgressBar) { pb.progress = progress }
}

// WithConstProgress sets the progress and right padding to the supplied consts.
func WithConstProgress(progress float64, right string) ProgressBarOption {
	return func(pb *ProgressBar) {
		pb.progress = func() (float64, string) { return progress, right }
	}
}

// WithHijack replaces the progressbar String function with the argument.
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

// Modify changes the progressbar options in a thread-safe way.
func (pb *ProgressBar) Modify(options ...ProgressBarOption) {
	pb.mutex.Lock()
	defer pb.mutex.Unlock()
	for _, option := range options {
		option(pb)
	}
}

// String locks the progressbar struct for reading and calls all of its methods
// to assemble the progress bar and return it as a string.
//TODO: something prettier? paddings, right-alignment of the left column, line trimming by terminal size
func (pb *ProgressBar) String() string {
	pb.mutex.RLock()
	defer pb.mutex.RUnlock()

	if pb.hijack != nil {
		return pb.hijack()
	}

	var left, right string
	if pb.left != nil {
		left = pb.left() + " "
	}

	var progress float64
	if pb.progress != nil {
		progress, right = pb.progress()
		right = " " + right
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

	return fmt.Sprintf("%s[%s%s%s]%s", left, filling, caret, padding, right)
}
