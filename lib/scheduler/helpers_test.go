/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2019 Load Impact
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

package scheduler

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckPercentagesSum(t *testing.T) {
	t.Parallel()
	assert.NoError(t, checkPercentagesSum([]float64{100}))
	assert.NoError(t, checkPercentagesSum([]float64{50, 50}))
	assert.NoError(t, checkPercentagesSum([]float64{100.0 / 3, 100.0 / 3, 100.0 / 3}))
	assert.NoError(t, checkPercentagesSum([]float64{33.33, 33.33, 33.34}))

	assert.Error(t, checkPercentagesSum([]float64{}))
	assert.Error(t, checkPercentagesSum([]float64{100 / 3, 100 / 3, 100 / 3}))
	assert.Error(t, checkPercentagesSum([]float64{33.33, 33.33, 33.33}))
	assert.Error(t, checkPercentagesSum([]float64{40, 40, 40}))
}

func TestStrictJSONUnmarshal(t *testing.T) {
	t.Parallel()
	type someElement struct {
		Data  int               `json:"data"`
		Props map[string]string `json:"props"`
	}

	testCases := []struct {
		data           string
		expectedError  bool
		destination    interface{}
		expectedResult interface{}
	}{
		{``, true, &someElement{}, nil},
		{`123`, true, &someElement{}, nil},
		{`"blah"`, true, &someElement{}, nil},
		{`null`, false, &someElement{}, &someElement{}},
		{
			`{"data": 123, "props": {"test": "mest"}}`, false, &someElement{},
			&someElement{123, map[string]string{"test": "mest"}},
		},
		{`{"data": 123, "props": {"test": "mest"}}asdg`, true, &someElement{}, nil},
	}
	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("TestCase#%d", i), func(t *testing.T) {
			err := strictJSONUnmarshal([]byte(tc.data), &tc.destination)
			if tc.expectedError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, tc.destination)
		})
	}
}
