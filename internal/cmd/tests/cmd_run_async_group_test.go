package tests

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/lib/fsext"
)

func runAsyncGroupIntegrationScript(t *testing.T, script string, archived bool) []byte {
	t.Helper()

	var ts *GlobalTestState
	if archived {
		archiveState := NewGlobalTestState(t)
		require.NoError(t, fsext.WriteFile(
			archiveState.FS, filepath.Join(archiveState.Cwd, "test.js"), []byte(script), 0o644))
		archiveState.CmdArgs = []string{"k6", "archive", "test.js"}
		cmd.ExecuteWithGlobalState(archiveState.GlobalState)

		archive, err := fsext.ReadFile(archiveState.FS, "archive.tar")
		require.NoError(t, err)
		ts = NewGlobalTestState(t)
		require.NoError(t, fsext.WriteFile(ts.FS, filepath.Join(ts.Cwd, "archive.tar"), archive, 0o644))
		ts.CmdArgs = []string{
			"k6", "run", "--features", "async-metric-context", "--out", "json=results.json",
			"--summary-mode=disabled", "archive.tar",
		}
	} else {
		ts = getSingleFileTestState(t, script,
			[]string{"--features", "async-metric-context", "--out", "json=results.json", "--summary-mode=disabled"}, 0)
	}

	cmd.ExecuteWithGlobalState(ts.GlobalState)
	results, err := fsext.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)
	return results
}

func TestRunAsyncGroupSetupAndTeardownReactionsKeepStageGroup(t *testing.T) {
	t.Parallel()

	script := `
		import { check, group } from 'k6';

		export const options = {
			setupTimeout: '1s',
			teardownTimeout: '1s',
			systemTags: ['group', 'check'],
		};

		function queueReaction(stage) {
			Promise.resolve().then(() => check(null, {[stage]: true}));
			group('warmup', () => {});
		}

		export function setup() { queueReaction('setup'); }
		export default function() {}
		export function teardown() { queueReaction('teardown'); }
	`

	for _, variant := range []struct {
		name     string
		archived bool
	}{{"source", false}, {"archive", true}} {
		t.Run(variant.name, func(t *testing.T) {
			t.Parallel()
			results := runAsyncGroupIntegrationScript(t, script, variant.archived)
			for _, stage := range []string{"setup", "teardown"} {
				assert.Equal(t, []float64{1}, getSampleValues(t, results, "checks", map[string]string{
					"check": stage, "group": "::" + stage,
				}))
			}
		})
	}
}

func TestRunOverlappingAsyncGroupsEmitIndependentTags(t *testing.T) {
	t.Parallel()

	script := `
		import { check, group } from 'k6';

		export const options = { systemTags: ['group', 'check'] };

		const delay = ms => new Promise(resolve => setTimeout(resolve, ms));

		export default async function() {
			const names = ['alpha', 'beta', 'gamma', 'delta'];
			const grouped = names.map((name, index) => group(name, async () => {
				check(null, {[name + ':start']: true});
				await delay((names.length - index) * 2);
				check(null, {[name + ':finish']: true});
			}));
			await Promise.all(grouped);
			check(null, {'outside:after': true});
		}
	`

	for _, variant := range []struct {
		name     string
		archived bool
	}{{"source", false}, {"archive", true}} {
		t.Run(variant.name, func(t *testing.T) {
			t.Parallel()
			results := runAsyncGroupIntegrationScript(t, script, variant.archived)
			for _, group := range []string{"alpha", "beta", "gamma", "delta"} {
				for _, phase := range []string{"start", "finish"} {
					assert.Equal(t, []float64{1}, getSampleValues(t, results, "checks", map[string]string{
						"check": group + ":" + phase, "group": "::" + group,
					}))
				}
				assert.Len(t, getSampleValues(t, results, "group_duration", map[string]string{
					"group": "::" + group,
				}), 1)
			}
			assert.Equal(t, []float64{1}, getSampleValues(t, results, "checks", map[string]string{
				"check": "outside:after", "group": "",
			}))
		})
	}
}

func TestRunAsyncGroupSampleTagOverrideDoesNotMutateContext(t *testing.T) {
	t.Parallel()

	script := `
		import { group } from 'k6';
		import exec from 'k6/execution';
		import { Counter } from 'k6/metrics';

		const events = new Counter('events');

		function setContext(owner, phase) {
			exec.vu.metrics.tags.owner = owner;
			exec.vu.metrics.tags.phase = phase;
			exec.vu.metrics.metadata.trace = owner;
			exec.vu.metrics.metadata.step = phase;
		}

		export default function () {
			setContext('root', 'root');

			group('checkout', () => {
				setContext('checkout', 'body');

				events.add(1, { kind: 'grouped' });
				events.add(1, { kind: 'ungrouped', group: '' });
				events.add(1, { kind: 'grouped-again' });
			});

			events.add(1, { kind: 'outside' });
		}
	`

	results := runAsyncGroupIntegrationScript(t, script, false)
	for _, sample := range []struct {
		kind  string
		group string
		owner string
		phase string
	}{
		{kind: "grouped", group: "::checkout", owner: "checkout", phase: "body"},
		{kind: "ungrouped", group: "", owner: "checkout", phase: "body"},
		{kind: "grouped-again", group: "::checkout", owner: "checkout", phase: "body"},
		{kind: "outside", group: "", owner: "root", phase: "root"},
	} {
		assert.Equal(t, []float64{1}, getSampleValuesWithMetadata(t, results, "events", map[string]string{
			"kind": sample.kind, "group": sample.group, "owner": sample.owner, "phase": sample.phase,
		}, map[string]string{"trace": sample.owner, "step": sample.phase}))
	}

	assert.Len(t, getSampleValuesWithMetadata(t, results, "group_duration", map[string]string{
		"group": "::checkout", "owner": "root", "phase": "root",
	}, map[string]string{"trace": "root", "step": "root"}), 1)
}
