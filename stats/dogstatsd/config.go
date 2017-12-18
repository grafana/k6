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

package dogstatsd

import (
	"encoding/json"

	statsd "github.com/loadimpact/k6/stats/statsd/common"
)

// ConfigFields contains statsd configuration
type ConfigFields struct {
	Addr         string `json:"addr,omitempty" envconfig:"DOGSTATSD_ADDR"`
	P            string `json:"port,omitempty" envconfig:"DOGSTATSD_PORT" default:"8126"`
	BuffSize     int    `json:"buffer_size,omitempty" envconfig:"DOGSTATSD_BUFFER_SIZE" default:"20"`
	Namespace    string `json:"namespace,omitempty" envconfig:"DOGSTATSD_NAMESPACE"`
	TagWhitelist string `json:"tag_whitelist,omitempty" envconfig:"DOGSTATSD_TAG_WHITELIST" default:"status, method"`
}

// Config defines a config type
type Config ConfigFields

// Address returns client address
func (c Config) Address() string {
	return c.Addr
}

// Port returns client port
func (c Config) Port() string {
	return c.P
}

// BufferSize returns client buffer size
func (c Config) BufferSize() int {
	return c.BuffSize
}

// Extra returns client extra config
func (c Config) Extra() *statsd.ExtraConfig {
	return &statsd.ExtraConfig{
		Namespace:    c.Namespace,
		TagWhitelist: c.TagWhitelist,
	}
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
