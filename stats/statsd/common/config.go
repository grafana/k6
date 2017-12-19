package statsd

import "encoding/json"

// ExtraConfig contains extra statsd config
type ExtraConfig struct {
	Namespace    string
	TagWhitelist string
}

// Config defines the statsd configuration
type Config struct {
	Addr         string `json:"addr,omitempty"`
	Port         string `json:"port,omitempty" default:"8126"`
	BufferSize   int    `json:"buffer_size,omitempty" default:"20"`
	Namespace    string `json:"namespace,omitempty"`
	TagWhitelist string `json:"tag_whitelist,omitempty" default:"status, method"`
}

// Apply returns config with defaults applied
func (c Config) Apply(cfg Config) Config {
	return c
}

// UnmarshalText used to convert string into a struct
func (c *Config) UnmarshalText(text []byte) error {
	return nil
}

// UnmarshalJSON sets Config from json
func (c *Config) UnmarshalJSON(data []byte) error {
	fields := Config(*c)
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*c = Config(fields)
	return nil
}

// MarshalJSON returns a marshalled json object
func (c Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(Config(c))
}
