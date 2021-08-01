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

package stats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseThresholdName(t *testing.T) {
	testdata := map[string]struct {
		metricName string
		tagsAsStr  string
		tags       map[string]string
	}{
		"my_metric": {"my_metric", "", map[string]string{}},
		"my_metric{}": {"my_metric", "", map[string]string{}},
		"my_metric{a}": {"my_metric", "a", map[string]string{"a": ""}},
		"my_metric{a:1}": {"my_metric", "a:1", map[string]string{"a": "1"}},
		"my_metric{ a : 1 }": {"my_metric", " a : 1 ", map[string]string{"a": "1"}},
		"my_metric{a,b}": {"my_metric", "a,b", map[string]string{"a": "", "b": ""}},
		"my_metric{a:1,b:2}": {"my_metric", "a:1,b:2", map[string]string{"a": "1", "b": "2"}},
		"my_metric{ a : 1, b : 2 }": {"my_metric", " a : 1, b : 2 ", map[string]string{"a": "1", "b": "2"}},
	}

	for input, expected := range testdata {
		t.Run(input, func(t *testing.T) {
			metricName, tagsAsStr, tags := ParseThresholdName(input)
			assert.Equal(t, expected.metricName, metricName)
			assert.Equal(t, expected.tagsAsStr, tagsAsStr)
			assert.Equal(t, expected.tags, tags)
		})
	}
}
