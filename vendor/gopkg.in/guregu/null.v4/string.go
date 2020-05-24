// Package null contains SQL types that consider zero input and null input as separate values,
// with convenient support for JSON and text marshaling.
// Types in this package will always encode to their null value if null.
// Use the zero subpackage if you want zero values and null to be treated the same.
package null

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
)

// nullBytes is a JSON null literal
var nullBytes = []byte("null")

// String is a nullable string. It supports SQL and JSON serialization.
// It will marshal to null if null. Blank string input will be considered null.
type String struct {
	sql.NullString
}

// StringFrom creates a new String that will never be blank.
func StringFrom(s string) String {
	return NewString(s, true)
}

// StringFromPtr creates a new String that be null if s is nil.
func StringFromPtr(s *string) String {
	if s == nil {
		return NewString("", false)
	}
	return NewString(*s, true)
}

// ValueOrZero returns the inner value if valid, otherwise zero.
func (s String) ValueOrZero() string {
	if !s.Valid {
		return ""
	}
	return s.String
}

// NewString creates a new String
func NewString(s string, valid bool) String {
	return String{
		NullString: sql.NullString{
			String: s,
			Valid:  valid,
		},
	}
}

// UnmarshalJSON implements json.Unmarshaler.
// It supports string and null input. Blank string input does not produce a null String.
func (s *String) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, nullBytes) {
		s.Valid = false
		return nil
	}

	if err := json.Unmarshal(data, &s.String); err != nil {
		return fmt.Errorf("null: couldn't unmarshal JSON: %w", err)
	}

	s.Valid = true
	return nil
}

// MarshalJSON implements json.Marshaler.
// It will encode null if this String is null.
func (s String) MarshalJSON() ([]byte, error) {
	if !s.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(s.String)
}

// MarshalText implements encoding.TextMarshaler.
// It will encode a blank string when this String is null.
func (s String) MarshalText() ([]byte, error) {
	if !s.Valid {
		return []byte{}, nil
	}
	return []byte(s.String), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
// It will unmarshal to a null String if the input is a blank string.
func (s *String) UnmarshalText(text []byte) error {
	s.String = string(text)
	s.Valid = s.String != ""
	return nil
}

// SetValid changes this String's value and also sets it to be non-null.
func (s *String) SetValid(v string) {
	s.String = v
	s.Valid = true
}

// Ptr returns a pointer to this String's value, or a nil pointer if this String is null.
func (s String) Ptr() *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

// IsZero returns true for null strings, for potential future omitempty support.
func (s String) IsZero() bool {
	return !s.Valid
}

// Equal returns true if both strings have the same value or are both null.
func (s String) Equal(other String) bool {
	return s.Valid == other.Valid && (!s.Valid || s.String == other.String)
}
