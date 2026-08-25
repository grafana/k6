package k6

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

func checkGroups(t *testing.T, tc *testCase) map[string]string {
	t.Helper()

	checks := make(map[string]string)
	checksMetric := tc.testRuntime.VU.State().BuiltinMetrics.Checks
	for _, sampleContainer := range metrics.GetBufferedSamples(tc.samples) {
		for _, sample := range sampleContainer.GetSamples() {
			if sample.Metric != checksMetric {
				continue
			}
			name, ok := sample.Tags.Get(metrics.TagCheck.String())
			require.True(t, ok)
			_, duplicate := checks[name]
			require.Falsef(t, duplicate, "duplicate check sample %q", name)
			assert.Equal(t, float64(1), sample.Value, "check %q failed", name)
			group, _ := sample.Tags.Get(metrics.TagGroup.String())
			checks[name] = group
		}
	}
	return checks
}

func newOrderRecorder(t *testing.T, tc *testCase) *[]string {
	t.Helper()
	order := new([]string)
	require.NoError(t, tc.testRuntime.VU.Runtime().Set("mark", func(s string) {
		*order = append(*order, s)
	}))
	return order
}

// Regression test for #2728 and #3392.
func TestThenContinuationKeepsGroup(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	_, err := tc.testRuntime.RunOnEventLoop(`
		(async () => {
			const delay = () => { return Promise.resolve(); };
			await k6.group("coolgroup", () => {
				k6.check(null, {sync: true});
				return delay(1).then(() => { k6.check(null, {continuation: true}); });
			});
			k6.check(null, {after: true});
		})()
	`)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		"sync":         "::coolgroup",
		"continuation": "::coolgroup",
		"after":        lib.RootGroupPath,
	}, checkGroups(t, tc))
}

// Regression test for #2728's manual group re-entry workaround.
func TestManualGroupInThenContinuation(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	_, err := tc.testRuntime.RunOnEventLoop(`
		(async () => {
			const delay = () => { return Promise.resolve(); };
			await delay(1).then(() => {
				k6.check(null, {before: true});
				return k6.group("coolgroup", () => { k6.check(null, {inside: true}); });
			});
			k6.check(null, {after: true});
		})()
	`)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		"before": lib.RootGroupPath,
		"inside": "::coolgroup",
		"after":  lib.RootGroupPath,
	}, checkGroups(t, tc))
}

// Regression test for #2848.
func TestAsyncGroupExecutionOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		script   string
		expected []string
	}{
		{
			name: "synchronous work outside group",
			script: `
				(async () => {
					await k6.group("somename", async () => {
						mark("A");
						await Promise.resolve();
						mark("B");
					});
					mark("after-group");
				})();
				mark("C");
			`,
			expected: []string{"A", "C", "B", "after-group"},
		},
		{
			name: "queued promise reaction",
			script: `
				(async () => {
					await k6.group("somename", async () => {
						mark("A");
						await Promise.resolve();
						mark("B");
					});
					mark("after-group");
				})();
				Promise.resolve("D").then(() => { mark("D"); });
				mark("C");
			`,
			expected: []string{"A", "C", "B", "D", "after-group"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			tc := testCaseRuntime(t)
			order := newOrderRecorder(t, tc)
			_, err := tc.testRuntime.RunOnEventLoop(test.script)
			require.NoError(t, err)
			require.Equal(t, test.expected, *order)
		})
	}
}

