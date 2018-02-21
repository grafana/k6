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

package stats

import (
	"encoding/json"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
)

func TestNewThreshold(t *testing.T) {
	src := `1+1==2`
	rt := goja.New()
	abortOnFail := false
	th, err := NewThreshold(src, rt, abortOnFail)
	assert.NoError(t, err)

	assert.Equal(t, src, th.Source)
	assert.False(t, th.Failed)
	assert.NotNil(t, th.pgm)
	assert.Equal(t, rt, th.rt)
	assert.Equal(t, abortOnFail, th.AbortOnFail)
}

func TestThresholdRun(t *testing.T) {
	t.Run("true", func(t *testing.T) {
		th, err := NewThreshold(`1+1==2`, goja.New(), false)
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
		th, err := NewThreshold(`1+1==4`, goja.New(), false)
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
			assert.False(t, th.AbortOnFail)
			assert.NotNil(t, th.pgm)
			assert.Equal(t, ts.Runtime, th.rt)
		}
	})
}

func TestNewThresholdsWithConfig(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		ts, err := NewThresholds([]string{})
		assert.NoError(t, err)
		assert.Len(t, ts.Thresholds, 0)
	})
	t.Run("two", func(t *testing.T) {
		configs := []ThresholdConfig{
			{`1+1==2`, false},
			{`1+1==4`, true},
		}
		ts, err := NewThresholdsWithConfig(configs)
		assert.NoError(t, err)
		assert.Len(t, ts.Thresholds, 2)
		for i, th := range ts.Thresholds {
			assert.Equal(t, configs[i].Threshold, th.Source)
			assert.False(t, th.Failed)
			assert.Equal(t, configs[i].AbortOnFail, th.AbortOnFail)
			assert.NotNil(t, th.pgm)
			assert.Equal(t, ts.Runtime, th.rt)
		}
	})
}

func TestThresholdsUpdateVM(t *testing.T) {
	ts, err := NewThresholds(nil)
	assert.NoError(t, err)
	assert.NoError(t, ts.UpdateVM(DummySink{"a": 1234.5}, 0))
	assert.Equal(t, 1234.5, ts.Runtime.Get("a").ToFloat())
}

func TestThresholdsRunAll(t *testing.T) {
	testdata := map[string]struct {
		succ  bool
		err   bool
		abort bool
		srcs  []string
	}{
		"one passing":  {true, false, false, []string{`1+1==2`}},
		"one failing":  {false, false, false, []string{`1+1==4`}},
		"two passing":  {true, false, false, []string{`1+1==2`, `2+2==4`}},
		"two failing":  {false, false, false, []string{`1+1==4`, `2+2==2`}},
		"two mixed":    {false, false, false, []string{`1+1==2`, `1+1==4`}},
		"one erroring": {false, true, false, []string{`throw new Error('?!');`}},
		"one aborting": {false, false, true, []string{`1+1==4`}},
	}

	for name, data := range testdata {
		t.Run(name, func(t *testing.T) {
			ts, err := NewThresholds(data.srcs)
			assert.NoError(t, err)

			if data.abort {
				ts.Thresholds[0].AbortOnFail = true
			}

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

			if data.abort {
				assert.True(t, ts.Abort)
			} else {
				assert.False(t, ts.Abort)
			}
		})
	}
}

func TestThresholdsRun(t *testing.T) {
	ts, err := NewThresholds([]string{"a>0"})
	assert.NoError(t, err)

	t.Run("error", func(t *testing.T) {
		b, err := ts.Run(DummySink{}, 0)
		assert.Error(t, err)
		assert.False(t, b)
	})

	t.Run("pass", func(t *testing.T) {
		b, err := ts.Run(DummySink{"a": 1234.5}, 0)
		assert.NoError(t, err)
		assert.True(t, b)
	})

	t.Run("fail", func(t *testing.T) {
		b, err := ts.Run(DummySink{"a": 0}, 0)
		assert.NoError(t, err)
		assert.False(t, b)
	})
}

func TestThresholdsJSON(t *testing.T) {
	var testdata = []struct {
		JSON        string
		srcs        []string
		abortOnFail bool
		outputJSON  string
	}{
		{`[]`, []string{}, false, ""},
		{`["1+1==2"]`, []string{"1+1==2"}, false, ""},
		{`["1+1==2","1+1==3"]`, []string{"1+1==2", "1+1==3"}, false, ""},

		{`[{"threshold":"1+1==2"}]`, []string{"1+1==2"}, false, `["1+1==2"]`},
		{`[{"threshold":"1+1==2","abortOnFail":true}]`, []string{"1+1==2"}, true, ""},
		{`[{"threshold":"1+1==2","abortOnFail":false}]`, []string{"1+1==2"}, false, `["1+1==2"]`},
		{`[{"threshold":"1+1==2"}, "1+1==3"]`, []string{"1+1==2", "1+1==3"}, false, `["1+1==2","1+1==3"]`},
	}

	for _, data := range testdata {
		t.Run(data.JSON, func(t *testing.T) {
			var ts Thresholds
			assert.NoError(t, json.Unmarshal([]byte(data.JSON), &ts))
			assert.Equal(t, len(data.srcs), len(ts.Thresholds))
			for i, src := range data.srcs {
				assert.Equal(t, src, ts.Thresholds[i].Source)
				assert.Equal(t, data.abortOnFail, ts.Thresholds[i].AbortOnFail)
			}

			t.Run("marshal", func(t *testing.T) {
				data2, err := json.Marshal(ts)
				assert.NoError(t, err)
				output := data.JSON
				if data.outputJSON != "" {
					output = data.outputJSON
				}
				assert.Equal(t, output, string(data2))
			})
		})
	}

	t.Run("bad JSON", func(t *testing.T) {
		var ts Thresholds
		assert.Error(t, json.Unmarshal([]byte("42"), &ts))
		assert.Nil(t, ts.Thresholds)
		assert.Nil(t, ts.Runtime)
		assert.False(t, ts.Abort)
	})

	t.Run("bad source", func(t *testing.T) {
		var ts Thresholds
		assert.Error(t, json.Unmarshal([]byte(`["="]`), &ts))
		assert.Nil(t, ts.Thresholds)
		assert.Nil(t, ts.Runtime)
		assert.False(t, ts.Abort)
	})
}
