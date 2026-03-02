package jsexec

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/lib"
)

func TestConfigFromRuntimeOptions(t *testing.T) {
	opts := lib.RuntimeOptions{
		JSProfilingEnabled:   null.BoolFrom(true),
		JSCPUProfileOutput:   null.StringFrom("cpu.pprof"),
		JSRuntimeTraceOutput: null.StringFrom("run.trace"),
		JSProfileID:          null.StringFrom("test-profile"),
	}

	cfg := ConfigFromRuntimeOptions(opts)
	require.True(t, cfg.Enabled)
	require.Equal(t, "cpu.pprof", cfg.CPUProfilePath)
	require.Equal(t, "run.trace", cfg.RuntimeTracePath)
	require.Equal(t, "test-profile", cfg.ProfileID)
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
	m.maybeStartRuntimeProfile(rt)
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
