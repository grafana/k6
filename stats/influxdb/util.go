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
	"strings"

	client "github.com/influxdata/influxdb/client/v2"
)

func MakeClient(conf Config) (client.Client, error) {
	if strings.HasPrefix(conf.Addr, "udp://") {
		return client.NewUDPClient(client.UDPConfig{
			Addr:        strings.TrimPrefix(conf.Addr, "udp://"),
			PayloadSize: conf.PayloadSize,
		})
	}
	if conf.Addr == "" {
		conf.Addr = "http://localhost:8086"
	}
	return client.NewHTTPClient(client.HTTPConfig{
		Addr:               conf.Addr,
		Username:           conf.Username,
		Password:           conf.Password,
		UserAgent:          "k6",
		InsecureSkipVerify: conf.Insecure,
	})
}

func MakeBatchConfig(conf Config) client.BatchPointsConfig {
	if conf.Database == "" {
		conf.Database = "k6"
	}
	return client.BatchPointsConfig{
		Precision:        conf.Precision,
		Database:         conf.Database,
		RetentionPolicy:  conf.Retention,
		WriteConsistency: conf.Consistency,
	}
}
