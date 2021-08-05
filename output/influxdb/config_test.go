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
	"time"

	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/lib/types"
	"gopkg.in/guregu/null.v3"
)

func TestParseURL(t *testing.T) {
	t.Parallel()
	testdata := map[string]Config{
		"":                                 {},
		"bucketname":                       {Bucket: null.StringFrom("bucketname")},
		"/bucketname":                      {Bucket: null.StringFrom("bucketname")},
		"/dbname/retention":                {Bucket: null.StringFrom("dbname/retention")}, // 1.8+ API compatibility
		"http://localhost:8086":            {Addr: null.StringFrom("http://localhost:8086")},
		"http://localhost:8086/bucketname": {Addr: null.StringFrom("http://localhost:8086"), Bucket: null.StringFrom("bucketname")},
	}
	queries := map[string]struct {
		Config Config
		Err    string
	}{
		"?":                                  {Config{}, ""},
		"?insecure=false":                    {Config{Insecure: null.BoolFrom(false)}, ""},
		"?insecure=true":                     {Config{Insecure: null.BoolFrom(true)}, ""},
		"?insecure=ture":                     {Config{}, "insecure must be true or false, not ture"},
		"?pushInterval=5s":                   {Config{PushInterval: types.NullDurationFrom(5 * time.Second)}, ""},
		"?concurrentWrites=2":                {Config{ConcurrentWrites: null.IntFrom(2)}, ""},
		"?tagsAsFields=foo&tagsAsFields=bar": {Config{TagsAsFields: []string{"foo", "bar"}}, ""},
	}
	for str, data := range testdata {
		t.Run(str, func(t *testing.T) {
			t.Parallel()
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
						expected2.Bucket = data.Bucket
						expected2.Addr = data.Addr
						assert.Equal(t, expected2, config)
					}
				})
			}
		})
	}
}
