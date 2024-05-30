package k6

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

func TestFail(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	_, err := tc.testRuntime.RunOnEventLoop(`k6.fail("blah")`)
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
			tc := testCaseRuntime(t)
			startTime := time.Now()
			_, err := tc.testRuntime.RunOnEventLoop(`k6.sleep(1)`)
			endTime := time.Now()
			assert.NoError(t, err)
			assert.True(t, endTime.Sub(startTime) > d, "did not sleep long enough")
		})
	}

	t.Run("Cancel", func(t *testing.T) {
		t.Parallel()

		tc := testCaseRuntime(t)

		dch := make(chan time.Duration)
		go func() {
			startTime := time.Now()
			_, err := tc.testRuntime.RunOnEventLoop(`k6.sleep(10)`)
			endTime := time.Now()
			assert.NoError(t, err)
			dch <- endTime.Sub(startTime)
		}()

		time.Sleep(1 * time.Second)
		tc.testRuntime.CancelContext()
		d := <-dch

		assert.True(t, d > 500*time.Millisecond, "did not sleep long enough")
		assert.True(t, d < 2*time.Second, "slept for too long!!")
	})
}

func TestRandSeed(t *testing.T) {
	t.Parallel()

	tc := testCaseRuntime(t)

	rand := 0.8487305991992138
	_, err := tc.testRuntime.RunOnEventLoop(fmt.Sprintf(`
		var rnd = Math.random();
		if (rnd == %.16f) { throw new Error("wrong random: " + rnd); }
	`, rand))
	assert.NoError(t, err)

	_, err = tc.testRuntime.RunOnEventLoop(fmt.Sprintf(`
		k6.randomSeed(12345)
		var rnd = Math.random();
		if (rnd != %.16f) { throw new Error("wrong random: " + rnd); }
	`, rand))
	assert.NoError(t, err)
}

func TestGroup(t *testing.T) {
	t.Parallel()

	t.Run("Valid", func(t *testing.T) {
		t.Parallel()
		tc := testCaseRuntime(t)
		state := tc.testRuntime.VU.State()
		require.NoError(t, tc.testRuntime.VU.Runtime().Set("fn", func() {
			groupTag, ok := state.Tags.GetCurrentValues().Tags.Get("group")
			require.True(t, ok)
			assert.Equal(t, groupTag, "::my group")
		}))
		_, err := tc.testRuntime.RunOnEventLoop(`k6.group("my group", fn)`)
		assert.NoError(t, err)
		groupTag, ok := state.Tags.GetCurrentValues().Tags.Get("group")
		require.True(t, ok)
		assert.Equal(t, groupTag, "")
	})

	t.Run("Invalid", func(t *testing.T) {
		t.Parallel()
		tc := testCaseRuntime(t)
		_, err := tc.testRuntime.RunOnEventLoop(`k6.group("::", function() { throw new Error("nooo") })`)
		assert.Contains(t, err.Error(), "group and check names may not contain '::'")
	})

	t.Run("async function", func(t *testing.T) {
		t.Parallel()
		tc := testCaseRuntime(t)
		_, err := tc.testRuntime.RunOnEventLoop(`k6.group("something", async function() { })`)
		assert.ErrorContains(t, err, "group() does not support async functions as arguments")
	})

	t.Run("async lambda", func(t *testing.T) {
		t.Parallel()
		tc := testCaseRuntime(t)
		_, err := tc.testRuntime.RunOnEventLoop(`k6.group("something", async () => { })`)
		assert.ErrorContains(t, err, "group() does not support async functions as arguments")
	})
}

func TestCheckObject(t *testing.T) {
	t.Parallel()
	t.Run("boolean", func(t *testing.T) {
		t.Parallel()
		tc := testCaseRuntime(t)

		_, err := tc.testRuntime.RunOnEventLoop(`k6.check(null, { "check": true })`)
		assert.NoError(t, err)

		bufSamples := metrics.GetBufferedSamples(tc.samples)
		require.Len(t, bufSamples, 1)
		sample, ok := bufSamples[0].(metrics.Sample)
		require.True(t, ok)

		assert.NotZero(t, sample.Time)
		assert.Equal(t, tc.testRuntime.VU.State().BuiltinMetrics.Checks, sample.Metric)
		assert.Equal(t, float64(1), sample.Value)
		assert.Equal(t, map[string]string{
			"group": "",
			"check": "check",
		}, sample.Tags.Map())
	})

	t.Run("Multiple", func(t *testing.T) {
		t.Parallel()
		tc := testCaseRuntime(t)

		_, err := tc.testRuntime.RunOnEventLoop(`k6.check(null, { "a": true, "b": false })`)
		assert.NoError(t, err)

		bufSamples := metrics.GetBufferedSamples(tc.samples)
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
		tc := testCaseRuntime(t)
		_, err := tc.testRuntime.RunOnEventLoop(`k6.check(null, { "::": true })`)
		assert.Contains(t, err.Error(), "group and check names may not contain '::'")
	})

	t.Run("async function", func(t *testing.T) {
		t.Parallel()
		tc := testCaseRuntime(t)
		_, err := tc.testRuntime.RunOnEventLoop(`k6.check("something", {"async": async function() { }})`)
		assert.ErrorContains(t, err, "check() does not support async functions as arguments")
	})

	t.Run("async lambda", func(t *testing.T) {
		t.Parallel()
		tc := testCaseRuntime(t)
		_, err := tc.testRuntime.RunOnEventLoop(`k6.check("something", {"async": async () =>{ }})`)
		assert.ErrorContains(t, err, "check() does not support async functions as arguments")
	})
}

