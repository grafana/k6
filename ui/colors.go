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
	"github.com/fatih/color"
)

var (
	StdColor   = color.New()            // Default color.
	ErrorColor = color.New(color.FgRed) // Errors.

	SuccColor     = color.New(color.FgGreen)             // Successful stuff.
	FailColor     = color.New(color.FgRed)               // Failed stuff.
	GrayColor     = color.New(color.Faint)               // Padding and disabled stuff.
	ValueColor    = color.New(color.FgCyan)              // Values of all kinds.
	ExtraColor    = color.New(color.FgCyan, color.Faint) // Extra annotations for values.
	ExtraKeyColor = color.New(color.Faint)               // Keys inside extra annotations.

	TypeColor    = color.New(color.FgYellow) // Syntax: Types.
	CommentColor = color.New(color.FgBlue)   // Syntax: Comments.
)
