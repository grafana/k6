/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2016 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package statsd

import (
	"encoding/json"
)

// ConfigFields contains statsd configuration
type ConfigFields struct {
	// Connection.
	StatsDAddr       string `json:"statsd_addr,omitempty" envconfig:"STATSD_ADDR"`
	StatsDPort       string `json:"statsd_port,omitempty" envconfig:"STATSD_PORT" default:"8126"`
	StatsDBufferSize int    `json:"statsd_buffer_size,omitempty" envconfig:"STATSD_BUFFER_SIZE" default:"10"`

	DogStatsDAddr        string `json:"dogstatsd_addr,omitempty" envconfig:"DOGSTATSD_ADDR"`
	DogStatsDPort        string `json:"dogstatsd_port,omitempty" envconfig:"DOGSTATSD_PORT" default:"8126"`
	DogStatsDBufferSize  int    `json:"dogstatsd_buffer_size,omitempty" envconfig:"DOGSTATSD_BUFFER_SIZE" default:"10"`
	DogStatsNamespace    string `json:"dogstatsd_namespace,omitempty" envconfig:"DOGSTATSD_NAMESPACE"`
	DogStatsTagWhitelist string `json:"dogstatsd_tag_whitelist,omitempty" envconfig:"DOGSTATSD_TAG_WHITELIST" default:"status, method"`
}

// Config defines a config type
type Config ConfigFields

// Apply returns config with defaults applied
func (c Config) Apply(cfg Config) Config {
	if cfg.StatsDAddr != "" {
		c.StatsDAddr = cfg.StatsDAddr
	}
	if cfg.StatsDPort != "" {
		c.StatsDPort = cfg.StatsDPort
	}
	if cfg.StatsDBufferSize != 0 {
		c.StatsDBufferSize = cfg.StatsDBufferSize
	}

	if cfg.DogStatsDAddr != "" {
		c.DogStatsDAddr = cfg.DogStatsDAddr
	}
	if cfg.DogStatsDPort != "" {
		c.DogStatsDPort = cfg.DogStatsDPort
	}
	if cfg.DogStatsDBufferSize != 0 {
		c.DogStatsDBufferSize = cfg.DogStatsDBufferSize
	}
	if cfg.DogStatsNamespace != "" {
		c.DogStatsNamespace = cfg.DogStatsNamespace
	}
	if cfg.DogStatsTagWhitelist != "" {
		c.DogStatsTagWhitelist = cfg.DogStatsTagWhitelist
	}

	return c
}

// UnmarshalText used to convert string into a struct
func (c *Config) UnmarshalText(text []byte) error {
	return nil
}

// UnmarshalJSON sets Config from json
func (c *Config) UnmarshalJSON(data []byte) error {
	fields := ConfigFields(*c)
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*c = Config(fields)
	return nil
}

// MarshalJSON returns a marshalled json object
func (c Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(ConfigFields(c))
}
