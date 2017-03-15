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

package main

import (
	"testing"
	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
)

func TestParseStage(t *testing.T) {
	testdata := map[string]lib.Stage{
		"":        {},
		":":       {},
		"10s":     {Duration: 10 * time.Second},
		"10s:":    {Duration: 10 * time.Second},
		"10s:100": {Duration: 10 * time.Second, Target: null.IntFrom(100)},
		":100":    {Target: null.IntFrom(100)},
	}
	for s, st := range testdata {
		t.Run(s, func(t *testing.T) {
			parsed, err := ParseStage(s)
			assert.NoError(t, err)
			assert.Equal(t, st, parsed)
		})
	}
}
