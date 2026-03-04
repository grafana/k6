package jsexec

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/pprof/profile"
	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
)

func TestConfigFromRuntimeOptions(t *testing.T) {
	opts := lib.RuntimeOptions{
		JSProfilingEnabled:          null.BoolFrom(true),
		JSCPUProfileOutput:          null.StringFrom("cpu.pprof"),
		JSRuntimeTraceOutput:        null.StringFrom("run.trace"),
		JSProfileID:                 null.StringFrom("test-profile"),
		JSFirstRunnerMemMaxBytes:    null.IntFrom(1024),
		JSFirstRunnerMemStepPercent: null.IntFrom(5),
	}

	cfg := ConfigFromRuntimeOptions(opts)
	require.True(t, cfg.Enabled)
	require.Equal(t, ScopeCombined, cfg.Scope)
	require.Equal(t, "cpu.pprof", cfg.CPUProfilePath)
	require.Equal(t, "run.trace", cfg.RuntimeTracePath)
	require.Equal(t, "test-profile", cfg.ProfileID)
	require.EqualValues(t, 1024, cfg.FirstRunnerMemMaxBytes)
	require.EqualValues(t, 5, cfg.FirstRunnerMemStepPercent)
}

func TestManagerStartStopStoresArtifacts(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Config{
		Enabled:          true,
		CPUProfilePath:   filepath.Join(dir, "cpu.pprof"),
		RuntimeTracePath: filepath.Join(dir, "run.trace"),
		ProfileID:        "p1",
	})
	require.NoError(t, m.Start())
	Activate(m)
	defer Deactivate(m)
	rt := sobek.New()
	m.maybeStartRuntimeProfile(rt, ScopeVU)
	var err error
	DoWithLabels(context.Background(), map[string]string{
		"js.phase": "test",
		"js.vu":    "1",
	}, func(ctx context.Context) {
		WithRegion(ctx, "k6.js.test", func() {
			_, err = rt.RunString(`
let s = 0;
for (let i=0;i<100000;i++) { s += i; }
s;
`)
		})
	})
	require.NoError(t, err)
	time.Sleep(20 * time.Millisecond)
	m.Stop()

	cpu, ok := LatestArtifact("js-cpu")
	require.True(t, ok)
	require.Equal(t, "p1", cpu.ProfileID)
	require.NotEmpty(t, cpu.Data)

	trace, ok := LatestArtifact("js-trace")
	require.True(t, ok)
	require.Equal(t, "p1", trace.ProfileID)
	require.NotEmpty(t, trace.Data)
}

func TestManagerCapturesAsyncAllocationAndWaitLabels(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Config{
		Enabled:          true,
		CPUProfilePath:   filepath.Join(dir, "cpu_async.pprof"),
		RuntimeTracePath: filepath.Join(dir, "run_async.trace"),
		ProfileID:        "p-async",
	})
	require.NoError(t, m.Start())
	Activate(m)
	defer Deactivate(m)

	rt := sobek.New()
	InstallRuntimeAsyncContextTracker(rt, map[string]string{
		"js.phase": "test.async",
		"js.vu":    "1",
	})
	m.maybeStartRuntimeProfile(rt, ScopeVU)

	_, err := rt.RunString(`
async function allocateAsync() {
  let out = [];
  for (let i = 0; i < 200; i++) {
    await Promise.resolve(i);
    out.push({ i, payload: "v-" + i, nested: { k: i % 3 } });
  }
  return out.length;
}
allocateAsync();
`)
	require.NoError(t, err)
	time.Sleep(30 * time.Millisecond)
	m.Stop()

	cpu, ok := LatestArtifact("js-cpu")
	require.True(t, ok)
	require.NotEmpty(t, cpu.Data)

	pr, err := profile.Parse(bytes.NewReader(cpu.Data))
	require.NoError(t, err)

	hasAllocObjects := false
	hasAllocSpace := false
	allocObjectsIdx := -1
	allocSpaceIdx := -1
	for i, st := range pr.SampleType {
		if st.Type == "alloc_objects" {
			hasAllocObjects = true
			allocObjectsIdx = i
		}
		if st.Type == "alloc_space" {
			hasAllocSpace = true
			allocSpaceIdx = i
		}
	}
	require.True(t, hasAllocObjects, "alloc_objects sample type missing")
	require.True(t, hasAllocSpace, "alloc_space sample type missing")
	require.NotEqual(t, -1, allocObjectsIdx)
	require.NotEqual(t, -1, allocSpaceIdx)

	var totalAllocObjects, totalAllocSpace int64
	for _, s := range pr.Sample {
		if allocObjectsIdx < len(s.Value) {
			totalAllocObjects += s.Value[allocObjectsIdx]
		}
		if allocSpaceIdx < len(s.Value) {
			totalAllocSpace += s.Value[allocSpaceIdx]
		}
	}
	require.Greater(t, totalAllocObjects, int64(0), "expected attributed alloc_objects to be > 0")
	require.Greater(t, totalAllocSpace, int64(0), "expected attributed alloc_space to be > 0")

	trace, ok := LatestArtifact("js-trace")
	require.True(t, ok)
	require.NotEmpty(t, trace.Data)
	require.Contains(t, string(trace.Data), "sobek.async.promise_reaction")
}

func TestManagerScopesInitVUCombined(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Config{
		Enabled:          true,
		Scope:            ScopeCombined,
		CPUProfilePath:   filepath.Join(dir, "cpu.pprof"),
		RuntimeTracePath: filepath.Join(dir, "run.trace"),
		ProfileID:        "p-scope",
	})
	require.NoError(t, m.Start())
	rtInit := sobek.New()
	rtVU := sobek.New()
	m.maybeStartRuntimeProfile(rtInit, ScopeInit)
	m.maybeStartRuntimeProfile(rtVU, ScopeVU)
	_, err := rtInit.RunString(`let a=0; for (let i=0;i<50000;i++) { a += i; } a;`)
	require.NoError(t, err)
	_, err = rtVU.RunString(`let b=0; for (let i=0;i<50000;i++) { b += i*2; } b;`)
	require.NoError(t, err)
	time.Sleep(25 * time.Millisecond)
	m.Stop()

	initCPU, ok := LatestArtifact("js-cpu-init")
	require.True(t, ok)
	require.NotEmpty(t, initCPU.Data)
	vuCPU, ok := LatestArtifact("js-cpu-vu")
	require.True(t, ok)
	require.NotEmpty(t, vuCPU.Data)
	combinedCPU, ok := LatestArtifact("js-cpu-combined")
	require.True(t, ok)
	require.NotEmpty(t, combinedCPU.Data)
	defaultCPU, ok := LatestArtifact("js-cpu")
	require.True(t, ok)
	require.NotEmpty(t, defaultCPU.Data)
}
