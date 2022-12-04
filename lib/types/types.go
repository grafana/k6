// Package types contains types used in the codebase
// Most of the types have a Null prefix like gopkg.in/guregu/null.v3
// and UnmarshalJSON and MarshalJSON methods.
package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// TODO: something better that won't require so much boilerplate and casts for NullDuration values...

// Duration is an alias for time.Duration that de/serialises to JSON as human-readable strings.
type Duration time.Duration

func (d Duration) String() string {
	return time.Duration(d).String()
}

// ParseExtendedDuration is a helper function that allows for string duration
// values containing days.
func ParseExtendedDuration(data string) (result time.Duration, err error) {
	// Assume millisecond values if data is provided with no units
	if t, errp := strconv.ParseFloat(data, 64); errp == nil {
		return time.Duration(t * float64(time.Millisecond)), nil
	}

	dPos := strings.IndexByte(data, 'd')
	if dPos < 0 {
		return time.ParseDuration(data)
	}

	var hours time.Duration
	if dPos+1 < len(data) { // case "12d"
		hours, err = time.ParseDuration(data[dPos+1:])
		if err != nil {
			return
		}
		if hours < 0 {
			return 0, fmt.Errorf("invalid time format '%s'", data[dPos+1:])
		}
	}

	days, err := strconv.ParseInt(data[:dPos], 10, 64)
	if err != nil {
		return
	}
	if days < 0 {
		hours = -hours
	}
	return time.Duration(days)*24*time.Hour + hours, nil
}

// UnmarshalText converts text data to Duration
func (d *Duration) UnmarshalText(data []byte) error {
	v, err := ParseExtendedDuration(string(data))
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

// UnmarshalJSON converts JSON data to Duration
func (d *Duration) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}

		v, err := ParseExtendedDuration(str)
		if err != nil {
			return err
		}

		*d = Duration(v)
	} else if t, errp := strconv.ParseFloat(string(data), 64); errp == nil {
		*d = Duration(t * float64(time.Millisecond))
	} else {
		return fmt.Errorf("'%s' is not a valid duration value", string(data))
	}

	return nil
}

// MarshalJSON returns the JSON representation of d
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// NullDuration is a nullable Duration, in the same vein as the nullable types provided by
// package gopkg.in/guregu/null.v3.
type NullDuration struct {
	Duration
	Valid bool
}

// NewNullDuration is a simple helper constructor function
func NewNullDuration(d time.Duration, valid bool) NullDuration {
	return NullDuration{Duration(d), valid}
}

// NullDurationFrom returns a new valid NullDuration from a time.Duration.
func NullDurationFrom(d time.Duration) NullDuration {
	return NullDuration{Duration(d), true}
}

// UnmarshalText converts text data to a valid NullDuration
func (d *NullDuration) UnmarshalText(data []byte) error {
	if len(data) == 0 {
		*d = NullDuration{}
		return nil
	}
	if err := d.Duration.UnmarshalText(data); err != nil {
		return err
	}
	d.Valid = true
	return nil
}

// UnmarshalJSON converts JSON data to a valid NullDuration
func (d *NullDuration) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte(`null`)) {
		d.Valid = false
		return nil
	}
	if err := json.Unmarshal(data, &d.Duration); err != nil {
		return err
	}
	d.Valid = true
	return nil
}

// MarshalJSON returns the JSON representation of d
func (d NullDuration) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte(`null`), nil
	}
	return d.Duration.MarshalJSON()
}

// ValueOrZero returns the underlying Duration value of d if valid or
// its zero equivalent otherwise. It matches the existing guregu/null API.
func (d NullDuration) ValueOrZero() Duration {
	if !d.Valid {
		return Duration(0)
	}

	return d.Duration
}

// TimeDuration returns a NullDuration's value as a stdlib Duration.
func (d NullDuration) TimeDuration() time.Duration {
	return time.Duration(d.Duration)
}

func getInt64(v interface{}) (int64, error) {
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int8:
		return int64(n), nil
	case int16:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case uint:
		return int64(n), nil
	case uint8:
		return int64(n), nil
	case uint16:
		return int64(n), nil
	case uint32:
		return int64(n), nil
	case uint64:
		if n > math.MaxInt64 {
			return 0, fmt.Errorf("%d is too big", n)
		}
		return int64(n), nil
	default:
		return 0, fmt.Errorf("unable to use type %T as a duration value", v)
	}
}

// GetDurationValue is a helper function that can convert a lot of different
// types to time.Duration.
//
// TODO: move to a separate package and check for integer overflows?
func GetDurationValue(v interface{}) (time.Duration, error) {
	switch d := v.(type) {
	case time.Duration:
		return d, nil
	case string:
		return ParseExtendedDuration(d)
	case float32:
		return time.Duration(float64(d) * float64(time.Millisecond)), nil
	case float64:
		return time.Duration(d * float64(time.Millisecond)), nil
	default:
		n, err := getInt64(v)
		if err != nil {
			return 0, err
		}
		return time.Duration(n) * time.Millisecond, nil
	}
}
