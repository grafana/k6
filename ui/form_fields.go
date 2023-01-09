package ui

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Verify that the fields implement the interface
var (
	_ Field = StringField{}
	_ Field = PasswordField{}
)

// StringField is just a simple field for reading cleartext strings
type StringField struct {
	Key     string
	Label   string
	Default string

	// Length constraints.
	Min, Max int
}

// GetKey returns the field's key
func (f StringField) GetKey() string {
	return f.Key
}

// GetLabel returns the field's label
func (f StringField) GetLabel() string {
	return f.Label
}

// GetLabelExtra returns the field's default value
func (f StringField) GetLabelExtra() string {
	return f.Default
}

// GetContents simply reads a string in cleartext from the supplied reader
// It's compllicated and doesn't use t he bufio utils because we can't read ahead
// of the newline and consume more of the stdin, because we'll mess up the next form field
func (f StringField) GetContents(r io.Reader) (string, error) {
	result := make([]byte, 0, 20)
	buf := make([]byte, 1)
	for {
		n, err := io.ReadAtLeast(r, buf, 1)
		if err != nil {
			return string(result), err
		}

		if n != 1 {
			// Shouldn't happen, but just in case
			return string(result), errors.New("unexpected input when reading string field")
		} else if buf[0] == '\n' {
			return string(result), nil
		}
		result = append(result, buf[0])
	}
}

// Clean trims the spaces in the string and checks for min and max length
func (f StringField) Clean(s string) (string, error) {
	s = strings.TrimSpace(s)
	if f.Min != 0 && len(s) < f.Min {
		return "", fmt.Errorf("invalid input, min length is %d", f.Min)
	}
	if f.Max != 0 && len(s) > f.Max {
		return "", fmt.Errorf("invalid input, max length is %d", f.Max)
	}
	if s == "" {
		s = f.Default
	}
	return s, nil
}

// PasswordField masks password input
type PasswordField struct {
	Key   string
	Label string
	Min   int
}

// GetKey returns the field's key
func (f PasswordField) GetKey() string {
	return f.Key
}

// GetLabel returns the field's label
func (f PasswordField) GetLabel() string {
	return f.Label
}

// GetLabelExtra doesn't return anything so we don't expose the current password
func (f PasswordField) GetLabelExtra() string {
	return ""
}

// GetContents simply reads a string in cleartext from the supplied reader
func (f PasswordField) GetContents(r io.Reader) (string, error) {
	stdin, ok := r.(*os.File)
	if !ok {
		return "", errors.New("cannot read password from the supplied terminal")
	}
	password, err := term.ReadPassword(int(stdin.Fd()))
	if err != nil {
		// Possibly running on Cygwin/mintty which doesn't emulate
		// pseudo terminals properly, so fallback to plain text input.
		// Note that passwords will be echoed if this is the case.
		// See https://github.com/mintty/mintty/issues/56
		// A workaround is to use winpty or mintty compiled with
		// Cygwin >=3.1.0 which supports the new ConPTY Windows API.
		bufR := bufio.NewReader(r)
		password, err = bufR.ReadBytes('\n')
	}
	return string(password), err
}

// Clean just checks if the minimum length is exceeded, it doesn't trim the string!
func (f PasswordField) Clean(s string) (string, error) {
	if f.Min != 0 && len(s) < f.Min {
		return "", fmt.Errorf("invalid input, min length is %d", f.Min)
	}
	return s, nil
}
