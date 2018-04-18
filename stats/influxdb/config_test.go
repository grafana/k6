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
)

func TestConfigText(t *testing.T) {
	defaultTagsAsFields := []string{"vu", "iter", "url"}
	testdata := map[string]Config{
		"":                             {TagsAsFields: defaultTagsAsFields},
		"dbname":                       {DB: "dbname", TagsAsFields: defaultTagsAsFields},
		"/dbname":                      {DB: "dbname", TagsAsFields: defaultTagsAsFields},
		"http://localhost:8086":        {Addr: "http://localhost:8086", TagsAsFields: defaultTagsAsFields},
		"http://localhost:8086/dbname": {Addr: "http://localhost:8086", DB: "dbname", TagsAsFields: defaultTagsAsFields},
	}
	queries := map[string]struct {
		Config Config
		Err    string
	}{
		"?":                {Config{TagsAsFields: defaultTagsAsFields}, ""},
		"?insecure=false":  {Config{Insecure: false, TagsAsFields: defaultTagsAsFields}, ""},
		"?insecure=true":   {Config{Insecure: true, TagsAsFields: defaultTagsAsFields}, ""},
		"?insecure=ture":   {Config{TagsAsFields: defaultTagsAsFields}, "insecure must be true or false, not ture"},
		"?payload_size=69": {Config{PayloadSize: 69, TagsAsFields: defaultTagsAsFields}, ""},
		"?payload_size=a":  {Config{TagsAsFields: defaultTagsAsFields}, "strconv.Atoi: parsing \"a\": invalid syntax"},
	}
	for str, data := range testdata {
		t.Run(str, func(t *testing.T) {
			var config Config
			assert.NoError(t, config.UnmarshalText([]byte(str)))
			assert.Equal(t, data, config)

			for q, qdata := range queries {
				t.Run(q, func(t *testing.T) {
					var config Config
					err := config.UnmarshalText([]byte(str + q))
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
