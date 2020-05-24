/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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
	"time"

	"github.com/loadimpact/k6/lib/types"
	null "gopkg.in/guregu/null.v4"
)

// Config defines the statsd configuration
type Config struct {
	Addr         null.String        `json:"addr,omitempty" envconfig:"ADDR"`
	BufferSize   null.Int           `json:"bufferSize,omitempty" envconfig:"BUFFER_SIZE"`
	Namespace    null.String        `json:"namespace,omitempty" envconfig:"NAMESPACE"`
	PushInterval types.NullDuration `json:"pushInterval,omitempty" envconfig:"PUSH_INTERVAL"`
}

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		Addr:         null.NewString("localhost:8125", false),
		BufferSize:   null.NewInt(20, false),
		Namespace:    null.NewString("k6.", false),
		PushInterval: types.NewNullDuration(1*time.Second, false),
	}
}

// Apply saves config non-zero config values from the passed config in the receiver.
func (c Config) Apply(cfg Config) Config {
	if cfg.Addr.Valid {
		c.Addr = cfg.Addr
	}

	if cfg.BufferSize.Valid {
		c.BufferSize = cfg.BufferSize
	}

	if cfg.Namespace.Valid {
		c.Namespace = cfg.Namespace
	}

	if cfg.PushInterval.Valid {
		c.PushInterval = cfg.PushInterval
	}

	return c
}
