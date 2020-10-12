/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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

package appinsights

import (
	"fmt"
	"strings"

	"gopkg.in/guregu/null.v3"
)

// Config struct for AppInsights
type Config struct {
	InstrumentationKey null.String `json:"instrumentation_key" envconfig:"K6_APPINSIGHTS_INSTRUMENTATION_KEY"`
}

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{}
}

// Apply saves config non-zero config values from the passed config in the receiver.
func (c Config) Apply(cfg Config) Config {
	if cfg.InstrumentationKey.Valid {
		c.InstrumentationKey = cfg.InstrumentationKey
	}
	return c
}

// ParseArg takes an arg string and converts it to a config
func ParseArg(arg string) (Config, error) {
	c := Config{}

	pairs := strings.Split(arg, ",")
	for _, pair := range pairs {
		r := strings.SplitN(pair, "=", 2)
		if len(r) != 2 {
			return c, fmt.Errorf("couldn't parse %q as argument for csv output", arg)
		}
		switch r[0] {
		case "instrumentation_key":
			c.InstrumentationKey = null.StringFrom(r[1])
		default:
			return c, fmt.Errorf("unknown key %q as argument for csv output", r[0])
		}
	}

	return c, nil
}
