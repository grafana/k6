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
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"

	"github.com/k6io/k6/lib/types"
)

func TestStageJSON(t *testing.T) {
	s := Stage{Duration: types.NullDurationFrom(10 * time.Second), Target: null.IntFrom(10)}

	data, err := json.Marshal(s)
	assert.NoError(t, err)
	assert.Equal(t, `{"duration":"10s","target":10}`, string(data))

	var s2 Stage
	assert.NoError(t, json.Unmarshal(data, &s2))
	assert.Equal(t, s, s2)
}

// Suggested by @nkovacs in https://github.com/k6io/k6/issues/207#issuecomment-330545467
func TestDataRaces(t *testing.T) {
	t.Run("Check race", func(t *testing.T) {
		group, err := NewGroup("test", nil)
		assert.Nil(t, err, "NewGroup")
		wg := sync.WaitGroup{}
		wg.Add(2)
		var check1, check2 *Check
		go func() {
			var err error // using the outer err would result in a data race
			check1, err = group.Check("race")
			assert.Nil(t, err, "Check 1")
			wg.Done()
		}()
		go func() {
			var err error
			check2, err = group.Check("race")
			assert.Nil(t, err, "Check 2")
			wg.Done()
		}()
		wg.Wait()
		assert.Equal(t, check1, check2, "Checks are the same")
	})
	t.Run("Group race", func(t *testing.T) {
		group, err := NewGroup("test", nil)
		assert.Nil(t, err, "NewGroup")
		wg := sync.WaitGroup{}
		wg.Add(2)
		var group1, group2 *Group
		go func() {
			var err error
			group1, err = group.Group("race")
			assert.Nil(t, err, "Group 1")
			wg.Done()
		}()
		go func() {
			var err error
			group2, err = group.Group("race")
			assert.Nil(t, err, "Group 2")
			wg.Done()
		}()
		wg.Wait()
		assert.Equal(t, group1, group2, "Groups are the same")
	})
}
