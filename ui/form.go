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
	Clean(s string) (string, error)
}

// A Form used to handle user interactions.
type Form struct {
	Banner string
	Fields []Field
}

// Run executes the form against the specified input and output.
func (f Form) Run(r io.Reader, w io.Writer) (map[string]string, error) {
	if f.Banner != "" {
		if _, err := fmt.Fprintln(w, color.BlueString(f.Banner)+"\n"); err != nil {
			return nil, err
		}
	}

	data := make(map[string]string, len(f.Fields))
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
