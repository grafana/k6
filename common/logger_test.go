/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
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
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testLogFormatter struct{}

func (f *testLogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	return []byte(entry.Message), nil
}

func TestConsoleLogFormatter(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		objects  []interface{}
		expected string
	}{
		{objects: nil, expected: ""},
		{
			objects: []interface{}{
				map[string]interface{}{"one": 1, "two": "two"},
				map[string]interface{}{"nested": map[string]interface{}{
					"sub": float64(7.777),
				}},
			},
			expected: `{"one":1,"two":"two"} {"nested":{"sub":7.777}}`,
		},
		{
			objects: []interface{}{
				map[string]interface{}{"one": 1, "fail": make(chan int)},
				map[string]interface{}{"two": 2},
			},
			expected: `{"two":2}`,
		},
	}

	fmtr := &consoleLogFormatter{&testLogFormatter{}}

	for _, tc := range testCases {
		var data logrus.Fields
		if tc.objects != nil {
			data = logrus.Fields{"objects": tc.objects}
		}
		out, err := fmtr.Format(&logrus.Entry{Data: data})
		require.NoError(t, err)
		assert.Equal(t, tc.expected, string(out))
	}
}
