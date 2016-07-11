package writer

import (
	"gopkg.in/yaml.v2"
)

type YAMLFormatter struct{}

func (YAMLFormatter) Format(data interface{}) ([]byte, error) {
	return yaml.Marshal(data)
}
