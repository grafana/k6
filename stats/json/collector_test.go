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

package json

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewWithInaccessibleFilename(t *testing.T) {
	p := string("/this_should_not_exist/badplacetolog.log")
	collector, err := New(p)
	assert.NotEqual(t, err, error(nil))
	assert.Equal(t, collector, (*Collector)(nil))
}

func TestNewWithFileName(t *testing.T) {
	p := string("/tmp/okplacetolog.log")
	collector, err := New(p)
	assert.Equal(t, err, error(nil))
	assert.NotEqual(t, collector, (*Collector)(nil))
}

func TestNewWithLocalFilename1(t *testing.T) {
	p := string("./okplacetolog.log")
	collector, err := New(p)
	assert.Equal(t, err, error(nil))
	assert.NotEqual(t, collector, (*Collector)(nil))
}

func TestNewWithLocalFilename2(t *testing.T) {
	p := string("okplacetolog.log")
	collector, err := New(p)
	assert.Equal(t, err, error(nil))
	assert.NotEqual(t, collector, (*Collector)(nil))
}
