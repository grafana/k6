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

	"github.com/loadimpact/k6/stats"
	"github.com/robertkrimen/otto"
	"github.com/stretchr/testify/assert"
)

func TestNewThreshold(t *testing.T) {
	src := `1+1==2`
	vm := otto.New()
	th, err := NewThreshold(src, vm)
	assert.NoError(t, err)

	assert.Equal(t, src, th.Source)
	assert.False(t, th.Failed)
	assert.NotNil(t, th.script)
	assert.Equal(t, vm, th.vm)
}

func TestThresholdRun(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		th, err := NewThreshold(`1+1==2`, otto.New())
		assert.NoError(t, err)

		t.Run("no taint", func(t *testing.T) {
			b, err := th.RunNoTaint()
			assert.NoError(t, err)
			assert.True(t, b)
			assert.False(t, th.Failed)
		})

		t.Run("taint", func(t *testing.T) {
			b, err := th.Run()
			assert.NoError(t, err)
			assert.True(t, b)
			assert.False(t, th.Failed)
		})
	})

	t.Run("false", func(t *testing.T) {
		th, err := NewThreshold(`1+1==4`, otto.New())
		assert.NoError(t, err)

		t.Run("no taint", func(t *testing.T) {
			b, err := th.RunNoTaint()
			assert.NoError(t, err)
			assert.False(t, b)
			assert.False(t, th.Failed)
		})

		t.Run("taint", func(t *testing.T) {
			b, err := th.Run()
			assert.NoError(t, err)
			assert.False(t, b)
			assert.True(t, th.Failed)
		})
	})
}

func TestNewThresholds(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ts, err := NewThresholds([]string{})
		assert.NoError(t, err)
		assert.Len(t, ts.Thresholds, 0)
	})
	t.Run("two", func(t *testing.T) {
		sources := []string{`1+1==2`, `1+1==4`}
		ts, err := NewThresholds(sources)
		assert.NoError(t, err)
		assert.Len(t, ts.Thresholds, 2)
		for i, th := range ts.Thresholds {
			assert.Equal(t, sources[i], th.Source)
			assert.False(t, th.Failed)
			assert.NotNil(t, th.script)
			assert.Equal(t, ts.VM, th.vm)
		}
	})
}

func TestThresholdsUpdateVM(t *testing.T) {
	ts, err := NewThresholds(nil)
	assert.NoError(t, err)
	assert.NoError(t, ts.UpdateVM(stats.DummySink{"a": 1234.5}))

	v, err := ts.VM.Get("a")
	assert.NoError(t, err)
	f, err := v.ToFloat()
	assert.NoError(t, err)
	assert.Equal(t, 1234.5, f)
}

func TestThresholdsRunAll(t *testing.T) {
	testdata := map[string]struct {
		succ bool
		err  bool
		srcs []string
	}{
		"one passing":  {true, false, []string{`1+1==2`}},
		"one failing":  {false, false, []string{`1+1==4`}},
		"two passing":  {true, false, []string{`1+1==2`, `2+2==4`}},
		"two failing":  {false, false, []string{`1+1==4`, `2+2==2`}},
		"two mixed":    {false, false, []string{`1+1==2`, `1+1==4`}},
		"one erroring": {false, true, []string{`throw new Error('?!');`}},
	}

	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			ts, err := NewThresholds(data.srcs)
			assert.NoError(t, err)
			b, err := ts.RunAll()

			if data.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if data.succ {
				assert.True(t, b)
			} else {
				assert.False(t, b)
			}
		})
	}
}

func TestThresholdsRun(t *testing.T) {
	ts, err := NewThresholds([]string{"a>0"})
	assert.NoError(t, err)

	t.Run("error", func(t *testing.T) {
		b, err := ts.Run(stats.DummySink{})
		assert.Error(t, err)
		assert.False(t, b)
	})

	t.Run("pass", func(t *testing.T) {
		b, err := ts.Run(stats.DummySink{"a": 1234.5})
		assert.NoError(t, err)
		assert.True(t, b)
	})

	t.Run("fail", func(t *testing.T) {
		b, err := ts.Run(stats.DummySink{"a": 0})
		assert.NoError(t, err)
		assert.False(t, b)
	})
}

func TestThresholdsJSON(t *testing.T) {
	testdata := map[string][]string{
		`[]`:                  {},
		`["1+1==2"]`:          {"1+1==2"},
		`["1+1==2","1+1==3"]`: {"1+1==2", "1+1==3"},
	}

	for data, srcs := range testdata {
		t.Run(data, func(t *testing.T) {
			var ts Thresholds
			assert.NoError(t, json.Unmarshal([]byte(data), &ts))
			assert.Equal(t, len(srcs), len(ts.Thresholds))
			for i, src := range srcs {
				assert.Equal(t, src, ts.Thresholds[i].Source)
			}

			t.Run("marshal", func(t *testing.T) {
				data2, err := json.Marshal(ts)
				assert.NoError(t, err)
				assert.Equal(t, data, string(data2))
			})
		})
	}
}
