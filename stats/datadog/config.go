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

package datadog

import (
	"encoding/json"
)

// ConfigFields contains statsd configuration
type ConfigFields struct {
	// Connection.
	Addr       string `json:"addr" envconfig:"STATSD_ADDR"`
	Port       string `json:"port,omitempty" envconfig:"STATSD_PORT"`
	Namespace  string `json:"namespace,omitempty" envconfig:"STATSD_NAMESPACE"`
	BufferSize int    `json:"buffer_size" envconfig:"STATSD_BUFFER_SIZE"`
}

// Config defines a config type
type Config ConfigFields

// Apply returns config with defaults applied
func (c Config) Apply(cfg Config) Config {
	if cfg.Addr != "" {
		c.Addr = cfg.Addr
	}
	if cfg.Port != "" {
		c.Port = cfg.Port
	}
	if cfg.Namespace != "" {
		c.Namespace = cfg.Namespace
	}
	if cfg.BufferSize != 0 {
		c.BufferSize = cfg.BufferSize
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
