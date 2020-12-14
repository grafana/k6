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

package k6

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/stats"
)

func TestFail(t *testing.T) {
	rt := goja.New()
	rt.Set("k6", common.Bind(rt, New(), nil))
	_, err := common.RunString(rt, `k6.fail("blah")`)
	assert.Contains(t, err.Error(), "blah")
}

func TestSleep(t *testing.T) {
	rt := goja.New()
	ctx, cancel := context.WithCancel(context.Background())
	rt.Set("k6", common.Bind(rt, New(), &ctx))

	testdata := map[string]time.Duration{
		"1":   1 * time.Second,
		"1.0": 1 * time.Second,
		"0.5": 500 * time.Millisecond,
	}
	for name, d := range testdata {
		t.Run(name, func(t *testing.T) {
			startTime := time.Now()
			_, err := common.RunString(rt, `k6.sleep(1)`)
			endTime := time.Now()
			assert.NoError(t, err)
			assert.True(t, endTime.Sub(startTime) > d, "did not sleep long enough")
		})
	}

	t.Run("Cancel", func(t *testing.T) {
		dch := make(chan time.Duration)
		go func() {
			startTime := time.Now()
			_, err := common.RunString(rt, `k6.sleep(10)`)
			endTime := time.Now()
			assert.NoError(t, err)
			dch <- endTime.Sub(startTime)
		}()
		runtime.Gosched()
		time.Sleep(1 * time.Second)
		runtime.Gosched()
		cancel()
		runtime.Gosched()
		d := <-dch
		assert.True(t, d > 500*time.Millisecond, "did not sleep long enough")
		assert.True(t, d < 2*time.Second, "slept for too long!!")
	})
}

func TestRandSeed(t *testing.T) {
	rt := goja.New()

	ctx := context.Background()
	ctx = common.WithRuntime(ctx, rt)

	rt.Set("k6", common.Bind(rt, New(), &ctx))

	rand := 0.8487305991992138
	_, err := common.RunString(rt, fmt.Sprintf(`
		var rnd = Math.random();
		if (rnd == %.16f) { throw new Error("wrong random: " + rnd); }
	`, rand))
	assert.NoError(t, err)

	_, err = common.RunString(rt, fmt.Sprintf(`
		k6.randomSeed(12345)
		var rnd = Math.random();
		if (rnd != %.16f) { throw new Error("wrong random: " + rnd); }
	`, rand))
	assert.NoError(t, err)
}

