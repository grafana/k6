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
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	client "github.com/influxdata/influxdb-client-go/v2"
	influxdblog "github.com/influxdata/influxdb-client-go/v2/log"
	"gopkg.in/guregu/null.v3"
)

func init() {
	// disable the internal influxdb log
	influxdblog.Log = nil
}

func MakeClient(conf Config) (client.Client, error) {
	if conf.Addr.String == "" {
		conf.Addr = null.StringFrom("http://localhost:8086")
	}
	opts := client.DefaultOptions().
		SetTLSConfig(&tls.Config{
			InsecureSkipVerify: conf.InsecureSkipTLSVerify.Bool, //nolint:gosec
		})
	if conf.Precision.Valid {
		opts.SetPrecision(time.Duration(conf.Precision.Duration))
	}
	return client.NewClientWithOptions(conf.Addr.String, conf.Token.String, opts), nil
}

func checkDuplicatedTypeDefinitions(fieldKinds map[string]FieldKind, tag string) error {
	if _, found := fieldKinds[tag]; found {
		return fmt.Errorf("a tag name (%s) shows up more than once in InfluxDB field type configurations", tag)
	}
	return nil
}

// MakeFieldKinds reads the Config and returns a lookup map of tag names to
// the field type their values should be converted to.
func MakeFieldKinds(conf Config) (map[string]FieldKind, error) {
	fieldKinds := make(map[string]FieldKind)
	for _, tag := range conf.TagsAsFields {
		var fieldName, fieldType string
		s := strings.SplitN(tag, ":", 2)
		if len(s) == 1 {
			fieldName, fieldType = s[0], "string"
		} else {
			fieldName, fieldType = s[0], s[1]
		}

		err := checkDuplicatedTypeDefinitions(fieldKinds, fieldName)
		if err != nil {
			return nil, err
		}

		switch fieldType {
		case "string":
			fieldKinds[fieldName] = String
		case "bool":
			fieldKinds[fieldName] = Bool
		case "float":
			fieldKinds[fieldName] = Float
		case "int":
			fieldKinds[fieldName] = Int
		default:
			return nil, fmt.Errorf("an invalid type (%s) is specified for an InfluxDB field (%s)",
				fieldType, fieldName)
		}
	}

	return fieldKinds, nil
}