// Regression test for #5435.
func TestConcurrentAsyncGroupsDurationsAndTags(t *testing.T) {
	t.Parallel()

	tc := testCaseRuntime(t)
	_, err := tc.testRuntime.RunOnEventLoop(`
		(async () => {
			const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
			const check = (name) => k6.check(null, {[name]: true});
			const slow = k6.group("coolgroup", async () => {
				check("slow:start");
				await delay(10);
				check("slow:afterFirst");
				await delay(40).then(() => {});
				check("slow:afterSecond");
			});
			const fast = k6.group("fastgroup", async () => {
				check("fast:start");
				await delay(10).then(() => {});
				check("fast:afterFirst");
			});
			await Promise.all([slow, fast]);
			check("after");
		})()
	`)
	require.NoError(t, err)

	builtinMetrics := tc.testRuntime.VU.State().BuiltinMetrics
	durByGroup := map[string]float64{}
	durationCounts := map[string]int{}
	checkGroups := map[string]string{}
	var checkCount int
	for _, sampleContainer := range metrics.GetBufferedSamples(tc.samples) {
		for _, sample := range sampleContainer.GetSamples() {
			group, _ := sample.Tags.Get(metrics.TagGroup.String())
			switch sample.Metric {
			case builtinMetrics.GroupDuration:
				durByGroup[group] = sample.Value
				durationCounts[group]++
			case builtinMetrics.Checks:
				checkCount++
				name, ok := sample.Tags.Get(metrics.TagCheck.String())
				require.True(t, ok)
				assert.Equal(t, float64(1), sample.Value, "check %q failed", name)
				checkGroups[name] = group
			}
		}
	}
	assert.Equal(t, 6, checkCount)
	assert.Equal(t, map[string]int{"::coolgroup": 1, "::fastgroup": 1}, durationCounts)

	assert.Equal(t, map[string]string{
		"slow:start":       "::coolgroup",
		"slow:afterFirst":  "::coolgroup",
		"slow:afterSecond": "::coolgroup",
		"fast:start":       "::fastgroup",
		"fast:afterFirst":  "::fastgroup",
		"after":            lib.RootGroupPath,
	}, checkGroups)

	slow, okSlow := durByGroup["::coolgroup"]
	fast, okFast := durByGroup["::fastgroup"]

	require.True(t, okSlow, "missing group_duration for ::coolgroup, got %v", durByGroup)
	require.True(t, okFast, "missing group_duration for ::fastgroup, got %v", durByGroup)

	t.Logf("group_duration ::coolgroup (slow) = %.2fms, ::fastgroup (fast) = %.2fms", slow, fast)

	assert.Greater(t, slow, fast,
		"slow group (::coolgroup=%.2fms) must last clearly longer than fast group (::fastgroup=%.2fms)",
		slow, fast)
}

func TestConcurrentAsyncGroupsKeepIndependentTags(t *testing.T) {
	t.Parallel()

	tc := testCaseRuntime(t)

	_, err := tc.testRuntime.RunOnEventLoop(`
		(async () => {
			const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
			const check = (name) => k6.check(null, {[name]: true});
			const names = ["alpha", "beta", "gamma", "delta"];
			const grouped = names.map((name, index) => k6.group(name, async () => {
				check(name + ":start");
				await delay((index + 1) * 5);
				check(name + ":afterTimer");
				await Promise.resolve();
				check(name + ":afterPromise");
			}));
			await Promise.all(grouped);
			check("after");
		})()
	`)
	require.NoError(t, err)

	durationCounts := make(map[string]int)
	checkGroups := make(map[string]string)
	var checkCount int
	builtinMetrics := tc.testRuntime.VU.State().BuiltinMetrics
	for _, sampleContainer := range metrics.GetBufferedSamples(tc.samples) {
		for _, sample := range sampleContainer.GetSamples() {
			group, _ := sample.Tags.Get(metrics.TagGroup.String())
			switch sample.Metric {
			case builtinMetrics.GroupDuration:
				durationCounts[group]++
			case builtinMetrics.Checks:
				checkCount++
				name, ok := sample.Tags.Get(metrics.TagCheck.String())
				require.True(t, ok)
				assert.Equal(t, float64(1), sample.Value, "check %q failed", name)
				checkGroups[name] = group
			}
		}
	}
	require.Equal(t, 13, checkCount)
	require.Len(t, checkGroups, 13)

	for check, group := range checkGroups {
		if check == "after" {
			assert.Equal(t, lib.RootGroupPath, group)
			continue
		}

		name, _, ok := strings.Cut(check, ":")
		require.True(t, ok, "invalid check name %q", check)
		expectedGroup, pathErr := lib.NewGroupPath(lib.RootGroupPath, name)
		require.NoError(t, pathErr)
		assert.Equalf(t, expectedGroup, group, "check %q saw the wrong group", check)
	}

	assert.Equal(t, map[string]int{
		"::alpha": 1,
		"::beta":  1,
		"::gamma": 1,
		"::delta": 1,
	}, durationCounts)
}

