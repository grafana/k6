package metrics

import "errors"

// Possible values for ValueType.
const (
	Default = ValueType(iota) // Values are presented as-is
	Time                      // Values are time durations (milliseconds)
	Data                      // Values are data amounts (bytes)
)

// ErrInvalidValueType indicates the serialized value type is invalid.
var ErrInvalidValueType = errors.New("invalid value type")

// ValueType holds the type of values a metric contains.
type ValueType int

// MarshalJSON serializes a ValueType to a JSON string.
func (t ValueType) MarshalJSON() ([]byte, error) {
	txt, err := t.MarshalText()
	if err != nil {
		return nil, err
	}
	return []byte(`"` + string(txt) + `"`), nil
}

// MarshalText serializes a ValueType as a human readable string.
func (t ValueType) MarshalText() ([]byte, error) {
	switch t {
	case Default:
		return []byte(defaultString), nil
	case Time:
		return []byte(timeString), nil
	case Data:
		return []byte(dataString), nil
	default:
		return nil, ErrInvalidValueType
	}
}

// UnmarshalText deserializes a ValueType from a string representation.
func (t *ValueType) UnmarshalText(data []byte) error {
	switch string(data) {
	case defaultString:
		*t = Default
	case timeString:
		*t = Time
	case dataString:
		*t = Data
	default:
		return ErrInvalidValueType
	}

	return nil
}

func (t ValueType) String() string {
	switch t {
	case Default:
		return defaultString
	case Time:
		return timeString
	case Data:
		return dataString
	default:
		return "[INVALID]"
	}
}
