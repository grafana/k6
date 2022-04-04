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

package lib

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

//TODO: update test
/*
func TestSumStages(t *testing.T) {
	testdata := map[string]struct {
		Time   types.NullDuration
		Stages []Stage
	}{
		"Blank":    {types.NullDuration{}, []Stage{}},
		"Infinite": {types.NullDuration{}, []Stage{{}}},
		"Limit": {
			types.NullDurationFrom(10 * time.Second),
			[]Stage{
				{Duration: types.NullDurationFrom(5 * time.Second)},
				{Duration: types.NullDurationFrom(5 * time.Second)},
			},
		},
		"InfiniteTail": {
			types.NullDuration{Duration: types.Duration(10 * time.Second), Valid: false},
			[]Stage{
				{Duration: types.NullDurationFrom(5 * time.Second)},
				{Duration: types.NullDurationFrom(5 * time.Second)},
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
*/

func TestMin(t *testing.T) {
	t.Parallel()
	assert.Equal(t, int64(10), Min(10, 100))
	assert.Equal(t, int64(10), Min(100, 10))
}

func TestMax(t *testing.T) {
	t.Parallel()
	assert.Equal(t, int64(100), Max(10, 100))
	assert.Equal(t, int64(100), Max(100, 10))
}
