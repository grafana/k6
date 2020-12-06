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

package statsd

import (
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/guregu/null.v3"

	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats/statsd/common"
)

// Config defines the StatsD configuration.
type Config struct {
	Addr         null.String        `json:"addr,omitempty" envconfig:"K6_STATSD_ADDR"`
	BufferSize   null.Int           `json:"bufferSize,omitempty" envconfig:"K6_STATSD_BUFFER_SIZE"`
	Namespace    null.String        `json:"namespace,omitempty" envconfig:"K6_STATSD_NAMESPACE"`
	PushInterval types.NullDuration `json:"pushInterval,omitempty" envconfig:"K6_STATSD_PUSH_INTERVAL"`
}

// GetAddr returns the address of the StatsD service.
func (c Config) GetAddr() null.String {
	return c.Addr
}

// GetBufferSize returns the size of the commands buffer.
func (c Config) GetBufferSize() null.Int {
	return c.BufferSize
}

// GetNamespace returns the namespace prepended to all statsd calls.
func (c Config) GetNamespace() null.String {
	return c.Namespace
}

// GetPushInterval returns the time interval between outgoing data batches.
func (c Config) GetPushInterval() types.NullDuration {
	return c.PushInterval
}

var _ common.Config = &Config{}

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

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		Addr:         null.NewString("localhost:8125", false),
		BufferSize:   null.NewInt(20, false),
		Namespace:    null.NewString("k6.", false),
		PushInterval: types.NewNullDuration(1*time.Second, false),
	}
}

// New creates a new statsd connector client
func New(logger logrus.FieldLogger, conf common.Config) (*common.Collector, error) {
	return &common.Collector{
		Config: conf,
		Type:   "statsd",
		Logger: logger,
	}, nil
}
