package types

import (
	"encoding/json"
	"errors"
)

type Email string

func (e Email) MarshalJSON() ([]byte, error) {
	if !emailRegex.MatchString(string(e)) {
		return nil, errors.New("email: failed to pass regex validation")
	}
	return json.Marshal(string(e))
}

func (e *Email) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if !emailRegex.MatchString(s) {
		return errors.New("email: failed to pass regex validation")
	}
	*e = Email(s)
	return nil
}
