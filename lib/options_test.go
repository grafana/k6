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
	"time"

	"crypto/tls"

	"github.com/loadimpact/k6/stats"
	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v3"
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
		opts := Options{}.Apply(Options{Duration: NullDurationFrom(2 * time.Minute)})
		assert.True(t, opts.Duration.Valid)
		assert.Equal(t, "2m0s", opts.Duration.String())
	})
	t.Run("Iterations", func(t *testing.T) {
		opts := Options{}.Apply(Options{Iterations: null.IntFrom(1234)})
		assert.True(t, opts.Iterations.Valid)
		assert.Equal(t, int64(1234), opts.Iterations.Int64)
	})
	t.Run("Stages", func(t *testing.T) {
		opts := Options{}.Apply(Options{Stages: []Stage{{Duration: NullDurationFrom(1 * time.Second)}}})
		assert.NotNil(t, opts.Stages)
		assert.Len(t, opts.Stages, 1)
		assert.Equal(t, 1*time.Second, time.Duration(opts.Stages[0].Duration.Duration))
	})
	t.Run("Linger", func(t *testing.T) {
		opts := Options{}.Apply(Options{Linger: null.BoolFrom(true)})
		assert.True(t, opts.Linger.Valid)
		assert.True(t, opts.Linger.Bool)
	})
	t.Run("MaxRedirects", func(t *testing.T) {
		opts := Options{}.Apply(Options{MaxRedirects: null.IntFrom(12345)})
		assert.True(t, opts.MaxRedirects.Valid)
		assert.Equal(t, int64(12345), opts.MaxRedirects.Int64)
	})
	t.Run("InsecureSkipTLSVerify", func(t *testing.T) {
		opts := Options{}.Apply(Options{InsecureSkipTLSVerify: null.BoolFrom(true)})
		assert.True(t, opts.InsecureSkipTLSVerify.Valid)
		assert.True(t, opts.InsecureSkipTLSVerify.Bool)
	})
	t.Run("TLSCipherSuites", func(t *testing.T) {
		for suiteName, suiteID := range SupportedTLSCipherSuites {
			t.Run(suiteName, func(t *testing.T) {
				opts := Options{}.Apply(Options{TLSCipherSuites: &TLSCipherSuites{[]uint16{suiteID}}})

				assert.NotNil(t, opts.TLSCipherSuites.Values)
				assert.Len(t, opts.TLSCipherSuites.Values, 1)
				assert.Equal(t, suiteID, opts.TLSCipherSuites.Values[0])
			})
		}
	})
	t.Run("TLSVersion", func(t *testing.T) {
		version := TLSVersion{Min: tls.VersionSSL30, Max: tls.VersionTLS12}
		opts := Options{}.Apply(Options{TLSVersion: &version})

		assert.NotNil(t, opts.TLSVersion)
		assert.Equal(t, opts.TLSVersion.Min, tls.VersionSSL30)
		assert.Equal(t, opts.TLSVersion.Max, tls.VersionTLS12)
	})
	t.Run("NoConnectionReuse", func(t *testing.T) {
		opts := Options{}.Apply(Options{NoConnectionReuse: null.BoolFrom(true)})
		assert.True(t, opts.NoConnectionReuse.Valid)
		assert.True(t, opts.NoConnectionReuse.Bool)
	})
	t.Run("Thresholds", func(t *testing.T) {
		opts := Options{}.Apply(Options{Thresholds: map[string]stats.Thresholds{
			"metric": {
				Thresholds: []*stats.Threshold{{}},
			},
		}})
		assert.NotNil(t, opts.Thresholds)
		assert.NotEmpty(t, opts.Thresholds)
	})
	t.Run("External", func(t *testing.T) {
		opts := Options{}.Apply(Options{External: map[string]interface{}{"a": 1}})
		assert.Equal(t, map[string]interface{}{"a": 1}, opts.External)
	})
	t.Run("NoUsageReport", func(t *testing.T) {
		opts := Options{}.Apply(Options{NoUsageReport: null.BoolFrom(true)})
		assert.True(t, opts.NoUsageReport.Valid)
		assert.True(t, opts.NoUsageReport.Bool)
	})
}
