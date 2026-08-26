// Package intstr implements a custom type for handling values that can be either a string or an int32
package intstr

import (
	"fmt"
	"strconv"
	"strings"
)

// ValueType defines the type of a IntOrString value
type ValueType int

const (
	// ValueTypeInt is a IntOrString that represents an integer value
	ValueTypeInt ValueType = iota
	// ValueTypeString is a IntOrString that represents a string value
	ValueTypeString
)

// IntOrString holds a value that can be either a string or a int
type IntOrString string

// NullValue is an empty IntOrString value
const NullValue = IntOrString("")

// Type returns the ValueType of a IntOrString value
func (value IntOrString) Type() ValueType {
	if _, err := strconv.Atoi(string(value)); err == nil {
		return ValueTypeInt
	}
	return ValueTypeString
}

// IsInt returns true if the value is an integer
func (value IntOrString) IsInt() bool {
	return value.Type() == ValueTypeInt
}

// AsPercentage return the value of a string transformed in a percentage and reports if
// this conversion was possible. '10%' return 10 and true, while "10", "foo" return 0 and false.
func (value IntOrString) AsPercentage() (int32, bool) {
	if value.Type() == ValueTypeInt {
		return 0, false
	}
	prefix, suffix, found := strings.Cut(value.Str(), "%")
	if !found || suffix != "" {
		return 0, false
	}

	if !IntOrString(prefix).IsInt() {
		return 0, false
	}

	return IntOrString(prefix).Int32(), true
}

// IsZero checks if the IntOrString value is an integer 0
func (value IntOrString) IsZero() bool {
	return value.IsInt() && value.Int32() == 0
}

// IsNull checks if the IntOrString value is the Int NullValue
func (value IntOrString) IsNull() bool {
	return value == NullValue
}

// Int32 returns the value of the IntOrString as an int32.
// If the current value is not an string, 0 is returned
func (value IntOrString) Int32() int32 {
	int64Value, err := strconv.ParseInt(string(value), 10, 32)
	if err != nil {
		panic(fmt.Errorf("invalid int32 value %s", value))
	}

	return int32(int64Value)
}

// Str returns the value of the IntOrString as a string.
func (value IntOrString) Str() string {
	return string(value)
}

// FromInt32 return a IntOrString from a int32
func FromInt32(value int32) IntOrString {
	strValue := fmt.Sprintf("%d", value)
	return IntOrString(strValue)
}

// FromString return a IntOrString from a string
func FromString(value string) IntOrString {
	return IntOrString(value)
}
