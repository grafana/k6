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

package core

import (
	"testing"

	"time"

	"github.com/loadimpact/k6/lib"
	"github.com/stretchr/testify/assert"
)

func TestSumStages(t *testing.T) {
	testdata := map[string]struct {
		Time   lib.NullDuration
		Stages []lib.Stage
	}{
		"Blank":    {lib.NullDuration{}, []lib.Stage{}},
		"Infinite": {lib.NullDuration{}, []lib.Stage{{}}},
		"Limit": {
			lib.NullDurationFrom(10 * time.Second),
			[]lib.Stage{
				{Duration: lib.NullDurationFrom(5 * time.Second)},
				{Duration: lib.NullDurationFrom(5 * time.Second)},
			},
		},
		"InfiniteTail": {
			lib.NullDuration{Duration: lib.Duration(10 * time.Second), Valid: false},
			[]lib.Stage{
				{Duration: lib.NullDurationFrom(5 * time.Second)},
				{Duration: lib.NullDurationFrom(5 * time.Second)},
				{},
			},
		},
	}
	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, data.Time, SumStages(data.Stages))
		})
	}
}
