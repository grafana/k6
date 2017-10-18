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

package influxdb

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

type ConfigFields struct {
	// Connection.
	Addr        string `json:"addr" envconfig:"INFLUXDB_ADDR"`
	Username    string `json:"username,omitempty" envconfig:"INFLUXDB_USERNAME"`
	Password    string `json:"password,omitempty" envconfig:"INFLUXDB_PASSWORD"`
	Insecure    bool   `json:"insecure,omitempty" envconfig:"INFLUXDB_INSECURE"`
	PayloadSize int    `json:"payload_size,omitempty" envconfig:"INFLUXDB_PAYLOAD_SIZE"`

	// Samples.
	DB          string `json:"db" envconfig:"INFLUXDB_DB"`
	Precision   string `json:"precision,omitempty" envconfig:"INFLUXDB_PRECISION"`
	Retention   string `json:"retention,omitempty" envconfig:"INFLUXDB_RETENTION"`
	Consistency string `json:"consistency,omitempty" envconfig:"INFLUXDB_CONSISTENCY"`
}

type Config ConfigFields

func (c *Config) UnmarshalText(text []byte) error {
	u, err := url.Parse(string(text))
	if err != nil {
		return err
	}
	if u.Host != "" {
		c.Addr = u.Scheme + "://" + u.Host
	}
	if db := strings.TrimPrefix(u.Path, "/"); db != "" {
		c.DB = db
	}
	if u.User != nil {
		c.Username = u.User.Username()
		c.Password, _ = u.User.Password()
	}
	for k, vs := range u.Query() {
		switch k {
		case "insecure":
			switch vs[0] {
			case "":
			case "false":
				c.Insecure = false
			case "true":
				c.Insecure = true
			default:
				return errors.Errorf("insecure must be true or false, not %s", vs[0])
			}
		case "payload_size":
			c.PayloadSize, err = strconv.Atoi(vs[0])
		case "precision":
			c.Precision = vs[0]
		case "retention":
			c.Retention = vs[0]
		case "consistency":
			c.Consistency = vs[0]
		default:
			return errors.Errorf("unknown query parameter: %s", k)
		}
	}
	return err
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
