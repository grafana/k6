package k6

import (
	"fmt"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"
)

//nolint:unparam // I am returning *K6 here as this will be used in other tests.
func testSetup(tb testing.TB) (*modulestest.Runtime, *K6) {
	runtime := modulestest.NewRuntime(tb)
	m, ok := New().NewModuleInstance(runtime.VU).(*K6)
	require.True(tb, ok)
	require.NoError(tb, runtime.VU.Runtime().Set("k6", m.Exports().Named))
	return runtime, m
}

// TODO try to get this in modulesTest
func moveToVUCode(tb testing.TB, runtime *modulestest.Runtime) chan metrics.SampleContainer {
	root, err := lib.NewGroup("", nil)
	assert.NoError(tb, err)
	samples := make(chan metrics.SampleContainer, 1000)
	state := &lib.State{
		Samples: samples,
		Tags:    lib.NewVUStateTags(runtime.VU.InitEnvField.Registry.RootTagSet()),
		Options: lib.Options{
			SystemTags: metrics.NewSystemTagSet(metrics.TagGroup),
		},
		BuiltinMetrics: runtime.BuiltinMetrics,
	}
	setGroup(root, state)
	runtime.MoveToVUContext(state)
	return samples
}

func TestAsyncGroup(t *testing.T) {
	t.Parallel()

	cases := []string{
		`
    k6.group("my group", async () => {
        fn("::my group", "");
        await fn("::my group", "");
        fn("::my group", "");
        Promise.resolve("").then( () => {
            fn("")
        })
    }).then(() => {
      fn("");
    })
    fn("");
    `,
		`
    k6.group("my group", async () => {
        fn("::my group", "");
        await fn("::my group", "");
        fn("::my group", "");
        k6.group("second", async() => {
            fn("::my group::second", "my group", "");
            await fn("::my group::second", "my group", "");
            fn("::my group::second", "my group", "");
            await fn("::my group::second", "my group", "");
            fn("::my group::second", "my group", "");
        });
    }).then(() => {
      fn("");
    })
      fn("");
    `,
		`
    k6.group("my group", async () => {
        fn("::my group", "");
        await fn("::my group", "");
        fn("::my group", "");
        k6.group("second", async() => {
            fn("::my group::second", "my group", "");
            await fn("::my group::second", "my group", "");
            fn("::my group::second", "my group", "");
        });
    }).then(() => {
      fn("");
    })
      fn("");
    `,
		`
    k6.group("my group", async () => {
        fn("::my group", "");
        await fn("::my group", "");
        fn("::my group", "");
        k6.group("second", async() => {
            fn("::my group::second", "my group", "");
        });
    }).then(() => {
      fn("");
    })
    `,
		`
    k6.group("my group", async () => {
        fn("::my group", "");
        await fn("::my group", "");
        await k6.group("second", async() => {
            await fn("::my group::second", "my group", "");
        });
    }).then(() => {
      fn("");
    })
    `,
	}
	for i, c := range cases {
		c := c
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Parallel()

			runtime, _ := testSetup(t)
			moveToVUCode(t, runtime)
			rt := runtime.VU.Runtime()
			state := runtime.VU.State()
			root := state.Group
			require.NoError(t, rt.Set("fn", func(expectedGroupTag string, expectedParentNames ...string) *goja.Promise {
				p, res, _ := rt.NewPromise()
				groupTag, ok := state.Tags.GetCurrentValues().Tags.Get("group")
				require.True(t, ok)
				require.Equal(t, expectedGroupTag, groupTag)
				parentGroup := state.Group.Parent
				for _, expectedParentName := range expectedParentNames {
					require.NotNil(t, parentGroup)
					require.Equal(t, expectedParentName, parentGroup.Name)
					parentGroup = parentGroup.Parent
				}
				require.Nil(t, parentGroup)
				res("")
				return p
			}))
			err := runtime.EventLoop.Start(func() error {
				_, err := rt.RunScript("main.js", c)
				return err
			})
			require.NoError(t, err)
			runtime.EventLoop.WaitOnRegistered()
			assert.Equal(t, state.Group, root)
			groupTag, ok := state.Tags.GetCurrentValues().Tags.Get("group")
			require.True(t, ok)
			assert.Equal(t, groupTag, root.Name)
		})
	}
}

func TestAsyncGroupDuration(t *testing.T) {
	t.Parallel()

	runtime, _ := testSetup(t)
	samples := moveToVUCode(t, runtime)
	rt := runtime.VU.Runtime()
	require.NoError(t, rt.Set("delay", func(ms float64) *goja.Promise {
		p, res, _ := rt.NewPromise()
		fn := runtime.VU.RegisterCallback()
		time.AfterFunc(time.Duration(ms*float64(time.Millisecond)), func() {
			fn(func() error {
				res("")
				return nil
			})
		})
		return p
	}))
	err := runtime.EventLoop.Start(func() error {
		_, err := rt.RunScript("main.js", `
        k6.group("1", async () => {
            await delay(100);
            await k6.group("2", async () => {
                await delay(100);
            })
            await delay(100);
        })`)
		return err
	})

	require.NoError(t, err)
	runtime.EventLoop.WaitOnRegistered()
	bufSamples := metrics.GetBufferedSamples(samples)
	require.Len(t, bufSamples, 2)
	{
		firstSample := bufSamples[0].GetSamples()[0]
		require.Equal(t, metrics.GroupDurationName, firstSample.Metric.Name)
		require.Equal(t, "::1::2", firstSample.Tags.Map()[metrics.TagGroup.String()])
		require.InDelta(t, 100, firstSample.Value, 10)
	}

	{
		secondSample := bufSamples[1].GetSamples()[0]
		require.Equal(t, metrics.GroupDurationName, secondSample.Metric.Name)
		require.Equal(t, "::1", secondSample.Tags.Map()[metrics.TagGroup.String()])
		require.InDelta(t, 300, secondSample.Value, 10)
	}
}

func TestAsyncGroupOrder(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		expected []string
		script   string
	}{
		{
			name:     "basic",
			expected: []string{"C", "A", "B"},
			script: `
    k6.group("somename", async () => {
        log("A");
        await 5;
        log("B");
    })
    log("C")`,
		},
		{
			name:     "basic + promise",
			expected: []string{"C", "A", "D", "B"},
			script: `
    k6.group("somename", async () => {
        log("A");
        await 5;
        log("B");
    })
    log("C")
    Promise.resolve("D").then((s) => {log(s)});`,
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			runtime, _ := testSetup(t)
			moveToVUCode(t, runtime)
			rt := runtime.VU.Runtime()
			var s []string
			require.NoError(t, rt.Set("log", func(line string) {
				s = append(s, line)
			}))
			err := runtime.EventLoop.Start(func() error {
				_, err := rt.RunScript("main.js", c.script)
				return err
			})

			require.NoError(t, err)
			runtime.EventLoop.WaitOnRegistered()
			require.Equal(t, c.expected, s)
		})
	}
}
