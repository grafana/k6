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
	"testing"
	"time"

	"github.com/loadimpact/k6/lib/types"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
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