func TestGroup(t *testing.T) {
	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	state := &lib.State{Group: root, Samples: make(chan stats.SampleContainer, 1000)}

	ctx := context.Background()
	ctx = lib.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)
	rt.Set("k6", common.Bind(rt, New(), &ctx))

	t.Run("Valid", func(t *testing.T) {
		assert.Equal(t, state.Group, root)
		rt.Set("fn", func() {
			assert.Equal(t, state.Group.Name, "my group")
			assert.Equal(t, state.Group.Parent, root)
		})
		_, err = common.RunString(rt, `k6.group("my group", fn)`)
		assert.NoError(t, err)
		assert.Equal(t, state.Group, root)
	})

	t.Run("Invalid", func(t *testing.T) {
		_, err := common.RunString(rt, `k6.group("::", function() { throw new Error("nooo") })`)
		assert.Contains(t, err.Error(), "group and check names may not contain '::'")
	})
}
func TestCheck(t *testing.T) {
	rt := goja.New()

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	baseCtx := common.WithRuntime(context.Background(), rt)

	ctx := new(context.Context)
	*ctx = baseCtx
	rt.Set("k6", common.Bind(rt, New(), ctx))

	getState := func() (*lib.State, chan stats.SampleContainer) {
		samples := make(chan stats.SampleContainer, 1000)
		return &lib.State{
			Group: root,
			Options: lib.Options{
				SystemTags: &stats.DefaultSystemTagSet,
			},
			Samples: samples,
			Tags:    map[string]string{"group": root.Path},
		}, samples
	}
	t.Run("Object", func(t *testing.T) {
		state, samples := getState()
		*ctx = lib.WithState(baseCtx, state)

		_, err := common.RunString(rt, `k6.check(null, { "check": true })`)
		assert.NoError(t, err)

		bufSamples := stats.GetBufferedSamples(samples)
		if assert.Len(t, bufSamples, 1) {
			sample, ok := bufSamples[0].(stats.Sample)
			require.True(t, ok)

			assert.NotZero(t, sample.Time)
			assert.Equal(t, metrics.Checks, sample.Metric)
			assert.Equal(t, float64(1), sample.Value)
			assert.Equal(t, map[string]string{
				"group": "",
				"check": "check",
			}, sample.Tags.CloneTags())
		}

		t.Run("Multiple", func(t *testing.T) {
			state, samples := getState()
			*ctx = lib.WithState(baseCtx, state)

			_, err := common.RunString(rt, `k6.check(null, { "a": true, "b": false })`)
			assert.NoError(t, err)

			bufSamples := stats.GetBufferedSamples(samples)
			assert.Len(t, bufSamples, 2)
			var foundA, foundB bool
			for _, sampleC := range bufSamples {
				for _, sample := range sampleC.GetSamples() {
					name, ok := sample.Tags.Get("check")
					assert.True(t, ok)
					switch name {
					case "a":
						assert.False(t, foundA, "duplicate 'a'")
						foundA = true
					case "b":
						assert.False(t, foundB, "duplicate 'b'")
						foundB = true
					default:
						assert.Fail(t, name)
					}
				}
			}
			assert.True(t, foundA, "missing 'a'")
			assert.True(t, foundB, "missing 'b'")
		})

		t.Run("Invalid", func(t *testing.T) {
			_, err := common.RunString(rt, `k6.check(null, { "::": true })`)
			assert.Contains(t, err.Error(), "group and check names may not contain '::'")
		})
	})

	t.Run("Array", func(t *testing.T) {
		state, samples := getState()
		*ctx = lib.WithState(baseCtx, state)

		_, err := common.RunString(rt, `k6.check(null, [ true ])`)
		assert.NoError(t, err)

		bufSamples := stats.GetBufferedSamples(samples)
		if assert.Len(t, bufSamples, 1) {
			sample, ok := bufSamples[0].(stats.Sample)
			require.True(t, ok)

			assert.NotZero(t, sample.Time)
			assert.Equal(t, metrics.Checks, sample.Metric)
			assert.Equal(t, float64(1), sample.Value)
			assert.Equal(t, map[string]string{
				"group": "",
				"check": "0",
			}, sample.Tags.CloneTags())
		}
	})

	t.Run("Literal", func(t *testing.T) {
		state, samples := getState()
		*ctx = lib.WithState(baseCtx, state)

		_, err := common.RunString(rt, `k6.check(null, 12345)`)
		assert.NoError(t, err)
		assert.Len(t, stats.GetBufferedSamples(samples), 0)
	})

	t.Run("Throws", func(t *testing.T) {
		state, samples := getState()
		*ctx = lib.WithState(baseCtx, state)

		_, err := common.RunString(rt, `
		k6.check(null, {
			"a": function() { throw new Error("error A") },
			"b": function() { throw new Error("error B") },
		})
		`)
		assert.EqualError(t, err, "Error: error A at <eval>:3:28(4)")

		bufSamples := stats.GetBufferedSamples(samples)
		if assert.Len(t, bufSamples, 1) {
			sample, ok := bufSamples[0].(stats.Sample)
			require.True(t, ok)

			assert.NotZero(t, sample.Time)
			assert.Equal(t, metrics.Checks, sample.Metric)
			assert.Equal(t, float64(0), sample.Value)
			assert.Equal(t, map[string]string{
				"group": "",
				"check": "a",
			}, sample.Tags.CloneTags())
		}
	})

	t.Run("Types", func(t *testing.T) {
		templates := map[string]string{
			"Literal":      `k6.check(null,{"check": %s})`,
			"Callable":     `k6.check(null,{"check": function() { return %s; }})`,
			"Callable/Arg": `k6.check(%s,{"check": function(v) {return v; }})`,
		}
		testdata := map[string]bool{
			`0`:         false,
			`1`:         true,
			`-1`:        true,
			`""`:        false,
			`"true"`:    true,
			`"false"`:   true,
			`true`:      true,
			`false`:     false,
			`null`:      false,
			`undefined`: false,
		}
		for name, tpl := range templates {
			t.Run(name, func(t *testing.T) {
				for value, succ := range testdata {
					t.Run(value, func(t *testing.T) {
						state, samples := getState()
						*ctx = lib.WithState(baseCtx, state)

						v, err := common.RunString(rt, fmt.Sprintf(tpl, value))
						if assert.NoError(t, err) {
							assert.Equal(t, succ, v.Export())
						}

						bufSamples := stats.GetBufferedSamples(samples)
						if assert.Len(t, bufSamples, 1) {
							sample, ok := bufSamples[0].(stats.Sample)
							require.True(t, ok)

							assert.NotZero(t, sample.Time)
							assert.Equal(t, metrics.Checks, sample.Metric)
							if succ {
								assert.Equal(t, float64(1), sample.Value)
							} else {
								assert.Equal(t, float64(0), sample.Value)
							}
							assert.Equal(t, map[string]string{
								"group": "",
								"check": "check",
							}, sample.Tags.CloneTags())
						}
					})
				}
			})
		}

		t.Run("ContextExpiry", func(t *testing.T) {
			root, err := lib.NewGroup("", nil)
			assert.NoError(t, err)

			state := &lib.State{Group: root, Samples: make(chan stats.SampleContainer, 1000)}
			ctx2, cancel := context.WithCancel(lib.WithState(baseCtx, state))
			*ctx = ctx2

			v, err := common.RunString(rt, `k6.check(null, { "check": true })`)
			if assert.NoError(t, err) {
				assert.Equal(t, true, v.Export())
			}

			check, _ := root.Check("check")
			assert.Equal(t, int64(1), check.Passes)
			assert.Equal(t, int64(0), check.Fails)

			cancel()

			v, err = common.RunString(rt, `k6.check(null, { "check": true })`)
			if assert.NoError(t, err) {
				assert.Equal(t, true, v.Export())
			}

			assert.Equal(t, int64(1), check.Passes)
			assert.Equal(t, int64(0), check.Fails)
		})
	})

	t.Run("Tags", func(t *testing.T) {
		state, samples := getState()
		*ctx = lib.WithState(baseCtx, state)

		v, err := common.RunString(rt, `k6.check(null, {"check": true}, {a: 1, b: "2"})`)
		if assert.NoError(t, err) {
			assert.Equal(t, true, v.Export())
		}

		bufSamples := stats.GetBufferedSamples(samples)
		if assert.Len(t, bufSamples, 1) {
			sample, ok := bufSamples[0].(stats.Sample)
			require.True(t, ok)

			assert.NotZero(t, sample.Time)
			assert.Equal(t, metrics.Checks, sample.Metric)
			assert.Equal(t, float64(1), sample.Value)
			assert.Equal(t, map[string]string{
				"group": "",
				"check": "check",
				"a":     "1",
				"b":     "2",
			}, sample.Tags.CloneTags())
		}
	})
}
