/*
*
* k6 - a next-generation load testing tool
* Copyright (C) 2020 Load Impact
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

package eventhubs

import (
	"time"

	"github.com/loadimpact/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

// Config struct for EventHubs
type Config struct {
	ConnectionString null.String        `json:"connection_string,omitempty" envconfig:"K6_EVENTHUBS_CONNECTION_STRING"`
	BufferEnabled    null.Bool          `json:"buffer_enabled,omitempty" envconfig:"K6_EVENTHUBS_BUFFER_ENABLED"`
	PushInterval     types.NullDuration `json:"push_interval,omitempty" envconfig:"K6_EVENTHUBS_PUSH_INTERVAL"`
}

// NewConfig creates a new Config instance with default values for some fields.
func NewConfig() Config {
	return Config{
		ConnectionString: null.NewString("", false),
		BufferEnabled:    null.NewBool(true, false),
		PushInterval:     types.NewNullDuration(1*time.Second, false),
	}
}

// Apply saves config non-zero config values from the passed config in the receiver.
func (c Config) Apply(cfg Config) Config {

	if cfg.ConnectionString.Valid {
		c.ConnectionString = cfg.ConnectionString
	}
	if cfg.PushInterval.Valid {
		c.PushInterval = cfg.PushInterval
	}
	if cfg.BufferEnabled.Valid {
		c.BufferEnabled = cfg.BufferEnabled
	}
	return c
}
