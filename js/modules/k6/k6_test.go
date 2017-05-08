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
	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/stretchr/testify/assert"
)

func TestFail(t *testing.T) {
	rt := goja.New()
	rt.Set("k6", common.Bind(rt, &K6{}, nil))
	_, err := common.RunString(rt, `k6.fail("blah")`)
	assert.EqualError(t, err, "GoError: blah")
}

func TestSleep(t *testing.T) {
	rt := goja.New()
	ctx, cancel := context.WithCancel(context.Background())
	rt.Set("k6", common.Bind(rt, &K6{}, &ctx))

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

func TestGroup(t *testing.T) {
	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	rt := goja.New()
	state := &common.State{Group: root}

	ctx := context.Background()
	ctx = common.WithState(ctx, state)
	ctx = common.WithRuntime(ctx, rt)
	rt.Set("k6", common.Bind(rt, &K6{}, &ctx))

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
		assert.EqualError(t, err, "GoError: group and check names may not contain '::'")
	})
}

func TestCheck(t *testing.T) {
	rt := goja.New()

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)

	baseCtx := common.WithRuntime(context.Background(), rt)

	ctx := new(context.Context)
	*ctx = baseCtx
	rt.Set("k6", common.Bind(rt, &K6{}, ctx))

	t.Run("Object", func(t *testing.T) {
		state := &common.State{Group: root}
		*ctx = common.WithState(baseCtx, state)

		_, err := common.RunString(rt, `k6.check(null, { "check": true })`)
		assert.NoError(t, err)

		if assert.Len(t, state.Samples, 1) {
			assert.NotZero(t, state.Samples[0].Time)
			assert.Equal(t, metrics.Checks, state.Samples[0].Metric)
			assert.Equal(t, float64(1), state.Samples[0].Value)
			assert.Equal(t, map[string]string{
				"group": "",
				"check": "check",
			}, state.Samples[0].Tags)
		}

		t.Run("Invalid", func(t *testing.T) {
			_, err := common.RunString(rt, `k6.check(null, { "::": true })`)
			assert.EqualError(t, err, "GoError: group and check names may not contain '::'")
		})
	})
	t.Run("Array", func(t *testing.T) {
		state := &common.State{Group: root}
		*ctx = common.WithState(baseCtx, state)

		_, err := common.RunString(rt, `k6.check(null, [ true ])`)
		assert.NoError(t, err)

		if assert.Len(t, state.Samples, 1) {
			assert.NotZero(t, state.Samples[0].Time)
			assert.Equal(t, metrics.Checks, state.Samples[0].Metric)
			assert.Equal(t, float64(1), state.Samples[0].Value)
			assert.Equal(t, map[string]string{
				"group": "",
				"check": "0",
			}, state.Samples[0].Tags)
		}
	})
	t.Run("Literal", func(t *testing.T) {
		_, err := common.RunString(rt, `k6.check(null, null)`)
		assert.EqualError(t, err, "TypeError: Cannot convert undefined or null to object")
	})

	t.Run("Throws", func(t *testing.T) {
		_, err := common.RunString(rt, `
		k6.check(null, {
			"a": function() { throw new Error("error A") },
			"b": function() { throw new Error("error B") },
		})
		`)
		assert.EqualError(t, err, "Error: error A at a (<eval>:3:27(6))")
	})

	t.Run("Types", func(t *testing.T) {
		templates := map[string]string{
			"Literal":      `k6.check(null,{"check": %s})`,
			"Callable":     `k6.check(null,{"check": ()=>%s})`,
			"Callable/Arg": `k6.check(%s,{"check":(v)=>v})`,
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
						state := &common.State{Group: root}
						*ctx = common.WithState(baseCtx, state)

						v, err := common.RunString(rt, fmt.Sprintf(tpl, value))
						if assert.NoError(t, err) {
							assert.Equal(t, succ, v.Export())
						}

						if assert.Len(t, state.Samples, 1) {
							assert.NotZero(t, state.Samples[0].Time)
							assert.Equal(t, metrics.Checks, state.Samples[0].Metric)
							if succ {
								assert.Equal(t, float64(1), state.Samples[0].Value)
							} else {
								assert.Equal(t, float64(0), state.Samples[0].Value)
							}
							assert.Equal(t, map[string]string{
								"group": "",
								"check": "check",
							}, state.Samples[0].Tags)
						}
					})
				}
			})
		}
	})

	t.Run("Tags", func(t *testing.T) {
		state := &common.State{Group: root}
		*ctx = common.WithState(baseCtx, state)

		v, err := common.RunString(rt, `k6.check(null, {"check": true}, {a: 1, b: "2"})`)
		if assert.NoError(t, err) {
			assert.Equal(t, true, v.Export())
		}

		if assert.Len(t, state.Samples, 1) {
			assert.NotZero(t, state.Samples[0].Time)
			assert.Equal(t, metrics.Checks, state.Samples[0].Metric)
			assert.Equal(t, float64(1), state.Samples[0].Value)
			assert.Equal(t, map[string]string{
				"group": "",
				"check": "check",
				"a":     "1",
				"b":     "2",
			}, state.Samples[0].Tags)
		}
	})
}
