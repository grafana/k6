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
	"bufio"
	"fmt"
	"io"

	"github.com/fatih/color"
)

// A Field in a form.
type Field interface {
	GetKey() string   // Key for the data map.
	GetLabel() string // Label to print as the prompt.

	// Sanitize user input and return the field's native type.
	Clean(s string) (interface{}, error)
}

// A Form used to handle user interactions.
type Form struct {
	Banner string
	Fields []Field
}

// Runs the form against the specified input and output.
func (f Form) Run(r io.Reader, w io.Writer) (map[string]interface{}, error) {
	if f.Banner != "" {
		fmt.Fprintln(w, color.BlueString(f.Banner)+"\n")
	}

	buf := bufio.NewReader(r)
	data := make(map[string]interface{}, len(f.Fields))
	for _, field := range f.Fields {
		for {
			fmt.Fprintf(w, "  "+field.GetLabel()+": ")

			color.Set(color.FgCyan)
			s, err := buf.ReadString('\n')
			color.Unset()
			if err != nil {
				return nil, err
			}

			v, err := field.Clean(s)
			if err != nil {
				fmt.Fprintln(w, color.RedString("- "+err.Error()))
				continue
			}

			data[field.GetKey()] = v
			break
		}
	}

	return data, nil
}
