package k6

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func TestFail(t *testing.T) {
	t.Parallel()
	rt := goja.New()
	m, ok := New().NewModuleInstance(
		&modulestest.VU{
			RuntimeField: rt,
			InitEnvField: &common.InitEnvironment{},
			CtxField:     context.Background(),
			StateField:   nil,
		},
	).(*K6)
	require.True(t, ok)
	require.NoError(t, rt.Set("k6", m.Exports().Named))

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
			m, ok := New().NewModuleInstance(
				&modulestest.VU{
					RuntimeField: rt,
					InitEnvField: &common.InitEnvironment{},
					CtxField:     context.Background(),
					StateField:   nil,
				},
			).(*K6)
			require.True(t, ok)
			require.NoError(t, rt.Set("k6", m.Exports().Named))

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
		m, ok := New().NewModuleInstance(
			&modulestest.VU{
				RuntimeField: rt,
				InitEnvField: &common.InitEnvironment{},
				CtxField:     ctx,
				StateField:   nil,
			},
		).(*K6)
		require.True(t, ok)
		require.NoError(t, rt.Set("k6", m.Exports().Named))

		dch := make(chan time.Duration)
		go func() {
			startTime := time.Now()
			_, err := rt.RunString(`k6.sleep(10)`)
			endTime := time.Now()
			assert.NoError(t, err)
			dch <- endTime.Sub(startTime)
		}()

		time.Sleep(1 * time.Second)
		cancel()
		d := <-dch

		assert.True(t, d > 500*time.Millisecond, "did not sleep long enough")
		assert.True(t, d < 2*time.Second, "slept for too long!!")
	})
}

func TestRandSeed(t *testing.T) {
	t.Parallel()
	rt := goja.New()

	m, ok := New().NewModuleInstance(
		&modulestest.VU{
			RuntimeField: rt,
			InitEnvField: &common.InitEnvironment{},
			CtxField:     context.Background(),
			StateField:   nil,
		},
	).(*K6)
	require.True(t, ok)
	require.NoError(t, rt.Set("k6", m.Exports().Named))

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
		state := &lib.State{
			Group:   root,
			Samples: make(chan metrics.SampleContainer, 1000),
			Tags:    lib.NewTagMap(nil),
			Options: lib.Options{
				SystemTags: metrics.NewSystemTagSet(metrics.TagGroup),
			},
		}
		state.BuiltinMetrics = metrics.RegisterBuiltinMetrics(metrics.NewRegistry())

		m, ok := New().NewModuleInstance(
			&modulestest.VU{
				RuntimeField: rt,
				CtxField:     context.Background(),
				StateField:   state,
			},
		).(*K6)
		require.True(t, ok)
		require.NoError(t, rt.Set("k6", m.Exports().Named))
		return rt, state, root
	}

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()
		rt, state, root := setupGroupTest()
		assert.Equal(t, state.Group, root)
		require.NoError(t, rt.Set("fn", func() {
			groupTag, ok := state.Tags.Get("group")
			require.True(t, ok)
			assert.Equal(t, groupTag, "::my group")
			assert.Equal(t, state.Group.Name, "my group")
			assert.Equal(t, state.Group.Parent, root)
		}))
		_, err := rt.RunString(`k6.group("my group", fn)`)
		assert.NoError(t, err)
		assert.Equal(t, state.Group, root)
		groupTag, ok := state.Tags.Get("group")
		require.True(t, ok)
		assert.Equal(t, groupTag, root.Name)
	})

	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()
		rt, _, _ := setupGroupTest()
		_, err := rt.RunString(`k6.group("::", function() { throw new Error("nooo") })`)
		assert.Contains(t, err.Error(), "group and check names may not contain '::'")
	})
}

func checkTestRuntime(t testing.TB) (*goja.Runtime, chan metrics.SampleContainer, *metrics.BuiltinMetrics) {
	rt := goja.New()

	test := modulestest.NewRuntime(t)
	m, ok := New().NewModuleInstance(test.VU).(*K6)
	require.True(t, ok)
	require.NoError(t, rt.Set("k6", m.Exports().Named))

	root, err := lib.NewGroup("", nil)
	assert.NoError(t, err)
	samples := make(chan metrics.SampleContainer, 1000)
	state := &lib.State{
		Group: root,
		Options: lib.Options{
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		Samples: samples,
		Tags: lib.NewTagMap(map[string]string{
			"group": root.Path,
		}),
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(metrics.NewRegistry()),
	}
	test.MoveToVUContext(state)

	return rt, samples, state.BuiltinMetrics
}

func TestCheckObject(t *testing.T) {
	t.Parallel()
	rt, samples, builtinMetrics := checkTestRuntime(t)

	_, err := rt.RunString(`k6.check(null, { "check": true })`)
	assert.NoError(t, err)

	bufSamples := metrics.GetBufferedSamples(samples)
	if assert.Len(t, bufSamples, 1) {
		sample, ok := bufSamples[0].(metrics.Sample)
		require.True(t, ok)

		assert.NotZero(t, sample.Time)
		assert.Equal(t, builtinMetrics.Checks, sample.Metric)
		assert.Equal(t, float64(1), sample.Value)
		assert.Equal(t, map[string]string{
			"group": "",
			"check": "check",
		}, sample.Tags.CloneTags())
	}

	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		rt, samples, _ := checkTestRuntime(t)

		_, err := rt.RunString(`k6.check(null, { "a": true, "b": false })`)
		assert.NoError(t, err)

		bufSamples := metrics.GetBufferedSamples(samples)
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
		rt, _, _ := checkTestRuntime(t)
		_, err := rt.RunString(`k6.check(null, { "::": true })`)
		assert.Contains(t, err.Error(), "group and check names may not contain '::'")
	})
}