func TestAsyncGroupScopesCompleteMetricContext(t *testing.T) {
	t.Parallel()

	tc := testCaseRuntime(t)
	state := tc.testRuntime.VU.State()
	setContext := func(phase string) {
		state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
			tagsAndMeta.SetTag("phase", phase)
			tagsAndMeta.SetMetadata("trace", phase)
		})
	}
	setContext("root")
	require.NoError(t, tc.testRuntime.VU.Runtime().Set("setContext", setContext))

	_, err := tc.testRuntime.RunOnEventLoop(`
		(async () => {
			await k6.group("g", async () => {
				setContext("inside");
				k6.check(null, {beforeAwait: true});
				await Promise.resolve();
				k6.check(null, {afterAwait: true});
			});
			k6.check(null, {outside: true});
		})()
	`)
	require.NoError(t, err)

	wantPhase := map[string]string{
		"beforeAwait": "inside",
		"afterAwait":  "inside",
		"outside":     "root",
	}
	for _, sampleContainer := range metrics.GetBufferedSamples(tc.samples) {
		for _, sample := range sampleContainer.GetSamples() {
			if sample.Metric != state.BuiltinMetrics.Checks {
				continue
			}
			name, _ := sample.Tags.Get(metrics.TagCheck.String())
			phase, _ := sample.Tags.Get("phase")
			assert.Equal(t, wantPhase[name], phase, "check %q used the wrong tag context", name)
			assert.Equal(t, wantPhase[name], sample.Metadata["trace"],
				"check %q used the wrong metadata context", name)
			delete(wantPhase, name)
		}
	}
	require.Empty(t, wantPhase)

	current := state.Tags.GetCurrentValues()
	phase, _ := current.Tags.Get("phase")
	require.Equal(t, "root", phase)
	require.Equal(t, "root", current.Metadata["trace"])
}

func TestDisabledAsyncMetricContextKeepsMutableTagsLive(t *testing.T) {
	t.Parallel()

	tc := testCaseRuntimeWithAsyncMetricContext(t, false)
	state := tc.testRuntime.VU.State()
	state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
		tagsAndMeta.SetTag("phase", "before")
		tagsAndMeta.SetMetadata("trace", "before")
	})
	require.NoError(t, tc.testRuntime.VU.Runtime().Set("setContext", func() {
		state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
			tagsAndMeta.SetTag("phase", "after")
			tagsAndMeta.SetMetadata("trace", "after")
		})
	}))

	_, err := tc.testRuntime.RunOnEventLoop(`k6.group("g", () => setContext())`)
	require.NoError(t, err)

	current := state.Tags.GetCurrentValues()
	phase, _ := current.Tags.Get("phase")
	require.Equal(t, "after", phase)
	require.Equal(t, "after", current.Metadata["trace"])

	samples := metrics.GetBufferedSamples(tc.samples)
	require.Len(t, samples, 1)
	sample, ok := samples[0].(metrics.Sample)
	require.True(t, ok)
	phase, _ = sample.Tags.Get("phase")
	require.Equal(t, "after", phase)
	require.Equal(t, "after", sample.Metadata["trace"])
}

// Regression test for #5673.
func TestAsyncContextGroupPropagation(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	_, err := tc.testRuntime.RunOnEventLoop(`
		const delay = (t) => new Promise((r) => setTimeout(r, t));
		(async () => {
			await k6.group("coolgroup", async () => {
				k6.check(null, {sync: true});
				await delay(5);
				k6.check(null, {afterAwait: true});
				await delay(5).then(() => { k6.check(null, {continuation: true}); });
			});
			k6.check(null, {after: true});
		})()
	`)
	require.NoError(t, err)

	assert.Equal(t, map[string]string{
		"sync":         "::coolgroup",
		"afterAwait":   "::coolgroup",
		"continuation": "::coolgroup",
		"after":        lib.RootGroupPath,
	}, checkGroups(t, tc))
}

func TestGroupTrackerRestoresStageGroup(t *testing.T) {
	t.Parallel()

	for _, stage := range []string{"setup", "teardown"} {
		t.Run(stage, func(t *testing.T) {
			t.Parallel()
			tc := testCaseRuntime(t)
			stageGroup, err := lib.NewGroupPath(lib.RootGroupPath, stage)
			require.NoError(t, err)
			tc.testRuntime.VU.State().Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
				tagsAndMeta.SetSystemTagOrMeta(metrics.TagGroup, stageGroup)
			})

			_, err = tc.testRuntime.RunOnEventLoop(`
					Promise.resolve().then(() => k6.check(null, {reaction: true}));
					k6.group("warmup", () => {});
				`)
			require.NoError(t, err)

			assert.Equal(t, map[string]string{"reaction": stageGroup}, checkGroups(t, tc))
			group, _ := tc.testRuntime.VU.State().Tags.GetCurrentValues().Tags.Get(
				metrics.TagGroup.String())
			require.Equal(t, stageGroup, group)
		})
	}
}

