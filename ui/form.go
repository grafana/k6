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
	"io"

	"github.com/fatih/color"
)

// A Field in a form.
type Field interface {
	GetKey() string                        // Key for the data map.
	GetLabel() string                      // Label to print as the prompt.
	GetLabelExtra() string                 // Extra info for the label, eg. defaults.
	GetContents(io.Reader) (string, error) // Read the field contents from the supplied reader

	// Sanitize user input and return the field's native type.
	Clean(s string) (interface{}, error)
}

// A Form used to handle user interactions.
type Form struct {
	Banner string
	Fields []Field
}

// Run executes the form against the specified input and output.
func (f Form) Run(r io.Reader, w io.Writer) (map[string]interface{}, error) {
	if f.Banner != "" {
		if _, err := fmt.Fprintln(w, color.BlueString(f.Banner)+"\n"); err != nil {
			return nil, err
		}
	}

	data := make(map[string]interface{}, len(f.Fields))
	for _, field := range f.Fields {
		for {
			displayLabel := field.GetLabel()
			if extra := field.GetLabelExtra(); extra != "" {
				displayLabel += " " + color.New(color.Faint, color.FgCyan).Sprint("["+extra+"]")
			}
			if _, err := fmt.Fprintf(w, "  "+displayLabel+": "); err != nil {
				return nil, err
			}

			color.Set(color.FgCyan)
			s, err := field.GetContents(r)

			if _, ok := field.(PasswordField); ok {
				fmt.Fprint(w, "\n")
			}

			color.Unset()
			if err != nil {
				return nil, err
			}

			v, err := field.Clean(s)
			if err != nil {
				if _, printErr := fmt.Fprintln(w, color.RedString("- "+err.Error())); printErr != nil {
					return nil, printErr
				}
				continue
			}

			data[field.GetKey()] = v
			break
		}
	}

	return data, nil
}