func TestCheckArray(t *testing.T) {
	t.Parallel()
	rt, samples, builtinMetrics := checkTestRuntime(t)

	_, err := rt.RunString(`k6.check(null, [ true ])`)
	assert.NoError(t, err)

	bufSamples := metrics.GetBufferedSamples(samples)
	if assert.Len(t, bufSamples, 1) {
		sample, ok := bufSamples[0].(metrics.Sample)
		require.True(t, ok)

		assert.NotZero(t, sample.Time)
		assert.Equal(t, builtinMetrics.Checks, sample.Metric)
		assert.Equal(t, float64(1), sample.Value)
		assert.Equal(t, map[string]string{
			"group": "",
			"check": "0",
		}, sample.Tags.CloneTags())
	}
}

func TestCheckLiteral(t *testing.T) {
	t.Parallel()
	rt, samples, _ := checkTestRuntime(t)

	_, err := rt.RunString(`k6.check(null, 12345)`)
	assert.NoError(t, err)
	assert.Len(t, metrics.GetBufferedSamples(samples), 0)
}

func TestCheckNull(t *testing.T) {
	t.Parallel()
	rt, samples, _ := checkTestRuntime(t)

	_, err := rt.RunString(`k6.check(5)`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no checks provided")
	assert.Len(t, metrics.GetBufferedSamples(samples), 0)
}

func TestCheckThrows(t *testing.T) {
	t.Parallel()
	rt, samples, builtinMetrics := checkTestRuntime(t)
	_, err := rt.RunString(`
		k6.check(null, {
			"a": function() { throw new Error("error A") },
			"b": function() { throw new Error("error B") },
		})
		`)
	assert.EqualError(t, err, "Error: error A at a (<eval>:3:28(3))")

	bufSamples := metrics.GetBufferedSamples(samples)
	if assert.Len(t, bufSamples, 1) {
		sample, ok := bufSamples[0].(metrics.Sample)
		require.True(t, ok)

		assert.NotZero(t, sample.Time)
		assert.Equal(t, builtinMetrics.Checks, sample.Metric)
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
					rt, samples, builtinMetrics := checkTestRuntime(t)

					v, err := rt.RunString(fmt.Sprintf(tpl, value))
					if assert.NoError(t, err) {
						assert.Equal(t, succ, v.Export())
					}

					bufSamples := metrics.GetBufferedSamples(samples)
					if assert.Len(t, bufSamples, 1) {
						sample, ok := bufSamples[0].(metrics.Sample)
						require.True(t, ok)

						assert.NotZero(t, sample.Time)
						assert.Equal(t, builtinMetrics.Checks, sample.Metric)
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

	rt := goja.New()
	ctx, cancel := context.WithCancel(context.Background())
	root, err := lib.NewGroup("", nil)
	require.NoError(t, err)

	samples := make(chan metrics.SampleContainer, 1000)
	state := &lib.State{
		Group: root,
		Options: lib.Options{
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		Samples: samples,
		Tags: lib.NewTagMap(map[string]string{
			"group": root.Path,
		}),
	}

	state.BuiltinMetrics = metrics.RegisterBuiltinMetrics(metrics.NewRegistry())
	m, ok := New().NewModuleInstance(
		&modulestest.VU{
			RuntimeField: rt,
			CtxField:     ctx,
			StateField:   state,
		},
	).(*K6)
	require.True(t, ok)
	require.NoError(t, rt.Set("k6", m.Exports().Named))

	v, err := rt.RunString(`k6.check(null, { "check": true })`)
	if assert.NoError(t, err) {
		assert.Equal(t, true, v.Export())
	}

	check, _ := root.Check("check")
	assert.Equal(t, int64(1), check.Passes)
	assert.Equal(t, int64(0), check.Fails)

	cancel()

	v, err = rt.RunString(`k6.check(null, { "check": true })`)
	require.NoError(t, err)
	assert.Equal(t, true, v.Export())

	assert.Equal(t, int64(1), check.Passes)
	assert.Equal(t, int64(0), check.Fails)
}

func TestCheckTags(t *testing.T) {
	t.Parallel()
	rt, samples, builtinMetrics := checkTestRuntime(t)

	v, err := rt.RunString(`k6.check(null, {"check": true}, {a: 1, b: "2"})`)
	if assert.NoError(t, err) {
		assert.Equal(t, true, v.Export())
	}

	bufSamples := metrics.GetBufferedSamples(samples)
	if assert.Len(t, bufSamples, 1) {
		sample, ok := bufSamples[0].(metrics.Sample)
		require.True(t, ok)

		assert.NotZero(t, sample.Time)
		assert.Equal(t, builtinMetrics.Checks, sample.Metric)
		assert.Equal(t, float64(1), sample.Value)
		assert.Equal(t, map[string]string{
			"group": "",
			"check": "check",
			"a":     "1",
			"b":     "2",
		}, sample.Tags.CloneTags())
	}
}
