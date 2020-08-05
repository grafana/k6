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

	client "github.com/influxdata/influxdb1-client/v2"
	"github.com/pkg/errors"
	"gopkg.in/guregu/null.v3"
)

func MakeClient(conf Config) (client.Client, error) {
	if strings.HasPrefix(conf.Addr.String, "udp://") {
		return client.NewUDPClient(client.UDPConfig{
			Addr:        strings.TrimPrefix(conf.Addr.String, "udp://"),
			PayloadSize: int(conf.PayloadSize.Int64),
		})
	}
	if conf.Addr.String == "" {
		conf.Addr = null.StringFrom("http://localhost:8086")
	}
	return client.NewHTTPClient(client.HTTPConfig{
		Addr:               conf.Addr.String,
		Username:           conf.Username.String,
		Password:           conf.Password.String,
		UserAgent:          "k6",
		InsecureSkipVerify: conf.Insecure.Bool,
	})
}

func MakeBatchConfig(conf Config) client.BatchPointsConfig {
	if !conf.DB.Valid || conf.DB.String == "" {
		conf.DB = null.StringFrom("k6")
	}
	return client.BatchPointsConfig{
		Precision:        conf.Precision.String,
		Database:         conf.DB.String,
		RetentionPolicy:  conf.Retention.String,
		WriteConsistency: conf.Consistency.String,
	}
}

func checkDuplicatedTypeDefinitions(fieldKinds map[string]FieldKind, tag string) error {
	if _, found := fieldKinds[tag]; found {
		return errors.Errorf("A tag name (%s) shows up more than once in InfluxDB field type configurations.", tag)
	}
	return nil
}

// MakeFieldKinds returns a map[string]fieldKind built from BoolFields, FloadFields and IntFields in a Config
func MakeFieldKinds(conf Config) (map[string]FieldKind, error) {
	fieldKinds := make(map[string]FieldKind)
	for _, tag := range conf.BoolFields {
		err := checkDuplicatedTypeDefinitions(fieldKinds, tag)
		if err != nil {
			return nil, err
		}
		fieldKinds[tag] = Bool
	}
	for _, tag := range conf.FloatFields {
		err := checkDuplicatedTypeDefinitions(fieldKinds, tag)
		if err != nil {
			return nil, err
		}
		fieldKinds[tag] = Float
	}
	for _, tag := range conf.IntFields {
		err := checkDuplicatedTypeDefinitions(fieldKinds, tag)
		if err != nil {
			return nil, err
		}
		fieldKinds[tag] = Int
	}
	return fieldKinds, nil
}
