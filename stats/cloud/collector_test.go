/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2017 Load Impact
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

package cloud

import (
	"os"
	"testing"

	"github.com/loadimpact/k6/lib"
	"github.com/stretchr/testify/assert"
)

func TestGetName(t *testing.T) {
	nameTests := []struct {
		lib      *lib.SourceData
		conf     loadimpactConfig
		expected string
	}{
		{&lib.SourceData{Filename: ""}, loadimpactConfig{}, TestName},
		{&lib.SourceData{Filename: "-"}, loadimpactConfig{}, TestName},
		{&lib.SourceData{Filename: "script.js"}, loadimpactConfig{}, "script.js"},
		{&lib.SourceData{Filename: "/file/name.js"}, loadimpactConfig{}, "name.js"},
		{&lib.SourceData{Filename: "/file/name"}, loadimpactConfig{}, "name"},
		{&lib.SourceData{Filename: "/file/name"}, loadimpactConfig{Name: "confName"}, "confName"},
	}

	for _, test := range nameTests {
		actual := getName(test.lib, test.conf)
		assert.Equal(t, actual, test.expected)
	}

	err := os.Setenv("K6CLOUD_NAME", "envname")
	assert.Nil(t, err)

	for _, test := range nameTests {
		actual := getName(test.lib, test.conf)
		assert.Equal(t, actual, "envname")
	}
}
