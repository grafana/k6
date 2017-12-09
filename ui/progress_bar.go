/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
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

package ui

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

var (
	faint = color.New(color.Faint)
)

type ProgressBar struct {
	Width       int
	Progress    float64
	Left, Right func() string
}

func (b ProgressBar) String() string {
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
		padding = faint.Sprint(strings.Repeat("-", space-filled))
	}

	var left, right string
	if b.Left != nil {
		left = b.Left() + " "
	}
	if b.Right != nil {
		right = " " + b.Right()
	}
	return fmt.Sprintf("%s[%s%s%s]%s", left, filling, caret, padding, right)
}
