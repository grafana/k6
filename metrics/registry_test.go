/*
 *
 * k6 - a next-generation load testing tool
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

package metrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryNewMetric(t *testing.T) {
	t.Parallel()
	r := NewRegistry()

	somethingCounter, err := r.NewMetric("something", Counter)
	require.NoError(t, err)
	require.Equal(t, "something", somethingCounter.Name)

	somethingCounterAgain, err := r.NewMetric("something", Counter)
	require.NoError(t, err)
	require.Equal(t, "something", somethingCounterAgain.Name)
	require.Same(t, somethingCounter, somethingCounterAgain)

	_, err = r.NewMetric("something", Gauge)
	require.Error(t, err)

	_, err = r.NewMetric("something", Counter, Time)
	require.Error(t, err)
}

func TestMetricNames(t *testing.T) {
	t.Parallel()
	testMap := map[string]bool{
		"simple":       true,
		"still_simple": true,
		"":             false,
		"@":            false,
		"a":            true,
		"special\n\t":  false,
		// this has both hangul and japanese numerals .
		"hello.World_in_한글一안녕一세상": true,
		// too long
		"tooolooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooooog": false,
	}

	for key, value := range testMap {
		key, value := key, value
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, value, checkName(key), key)
		})
	}
}

func TestRegistryBranchTagSetRootWith(t *testing.T) {
	t.Parallel()

	raw := map[string]string{
		"key":  "val",
		"key2": "val2",
	}

	r := NewRegistry()
	tags := r.BranchTagSetRootWith(raw)
	require.NotNil(t, tags)

	assert.Equal(t, raw, tags.Map())
}
