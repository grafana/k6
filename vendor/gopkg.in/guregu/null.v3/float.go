package null

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
)

// Float is a nullable float64.
// It does not consider zero values to be null.
// It will decode to null, not zero, if null.
type Float struct {
	sql.NullFloat64
}

// NewFloat creates a new Float
func NewFloat(f float64, valid bool) Float {
	return Float{
		NullFloat64: sql.NullFloat64{
			Float64: f,
			Valid:   valid,
		},
	}
}

// FloatFrom creates a new Float that will always be valid.
func FloatFrom(f float64) Float {
	return NewFloat(f, true)
}

// FloatFromPtr creates a new Float that be null if f is nil.
func FloatFromPtr(f *float64) Float {
	if f == nil {
		return NewFloat(0, false)
	}
	return NewFloat(*f, true)
}

// ValueOrZero returns the inner value if valid, otherwise zero.
func (f Float) ValueOrZero() float64 {
	if !f.Valid {
		return 0
	}
	return f.Float64
}

// UnmarshalJSON implements json.Unmarshaler.
// It supports number and null input.
// 0 will not be considered a null Float.
// It also supports unmarshalling a sql.NullFloat64.
func (f *Float) UnmarshalJSON(data []byte) error {
	var err error
	var v interface{}
	if err = json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case float64:
		f.Float64 = float64(x)
	case string:
		str := string(x)
		if len(str) == 0 {
			f.Valid = false
			return nil
		}
		f.Float64, err = strconv.ParseFloat(str, 64)
	case map[string]interface{}:
		err = json.Unmarshal(data, &f.NullFloat64)
	case nil:
		f.Valid = false
		return nil
	default:
		err = fmt.Errorf("json: cannot unmarshal %v into Go value of type null.Float", reflect.TypeOf(v).Name())
	}
	f.Valid = err == nil
	return err
}

// UnmarshalText implements encoding.TextUnmarshaler.
// It will unmarshal to a null Float if the input is a blank or not an integer.
// It will return an error if the input is not an integer, blank, or "null".
func (f *Float) UnmarshalText(text []byte) error {
	str := string(text)
	if str == "" || str == "null" {
		f.Valid = false
		return nil
	}
	var err error
	f.Float64, err = strconv.ParseFloat(string(text), 64)
	f.Valid = err == nil
	return err
}

// MarshalJSON implements json.Marshaler.
// It will encode null if this Float is null.
func (f Float) MarshalJSON() ([]byte, error) {
	if !f.Valid {
		return []byte("null"), nil
	}
	if math.IsInf(f.Float64, 0) || math.IsNaN(f.Float64) {
		return nil, &json.UnsupportedValueError{
			Value: reflect.ValueOf(f.Float64),
			Str:   strconv.FormatFloat(f.Float64, 'g', -1, 64),
		}
	}
	return []byte(strconv.FormatFloat(f.Float64, 'f', -1, 64)), nil
}

// MarshalText implements encoding.TextMarshaler.
// It will encode a blank string if this Float is null.
func (f Float) MarshalText() ([]byte, error) {
	if !f.Valid {
		return []byte{}, nil
	}
	return []byte(strconv.FormatFloat(f.Float64, 'f', -1, 64)), nil
}

// SetValid changes this Float's value and also sets it to be non-null.
func (f *Float) SetValid(n float64) {
	f.Float64 = n
	f.Valid = true
}

// Ptr returns a pointer to this Float's value, or a nil pointer if this Float is null.
func (f Float) Ptr() *float64 {
	if !f.Valid {
		return nil
	}
	return &f.Float64
}

// IsZero returns true for invalid Floats, for future omitempty support (Go 1.4?)
// A non-null Float with a 0 value will not be considered zero.
func (f Float) IsZero() bool {
	return !f.Valid
}

// Equal returns true if both floats have the same value or are both null.
// Warning: calculations using floating point numbers can result in different ways
// the numbers are stored in memory. Therefore, this function is not suitable to
// compare the result of a calculation. Use this method only to check if the value
// has changed in comparison to some previous value.
func (f Float) Equal(other Float) bool {
	return f.Valid == other.Valid && (!f.Valid || f.Float64 == other.Float64)
}