func TestGroupSupportsPromiseLikeThenable(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)
	order := newOrderRecorder(t, tc)

	_, err := tc.testRuntime.RunOnEventLoop(`
		(async () => {
			const grouped = k6.group("custom", () => ({
				then(onFulfilled) {
					setTimeout(() => {
						mark("settled");
						onFulfilled(42);
					}, 1);
				}
			}));
			mark(grouped instanceof Promise ? "promise" : "not-promise");
			const value = await grouped;
			mark("after:" + value);
		})();
	`)
	require.NoError(t, err)
	require.Equal(t, []string{"promise", "settled", "after:42"}, *order)
}

func TestGroupThenablePreservesSettlementIdentity(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	_, err := tc.testRuntime.RunOnEventLoop(`
		(async () => {
			const fulfillment = {};
			const fulfilled = await k6.group("fulfilled", () => ({
				then(onFulfilled) { return Promise.resolve(fulfillment).then(onFulfilled); }
			}));
			if (fulfilled !== fulfillment) {
				throw new Error("fulfillment identity changed");
			}

			const rejection = {};
			let caught;
			try {
				await k6.group("rejected", () => ({
					then(_, onRejected) { return Promise.reject(rejection).then(undefined, onRejected); }
				}));
			} catch (reason) {
				caught = reason;
			}
			if (caught !== rejection) {
				throw new Error("rejection identity changed");
			}
		})();
	`)
	require.NoError(t, err)
}

// Primitive results must not be boxed for thenable detection.
func TestGroupSynchronousResultStaysSynchronous(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	result, err := tc.testRuntime.RunOnEventLoop(`
		let thenCalled = false;
		Number.prototype.then = function() { thenCalled = true; };
		const result = k6.group("sync", () => 42);
		if (thenCalled) { throw new Error("synchronous result was treated as a thenable"); }
		result;
	`)
	require.NoError(t, err)
	require.Equal(t, int64(42), result.Export())
	group, _ := tc.testRuntime.VU.State().Tags.GetCurrentValues().Tags.Get(metrics.TagGroup.String())
	require.Equal(t, lib.RootGroupPath, group)

	samples := metrics.GetBufferedSamples(tc.samples)
	require.Len(t, samples, 1)
	sample, ok := samples[0].(metrics.Sample)
	require.True(t, ok)
	require.Equal(t, tc.testRuntime.VU.State().BuiltinMetrics.GroupDuration, sample.Metric)
	metricGroup, ok := sample.Tags.Get(metrics.TagGroup.String())
	require.True(t, ok)
	require.Equal(t, "::sync", metricGroup)
}

// group_duration uses the tag and metadata snapshot captured before the callback runs.
func TestGroupDurationUsesEntryTagsAndMetadata(t *testing.T) {
	t.Parallel()

	for name, script := range map[string]string{
		"sync": `k6.group("g", () => setPhase("after"));`,
		"async": `(async () => {
			await k6.group("g", async () => {
				setPhase("after");
				await Promise.resolve();
			});
		})();`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			tc := testCaseRuntime(t)
			state := tc.testRuntime.VU.State()
			state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
				tagsAndMeta.SetTag("phase", "before")
				tagsAndMeta.SetMetadata("trace", "before")
			})
			require.NoError(t, tc.testRuntime.VU.Runtime().Set("setPhase", func(phase string) {
				state.Tags.Modify(func(tagsAndMeta *metrics.TagsAndMeta) {
					tagsAndMeta.SetTag("phase", phase)
					tagsAndMeta.SetMetadata("trace", phase)
				})
			}))

			_, err := tc.testRuntime.RunOnEventLoop(script)
			require.NoError(t, err)

			samples := metrics.GetBufferedSamples(tc.samples)
			require.Len(t, samples, 1)
			sample, ok := samples[0].(metrics.Sample)
			require.True(t, ok)
			phase, ok := sample.Tags.Get("phase")
			require.True(t, ok)
			require.Equal(t, "before", phase)
			require.Equal(t, "before", sample.Metadata["trace"])
		})
	}
}

