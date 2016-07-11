package writer

import (
	"encoding/json"
)

type JSONFormatter struct{}

func (JSONFormatter) Format(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

type PrettyJSONFormatter struct{}

func (PrettyJSONFormatter) Format(data interface{}) ([]byte, error) {
	return json.MarshalIndent(data, "", "    ")
}
