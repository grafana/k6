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
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
)

func TestParseArg(t *testing.T) {
	testdata := map[string]Config{
		"":                                     {},
		"db=dbname":                            {DB: null.StringFrom("dbname")},
		"addr=http://localhost:8086":           {Addr: null.StringFrom("http://localhost:8086")},
		"addr=http://localhost:8086,db=dbname": {Addr: null.StringFrom("http://localhost:8086"), DB: null.StringFrom("dbname")},
		"addr=http://localhost:8086,db=dbname,insecure=false,payloadSize=69,":                    {Addr: null.StringFrom("http://localhost:8086"), DB: null.StringFrom("dbname"), Insecure: null.BoolFrom(false), PayloadSize: null.IntFrom(69)},
		"addr=http://localhost:8086,db=dbname,insecure=false,payloadSize=69,tagsAsFields={fake}": {Addr: null.StringFrom("http://localhost:8086"), DB: null.StringFrom("dbname"), Insecure: null.BoolFrom(false), PayloadSize: null.IntFrom(69), TagsAsFields: []string{"fake"}},
	}

	for str, expConfig := range testdata {
		t.Run(str, func(t *testing.T) {
			config, err := ParseArg(str)

			assert.NoError(t, err)
			assert.Equal(t, expConfig, config)
		})
	}
}

func TestParseURL(t *testing.T) {
	testdata := map[string]Config{
		"":                             {},
		"dbname":                       {DB: null.StringFrom("dbname")},
		"/dbname":                      {DB: null.StringFrom("dbname")},
		"http://localhost:8086":        {Addr: null.StringFrom("http://localhost:8086")},
		"http://localhost:8086/dbname": {Addr: null.StringFrom("http://localhost:8086"), DB: null.StringFrom("dbname")},
	}
	queries := map[string]struct {
		Config Config
		Err    string
	}{
		"?":                {Config{}, ""},
		"?insecure=false":  {Config{Insecure: null.BoolFrom(false)}, ""},
		"?insecure=true":   {Config{Insecure: null.BoolFrom(true)}, ""},
		"?insecure=ture":   {Config{}, "insecure must be true or false, not ture"},
		"?payload_size=69": {Config{PayloadSize: null.IntFrom(69)}, ""},
		"?payload_size=a":  {Config{}, "strconv.Atoi: parsing \"a\": invalid syntax"},
	}
	for str, data := range testdata {
		t.Run(str, func(t *testing.T) {
			config, err := ParseURL(str)
			assert.NoError(t, err)
			assert.Equal(t, data, config)

			for q, qdata := range queries {
				t.Run(q, func(t *testing.T) {
					config, err := ParseURL(str + q)
					if qdata.Err != "" {
						assert.EqualError(t, err, qdata.Err)
					} else {
						expected2 := qdata.Config
						expected2.DB = data.DB
						expected2.Addr = data.Addr
						assert.Equal(t, expected2, config)
					}
				})
			}
		})
	}
}