// A throwing then getter must restore the parent and still record duration.
func TestGroupThrowingThenGetterRestoresParent(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	_, err := tc.testRuntime.RunOnEventLoop(`
		try {
			k6.group("bad", () => ({
					get then() { throw new Error("boom"); }
				}));
			} catch (_) {}
			k6.check(null, {afterFailure: true});
			k6.group("next", () => k6.check(null, {next: true}));
			k6.check(null, {afterNext: true});
		`)
	require.NoError(t, err)

	builtinMetrics := tc.testRuntime.VU.State().BuiltinMetrics
	var badGroupDurations int
	checks := make(map[string]string)
	for _, sc := range metrics.GetBufferedSamples(tc.samples) {
		for _, sample := range sc.GetSamples() {
			group, _ := sample.Tags.Get(metrics.TagGroup.String())
			switch sample.Metric {
			case builtinMetrics.GroupDuration:
				if group == "::bad" {
					badGroupDurations++
				}
			case builtinMetrics.Checks:
				name, ok := sample.Tags.Get(metrics.TagCheck.String())
				require.True(t, ok)
				assert.Equal(t, float64(1), sample.Value, "check %q failed", name)
				checks[name] = group
			}
		}
	}
	assert.Equal(t, map[string]string{
		"afterFailure": lib.RootGroupPath,
		"next":         "::next",
		"afterNext":    lib.RootGroupPath,
	}, checks)
	require.Equal(t, 1, badGroupDurations)
}

// The Go implementation must not depend on the old private JavaScript globals.
func TestGroupNotTamperableViaGlobals(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	_, err := tc.testRuntime.RunOnEventLoop(`
			globalThis.__k6group = function () { throw new Error("hijacked group()"); };
			globalThis.__k6AsyncGroup = { getGroupTag: function () { return "tampered"; } };
			k6.group("real", () => { k6.check(null, {inside: true}); });
		`)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"inside": "::real"}, checkGroups(t, tc))
}

// A synchronously throwing then() rejects the returned promise and still records duration.
func TestGroupThenableThrowsStillEmitsDuration(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	_, err := tc.testRuntime.RunOnEventLoop(`
		(async () => {
			const reason = {};
			let caught;
			try {
				await k6.group("g", () => ({ then() { throw reason; } }));
			} catch (error) {
				caught = error;
			}
			if (caught !== reason) {
				throw new Error("rejection identity changed");
			}
		})();
	`)
	require.NoError(t, err)

	gd := tc.testRuntime.VU.State().BuiltinMetrics.GroupDuration
	expectedGroup, err2 := lib.NewGroupPath(lib.RootGroupPath, "g")
	require.NoError(t, err2)

	var found bool
	for _, sc := range metrics.GetBufferedSamples(tc.samples) {
		for _, s := range sc.GetSamples() {
			if s.Metric == gd {
				g, _ := s.Tags.Get("group")
				if g == expectedGroup {
					found = true
				}
			}
		}
	}
	require.True(t, found, "group_duration for %q must be emitted even when the thenable throws synchronously", expectedGroup)
}

func TestGroupSynchronousThenableRejectionEmitsDurationOnce(t *testing.T) {
	t.Parallel()
	tc := testCaseRuntime(t)

	_, err := tc.testRuntime.RunOnEventLoop(`
	(async () => {
		const rejection = new Error('rejected');
		let caught;
		try {
			await k6.group('g', () => ({
				then(_, reject) { reject(rejection); },
			}));
		} catch (reason) {
			caught = reason;
		}
		if (caught !== rejection) {
			throw new Error('rejection identity changed');
		}
		k6.check(null, {after: true});
	})();
	`)
	require.NoError(t, err)

	var durations int
	for _, sc := range metrics.GetBufferedSamples(tc.samples) {
		for _, sample := range sc.GetSamples() {
			group, _ := sample.Tags.Get(metrics.TagGroup.String())
			switch sample.Metric {
			case tc.testRuntime.VU.State().BuiltinMetrics.GroupDuration:
				if group == "::g" {
					durations++
				}
			case tc.testRuntime.VU.State().BuiltinMetrics.Checks:
				assert.Equal(t, lib.RootGroupPath, group)
			}
		}
	}
	require.Equal(t, 1, durations)
}

// Compare benchmark deltas: RunOnEventLoop adds the same compile and drain
// overhead to every case.

func benchRun(b *testing.B, tc *testCase, src string) {
	b.Helper()
	if _, err := tc.testRuntime.RunOnEventLoop(src); err != nil {
		b.Fatal(err)
	}
}

// Prevent group_duration emission from filling the sample buffer.
func benchDrainSamples(tc *testCase) func() {
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			case <-tc.samples:
			}
		}
	}()
	return func() { close(done) }
}

