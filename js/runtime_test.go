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

package js

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := New()
	assert.NoError(t, err)

	t.Run("Polyfill: Symbol", func(t *testing.T) {
		v, err := r.VM.Get("Symbol")
		assert.NoError(t, err)
		assert.False(t, v.IsUndefined())
	})
}

func TestLoad(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := New()
	assert.NoError(t, err)
	assert.NoError(t, r.VM.Set("__initapi__", &InitAPI{r: r}))

	t.Run("Importing Libraries", func(t *testing.T) {
		_, err := r.load("test.js", []byte(`
			import "k6";
		`))
		assert.NoError(t, err)
		assert.Contains(t, r.lib, "k6.js")
	})
}

func TestExtractOptions(t *testing.T) {
	if testing.Short() {
		return
	}

	r, err := New()
	assert.NoError(t, err)

	t.Run("nothing", func(t *testing.T) {
		_, err := r.load("test.js", []byte(``))
		assert.NoError(t, err)
	})

	t.Run("vus", func(t *testing.T) {
		_, err := r.load("test.js", []byte(`
			export let options = { vus: 12345 };
		`))
		assert.NoError(t, err)

		assert.True(t, r.Options.VUs.Valid)
		assert.Equal(t, int64(12345), r.Options.VUs.Int64)
	})
	t.Run("vusMax", func(t *testing.T) {
		_, err := r.load("test.js", []byte(`
			export let options = { vusMax: 12345 };
		`))
		assert.NoError(t, err)

		assert.True(t, r.Options.VUsMax.Valid)
		assert.Equal(t, int64(12345), r.Options.VUsMax.Int64)
	})
	t.Run("duration", func(t *testing.T) {
		_, err := r.load("test.js", []byte(`
			export let options = { duration: "2m" };
		`))
		assert.NoError(t, err)

		assert.True(t, r.Options.Duration.Valid)
		assert.Equal(t, "2m", r.Options.Duration.String)
	})
	t.Run("maxRedirects", func(t *testing.T) {
		_, err := r.load("test.js", []byte(`
			export let options = { maxRedirects: 12345 };
		`))
		assert.NoError(t, err)

		assert.True(t, r.Options.MaxRedirects.Valid)
		assert.Equal(t, int64(12345), r.Options.MaxRedirects.Int64)
	})
	t.Run("thresholds", func(t *testing.T) {
		_, err := r.load("test.js", []byte(`
			export let options = {
				thresholds: {
					my_metric: ["value<=1000"],
				}
			}
		`))
		assert.NoError(t, err)

		assert.Contains(t, r.Options.Thresholds, "my_metric")
		if assert.Len(t, r.Options.Thresholds["my_metric"].Thresholds, 1) {
			assert.Equal(t, "value<=1000", r.Options.Thresholds["my_metric"].Thresholds[0].Source)
		}
	})
	t.Run("stages", func(t *testing.T) {
		_, err := r.load("test.js", []byte(`
			export let options = {
				stages: [
					{},
					{duration: "10s"},
					{duration: "10s", target: 100},
				]
			}
		`))
		assert.NoError(t, err)

		assert.Len(t, r.Options.Stages, 3)
	})
}

func TestRuntime__ENV(t *testing.T) {
	assert.NoError(t, os.Setenv("TEST_KEY", "abc123"))

	rt, err := New()
	assert.NoError(t, err)
	_, err = rt.VM.Eval(`
	if (__ENV.TEST_KEY != "abc123") {
		throw new Error("Wrong: " + __ENV.TEST_KEY)
	}`)
	assert.NoError(t, err)
}
