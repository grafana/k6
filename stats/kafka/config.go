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

package kafka

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
)

type ConfigFields struct {
	// Connection.
	Broker  string `json:"broker" envconfig:"KAFKA_BROKER"`

	// Samples.
	Topic   string `json:"topic" envconfig:"KAFKA_TOPIC"`
	Format  string `json:"format" envconfig:"KAFKA_FORMAT"`
}

type Config ConfigFields

func (c Config) Apply(cfg Config) Config {
	if cfg.Broker != "" {
		c.Broker = cfg.Broker
	}
	if cfg.Format != "" {
		c.Format = cfg.Format
	}
	if cfg.Topic != "" {
		c.Topic = cfg.Topic
	}
	return c
}

func (c *Config) UnmarshalText(data []byte) error {
	params := strings.Split(string(data), ",")

	for _, param := range params {
		s := strings.SplitN(param, "=", 2)
		key := s[0]
		val := s[1]

		switch key {
		case "broker":
			c.Broker = val
		case "topic":
			c.Topic = val
		case "format":
			c.Format = val
		default:
			return errors.Errorf("unknown query parameter: %s", key)
		}
	}

	return nil
}

func (c *Config) UnmarshalJSON(data []byte) error {
	fields := ConfigFields(*c)
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	*c = Config(fields)
	return nil
}

func (c Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(ConfigFields(c))
}