func BenchmarkNoGroupSync(b *testing.B) {
	tc := testCaseRuntime(b)
	defer benchDrainSamples(tc)()
	benchRun(b, tc, `globalThis.work = () => 42;`)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}

func BenchmarkNoGroupSyncFeatureDisabled(b *testing.B) {
	tc := testCaseRuntimeWithAsyncMetricContext(b, false)
	defer benchDrainSamples(tc)()
	benchRun(b, tc, `globalThis.work = () => 42;`)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}

func BenchmarkGroupSync(b *testing.B) {
	tc := testCaseRuntime(b)
	defer benchDrainSamples(tc)()
	benchRun(b, tc, `globalThis.work = () => { k6.group("x", () => {}); };`)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}

func BenchmarkGroupAsync(b *testing.B) {
	tc := testCaseRuntime(b)
	defer benchDrainSamples(tc)()
	benchRun(b, tc, `globalThis.work = () => k6.group("x", async () => { await Promise.resolve(); });`)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}

func BenchmarkPromiseOnce(b *testing.B) {
	tc := testCaseRuntime(b)
	defer benchDrainSamples(tc)()
	benchRun(b, tc, `globalThis.work = () => Promise.resolve(1);`)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}

func BenchmarkAwaitPromiseOnce(b *testing.B) {
	tc := testCaseRuntime(b)
	defer benchDrainSamples(tc)()
	benchRun(b, tc, `globalThis.work = async () => { await Promise.resolve(); };`)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}

func BenchmarkAwaitPromiseOnceFeatureDisabled(b *testing.B) {
	tc := testCaseRuntimeWithAsyncMetricContext(b, false)
	defer benchDrainSamples(tc)()
	benchRun(b, tc, `globalThis.work = async () => { await Promise.resolve(); };`)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}

func BenchmarkAwaitGroupPromise(b *testing.B) {
	tc := testCaseRuntime(b)
	defer benchDrainSamples(tc)()
	benchRun(b, tc, `globalThis.work = async () => { await k6.group("x", () => Promise.resolve()); };`)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}

func BenchmarkGroupPromise(b *testing.B) {
	tc := testCaseRuntime(b)
	defer benchDrainSamples(tc)()
	benchRun(b, tc, `globalThis.work = () => k6.group("x", () => Promise.resolve(1));`)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}

const benchPromiseWorkload = `globalThis.work = async () => {
  let value = 0;
  for (let i = 0; i < 100; i++) {
    value = await Promise.resolve(value + 1);
  }
  return value;
};`

func BenchmarkPromiseWorkload(b *testing.B) {
	tc := testCaseRuntime(b)
	defer benchDrainSamples(tc)()
	benchRun(b, tc, benchPromiseWorkload)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}

const benchSparseAsyncGroup = `globalThis.work = async () => {
  let value = 0;
  for (let i = 0; i < 100; i++) {
    if (i === 50) {
      value = await k6.group("x", async () => Promise.resolve(value + 1));
    } else {
      value = await Promise.resolve(value + 1);
    }
  }
  return value;
};`

func BenchmarkSparseAsyncGroup(b *testing.B) {
	tc := testCaseRuntime(b)
	defer benchDrainSamples(tc)()
	benchRun(b, tc, benchSparseAsyncGroup)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}

const benchSparsePromiseGroup = `globalThis.work = async () => {
  let value = 0;
  for (let i = 0; i < 100; i++) {
    if (i === 50) {
      value = await k6.group("x", () => Promise.resolve(value + 1));
    } else {
      value = await Promise.resolve(value + 1);
    }
  }
  return value;
};`

func BenchmarkSparsePromiseGroup(b *testing.B) {
	tc := testCaseRuntime(b)
	defer benchDrainSamples(tc)()
	benchRun(b, tc, benchSparsePromiseGroup)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}

const benchPromiseChain = `globalThis.work = () => {
  let p = Promise.resolve(0);
  for (let i = 0; i < 20; i++) {
    p = p.then((v) => v + 1);
  }
  return p;
};`

func BenchmarkPromiseChainFeatureDisabled(b *testing.B) {
	tc := testCaseRuntimeWithAsyncMetricContext(b, false)
	benchRun(b, tc, benchPromiseChain)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}

// Compare with BenchmarkPromiseChainFeatureDisabled to measure tracker overhead.
func BenchmarkPromiseChainWithTracker(b *testing.B) {
	tc := testCaseRuntime(b)
	benchRun(b, tc, benchPromiseChain)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		benchRun(b, tc, `work()`)
	}
}
