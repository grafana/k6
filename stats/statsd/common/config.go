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

package common

import (
	"encoding/json"
	"strings"
)

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
	vals := strings.Split(string(text), ":")
	// A connection, if provided, needs to be in the shape of ADDRESS:PORT
	if len(vals) != 2 {
		return nil
	}
	c.Addr = vals[0]
	c.Port = vals[1]
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
