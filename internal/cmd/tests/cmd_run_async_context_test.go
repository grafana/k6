package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/lib/fsext"
)

func TestRunAsyncMetricContextWithoutK6Module(t *testing.T) {
	t.Parallel()

	script := `
		import exec from 'k6/execution';
		import { Counter } from 'k6/metrics';

		const callbackPhases = new Counter('callback_phases');
		const delay = ms => new Promise(resolve => setTimeout(resolve, ms));

		export default function () {
			let release;
			const gate = new Promise(resolve => { release = resolve; });
			exec.vu.metrics.tags.owner = 'registered';
			exec.vu.metrics.metadata.trace = 'registered';
			const pending = gate.then(async () => {
				for (const phase of ['first', 'second', 'third']) {
					exec.vu.metrics.tags.phase = phase;
					exec.vu.metrics.metadata.step = phase;
					if (phase === 'second') {
						await delay(0);
					} else {
						await Promise.resolve();
					}
					callbackPhases.add(1, {phase_name: phase});
				}
			});
			exec.vu.metrics.tags.owner = 'root';
			exec.vu.metrics.metadata.trace = 'root';
			release();
			return pending;
		}
	`

	ts := getSingleFileTestState(t, script,
		[]string{"--features", "async-metric-context", "--out", "json=results.json", "--summary-mode=disabled"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	results, err := fsext.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)
	for _, phase := range []string{"first", "second", "third"} {
		assert.Equal(t, []float64{1}, getSampleValuesWithMetadata(t, results, "callback_phases", map[string]string{
			"owner": "registered", "phase": phase, "phase_name": phase,
		}, map[string]string{"trace": "registered", "step": phase}))
	}
}
