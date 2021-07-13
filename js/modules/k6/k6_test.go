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

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/metrics"
	"go.k6.io/k6/stats"
)

func TestFail(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	require.NoError(t, rt.Set("k6", common.Bind(rt, New(), nil)))
	_, err := rt.RunString(`k6.fail("blah")`)
	assert.Contains(t, err.Error(), "blah")
}

func TestSleep(t *testing.T) {
	t.Parallel()

	testdata := map[string]time.Duration{
		"1":   1 * time.Second,
		"1.0": 1 * time.Second,
		"0.5": 500 * time.Millisecond,
	}
	for name, d := range testdata {
		d := d
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rt := goja.New()
			ctx := context.Background()
			require.NoError(t, rt.Set("k6", common.Bind(rt, New(), &ctx)))
			startTime := time.Now()
			_, err := rt.RunString(`k6.sleep(1)`)
			endTime := time.Now()
			assert.NoError(t, err)
			assert.True(t, endTime.Sub(startTime) > d, "did not sleep long enough")
		})
	}

	t.Run("Cancel", func(t *testing.T) {
		t.Parallel()
		rt := goja.New()
		ctx, cancel := context.WithCancel(context.Background())
		require.NoError(t, rt.Set("k6", common.Bind(rt, New(), &ctx)))
		dch := make(chan time.Duration)
		go func() {
			startTime := time.Now()
			_, err := rt.RunString(`k6.sleep(10)`)
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
	t.Parallel()
	rt := goja.New()

	ctx := context.Background()
	ctx = common.WithRuntime(ctx, rt)

	require.NoError(t, rt.Set("k6", common.Bind(rt, New(), &ctx)))

	rand := 0.8487305991992138
	_, err := rt.RunString(fmt.Sprintf(`
		var rnd = Math.random();
		if (rnd == %.16f) { throw new Error("wrong random: " + rnd); }
	`, rand))
	assert.NoError(t, err)

	_, err = rt.RunString(fmt.Sprintf(`
		k6.randomSeed(12345)
		var rnd = Math.random();
		if (rnd != %.16f) { throw new Error("wrong random: " + rnd); }
	`, rand))
	assert.NoError(t, err)
}

func TestGroup(t *testing.T) {
	t.Parallel()
	setupGroupTest := func() (*goja.Runtime, *lib.State, *lib.Group) {
		root, err := lib.NewGroup("", nil)
		assert.NoError(t, err)

		rt := goja.New()
		state := &lib.State{Group: root, Samples: make(chan stats.SampleContainer, 1000)}

		ctx := context.Background()
		ctx = lib.WithState(ctx, state)
		ctx = common.WithRuntime(ctx, rt)
		require.NoError(t, rt.Set("k6", common.Bind(rt, New(), &ctx)))
		return rt, state, root
	}

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()
		rt, state, root := setupGroupTest()
		assert.Equal(t, state.Group, root)
		require.NoError(t, rt.Set("fn", func() {
			assert.Equal(t, state.Group.Name, "my group")
			assert.Equal(t, state.Group.Parent, root)
		}))
		_, err := rt.RunString(`k6.group("my group", fn)`)
		assert.NoError(t, err)
		assert.Equal(t, state.Group, root)
	})

	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()
		rt, _, _ := setupGroupTest()
		_, err := rt.RunString(`k6.group("::", function() { throw new Error("nooo") })`)
		assert.Contains(t, err.Error(), "group and check names may not contain '::'")
	})
}

func checkTestRuntime(t testing.TB, ctxs ...*context.Context) (
	*goja.Runtime, chan stats.SampleContainer,
) {
	rt := goja.New()

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)
	samples := make(chan stats.SampleContainer, 1000)
	state := &lib.State{
		Group: root,
		Options: lib.Options{
			SystemTags: &stats.DefaultSystemTagSet,
		},
		Samples: samples,
		Tags:    map[string]string{"group": root.Path},
	}
	ctx := context.Background()
	if len(ctxs) == 1 { // hacks
		ctx = *ctxs[0]
	}
	ctx = common.WithRuntime(ctx, rt)
	ctx = lib.WithState(ctx, state)
	require.NoError(t, rt.Set("k6", common.Bind(rt, New(), &ctx)))
	if len(ctxs) == 1 { // hacks
		*ctxs[0] = ctx
	}
	return rt, samples
}

func TestCheckObject(t *testing.T) {
	t.Parallel()
	rt, samples := checkTestRuntime(t)

	_, err := rt.RunString(`k6.check(null, { "check": true })`)
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
		t.Parallel()
		rt, samples := checkTestRuntime(t)

		_, err := rt.RunString(`k6.check(null, { "a": true, "b": false })`)
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
		t.Parallel()
		rt, _ := checkTestRuntime(t)
		_, err := rt.RunString(`k6.check(null, { "::": true })`)
		assert.Contains(t, err.Error(), "group and check names may not contain '::'")
	})
}

func TestCheckArray(t *testing.T) {
	t.Parallel()
	rt, samples := checkTestRuntime(t)

	_, err := rt.RunString(`k6.check(null, [ true ])`)
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
}

func TestCheckLiteral(t *testing.T) {
	t.Parallel()
	rt, samples := checkTestRuntime(t)

	_, err := rt.RunString(`k6.check(null, 12345)`)
	assert.NoError(t, err)
	assert.Len(t, stats.GetBufferedSamples(samples), 0)
}

func TestCheckThrows(t *testing.T) {
	t.Parallel()
	rt, samples := checkTestRuntime(t)
	_, err := rt.RunString(`
		k6.check(null, {
			"a": function() { throw new Error("error A") },
			"b": function() { throw new Error("error B") },
		})
		`)
	assert.EqualError(t, err, "Error: error A at a (<eval>:3:28(4))")

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
}

func TestCheckTypes(t *testing.T) {
	t.Parallel()
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
		name, tpl := name, tpl
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			for value, succ := range testdata {
				value, succ := value, succ
				t.Run(value, func(t *testing.T) {
					t.Parallel()
					rt, samples := checkTestRuntime(t)

					v, err := rt.RunString(fmt.Sprintf(tpl, value))
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
}

func TestCheckContextExpiry(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	rt, _ := checkTestRuntime(t, &ctx)
	root := lib.GetState(ctx).Group

	v, err := rt.RunString(`k6.check(null, { "check": true })`)
	if assert.NoError(t, err) {
		assert.Equal(t, true, v.Export())
	}

	check, _ := root.Check("check")
	assert.Equal(t, int64(1), check.Passes)
	assert.Equal(t, int64(0), check.Fails)

	cancel()

	v, err = rt.RunString(`k6.check(null, { "check": true })`)
	if assert.NoError(t, err) {
		assert.Equal(t, true, v.Export())
	}

	assert.Equal(t, int64(1), check.Passes)
	assert.Equal(t, int64(0), check.Fails)
}

func TestCheckTags(t *testing.T) {
	t.Parallel()
	rt, samples := checkTestRuntime(t)

	v, err := rt.RunString(`k6.check(null, {"check": true}, {a: 1, b: "2"})`)
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
}