func TestCheckArray(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	_, err := tc.testRuntime.RunOnEventLoop(`k6.check(null, [ true ])`)
	assert.NoError(t, err)

	bufSamples := metrics.GetBufferedSamples(tc.samples)
	require.Len(t, bufSamples, 1)
	sample, ok := bufSamples[0].(metrics.Sample)
	require.True(t, ok)

	assert.NotZero(t, sample.Time)
	assert.Equal(t, tc.testRuntime.VU.State().BuiltinMetrics.Checks, sample.Metric)
	assert.Equal(t, float64(1), sample.Value)
	assert.Equal(t, map[string]string{
		"group": "",
		"check": "0",
	}, sample.Tags.Map())
}

func TestCheckContextDone(t *testing.T) {
	t.Parallel()
	t.Run("true", func(t *testing.T) {
		t.Parallel()
		tc := testCaseRuntime(t)

		tc.testRuntime.CancelContext()
		v, err := tc.testRuntime.RunOnEventLoop(` k6.check(null, {"name": ()=>{ return true }})`)
		assert.NoError(t, err)
		assert.Len(t, metrics.GetBufferedSamples(tc.samples), 0)
		assert.True(t, v.ToBoolean())
	})
	t.Run("false", func(t *testing.T) {
		t.Parallel()
		tc := testCaseRuntime(t)

		tc.testRuntime.CancelContext()
		v, err := tc.testRuntime.RunOnEventLoop(`k6.check(null, {"name": ()=>{ return false }})`)
		assert.NoError(t, err)
		assert.Len(t, metrics.GetBufferedSamples(tc.samples), 0)
		assert.False(t, v.ToBoolean())
	})
}

func TestCheckLiteral(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	_, err := tc.testRuntime.RunOnEventLoop(`k6.check(null, 12345)`)
	assert.NoError(t, err)
	assert.Len(t, metrics.GetBufferedSamples(tc.samples), 0)
}

func TestCheckNull(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	_, err := tc.testRuntime.RunOnEventLoop(`k6.check(5)`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no checks provided")
	assert.Len(t, metrics.GetBufferedSamples(tc.samples), 0)
}

func TestCheckThrows(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)
	_, err := tc.testRuntime.RunOnEventLoop(`
		k6.check(null, {
			"a": function() { throw new Error("error A") },
			"b": function() { throw new Error("error B") },
		})
		`)
	assert.EqualError(t, err, "Error: error A at a (<eval>:3:28(3))")

	bufSamples := metrics.GetBufferedSamples(tc.samples)
	require.Len(t, bufSamples, 1)
	sample, ok := bufSamples[0].(metrics.Sample)
	require.True(t, ok)

	assert.NotZero(t, sample.Time)
	assert.Equal(t, tc.testRuntime.VU.State().BuiltinMetrics.Checks, sample.Metric)
	assert.Equal(t, float64(0), sample.Value)
	assert.Equal(t, map[string]string{
		"group": "",
		"check": "a",
	}, sample.Tags.Map())
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
					tc := testCaseRuntime(t)

					v, err := tc.testRuntime.RunOnEventLoop(fmt.Sprintf(tpl, value))
					require.NoError(t, err)
					assert.Equal(t, succ, v.Export())

					bufSamples := metrics.GetBufferedSamples(tc.samples)
					require.Len(t, bufSamples, 1)
					sample, ok := bufSamples[0].(metrics.Sample)
					require.True(t, ok)

					assert.NotZero(t, sample.Time)
					assert.Equal(t, tc.testRuntime.VU.State().BuiltinMetrics.Checks, sample.Metric)
					if succ {
						assert.Equal(t, float64(1), sample.Value)
					} else {
						assert.Equal(t, float64(0), sample.Value)
					}
					assert.Equal(t, map[string]string{
						"group": "",
						"check": "check",
					}, sample.Tags.Map())
				})
			}
		})
	}
}

func TestCheckTags(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	v, err := tc.testRuntime.RunOnEventLoop(`k6.check(null, {"check": true}, {a: 1, b: "2"})`)
	require.NoError(t, err)
	assert.Equal(t, true, v.Export())

	bufSamples := metrics.GetBufferedSamples(tc.samples)
	require.Len(t, bufSamples, 1)
	sample, ok := bufSamples[0].(metrics.Sample)
	require.True(t, ok)

	assert.NotZero(t, sample.Time)
	assert.Equal(t, tc.testRuntime.VU.State().BuiltinMetrics.Checks, sample.Metric)
	assert.Equal(t, float64(1), sample.Value)
	assert.Equal(t, map[string]string{
		"group": "",
		"check": "check",
		"a":     "1",
		"b":     "2",
	}, sample.Tags.Map())
}

type testCase struct {
	samples     chan metrics.SampleContainer
	testRuntime *modulestest.Runtime
}

func testCaseRuntime(t testing.TB) *testCase {
	testRuntime := modulestest.NewRuntime(t)
	m, ok := New().NewModuleInstance(testRuntime.VU).(*K6)
	require.True(t, ok)
	require.NoError(t, testRuntime.VU.RuntimeField.Set("k6", m.Exports().Named))

	registry := metrics.NewRegistry()
	samples := make(chan metrics.SampleContainer, 1000)
	state := &lib.State{
		Options: lib.Options{
			SystemTags: &metrics.DefaultSystemTagSet,
		},
		Samples:        samples,
		Tags:           lib.NewVUStateTags(registry.RootTagSet().WithTagsFromMap(map[string]string{"group": lib.RootGroupPath})),
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
	}
	testRuntime.MoveToVUContext(state)

	return &testCase{
		samples:     samples,
		testRuntime: testRuntime,
	}
}
