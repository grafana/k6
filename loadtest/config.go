package loadtest

import (
	"gopkg.in/yaml.v2"
)

// Configuration type for a state.
type ConfigStage struct {
	Duration string `yaml:"duration"`
	VUs      []int  `yaml:"vus"`
}

type Config struct {
	Duration string        `yaml:"duration"`
	Script   string        `yaml:"script"`
	Stages   []ConfigStage `yaml:"stages"`
}

func NewConfig() Config {
	return Config{}
}

func ParseConfig(data []byte, conf *Config) (err error) {
	return yaml.Unmarshal(data, conf)
}
