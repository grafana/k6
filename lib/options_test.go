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
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
	"testing"
	"time"
)

func TestOptionsApply(t *testing.T) {
	t.Run("Paused", func(t *testing.T) {
		opts := Options{}.Apply(Options{Paused: null.BoolFrom(true)})
		assert.True(t, opts.Paused.Valid)
		assert.True(t, opts.Paused.Bool)
	})
	t.Run("VUs", func(t *testing.T) {
		opts := Options{}.Apply(Options{VUs: null.IntFrom(12345)})
		assert.True(t, opts.VUs.Valid)
		assert.Equal(t, int64(12345), opts.VUs.Int64)
	})
	t.Run("VUsMax", func(t *testing.T) {
		opts := Options{}.Apply(Options{VUsMax: null.IntFrom(12345)})
		assert.True(t, opts.VUsMax.Valid)
		assert.Equal(t, int64(12345), opts.VUsMax.Int64)
	})
	t.Run("Duration", func(t *testing.T) {
		opts := Options{}.Apply(Options{Duration: null.StringFrom("2m")})
		assert.True(t, opts.Duration.Valid)
		assert.Equal(t, "2m", opts.Duration.String)
	})
	t.Run("Stages", func(t *testing.T) {
		opts := Options{}.Apply(Options{Stages: []Stage{Stage{Duration: 1 * time.Second}}})
		assert.NotNil(t, opts.Stages)
		assert.Len(t, opts.Stages, 1)
		assert.Equal(t, 1*time.Second, opts.Stages[0].Duration)
	})
	t.Run("Linger", func(t *testing.T) {
		opts := Options{}.Apply(Options{Linger: null.BoolFrom(true)})
		assert.True(t, opts.Linger.Valid)
		assert.True(t, opts.Linger.Bool)
	})
	t.Run("AbortOnTaint", func(t *testing.T) {
		opts := Options{}.Apply(Options{AbortOnTaint: null.BoolFrom(true)})
		assert.True(t, opts.AbortOnTaint.Valid)
		assert.True(t, opts.AbortOnTaint.Bool)
	})
	t.Run("Acceptance", func(t *testing.T) {
		opts := Options{}.Apply(Options{Acceptance: null.FloatFrom(12345.0)})
		assert.True(t, opts.Acceptance.Valid)
		assert.Equal(t, float64(12345.0), opts.Acceptance.Float64)
	})
	t.Run("MaxRedirects", func(t *testing.T) {
		opts := Options{}.Apply(Options{MaxRedirects: null.IntFrom(12345)})
		assert.True(t, opts.MaxRedirects.Valid)
		assert.Equal(t, int64(12345), opts.MaxRedirects.Int64)
	})
	t.Run("Thresholds", func(t *testing.T) {
		opts := Options{}.Apply(Options{Thresholds: map[string][]string{
			"metric": []string{"1+1==2"},
		}})
		assert.NotNil(t, opts.Thresholds)
		assert.NotEmpty(t, opts.Thresholds)
	})
}
