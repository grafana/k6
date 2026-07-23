package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/lib/fsext"
)

const mergeRunTagsScript = `
	export const options = {
		tags: { env: 'staging', team: 'backend' },
		scenarios: {
			s: { executor: 'per-vu-iterations', vus: 1, iterations: 1 },
		},
	};
	export default function () {}
`

// Without the flag, CLI --tag replaces script-level options.tags wholesale.
// Samples should NOT carry the script-level `env` tag.
func TestRunMergeRunTagsFlagOff(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, mergeRunTagsScript,
		[]string{"--out", "json=results.json", "--no-usage-report", "--tag", "region=us-east"}, 0)

	results := runAndReadResults(t, ts)

	withScriptTag := getSampleValues(t, results, "iterations", map[string]string{"env": "staging"})
	assert.Empty(t, withScriptTag, "script-level tags must be replaced wholesale when the flag is off")

	withCLITag := getSampleValues(t, results, "iterations", map[string]string{"region": "us-east"})
	assert.NotEmpty(t, withCLITag, "CLI --tag must still apply")
}

// With the flag on, CLI --tag, script options.tags, and env K6_TAGS all contribute.
func TestRunMergeRunTagsFlagOn(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, mergeRunTagsScript,
		[]string{
			"--out", "json=results.json", "--no-usage-report",
			"--features", "merge-run-tags",
			"--tag", "region=us-east",
		}, 0)

	results := runAndReadResults(t, ts)

	for k, v := range map[string]string{
		"env":    "staging", // from script options.tags
		"team":   "backend", // from script options.tags
		"region": "us-east", // from CLI --tag
	} {
		samples := getSampleValues(t, results, "iterations", map[string]string{k: v})
		assert.NotEmpty(t, samples, "expected merged tag %s=%s", k, v)
	}
}

// Higher-priority layers must win on key collision when the flag is on.
func TestRunMergeRunTagsCLIWinsOnCollision(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, mergeRunTagsScript,
		[]string{
			"--out", "json=results.json", "--no-usage-report",
			"--features", "merge-run-tags",
			"--tag", "env=prod",
		}, 0)

	results := runAndReadResults(t, ts)

	wins := getSampleValues(t, results, "iterations", map[string]string{"env": "prod"})
	assert.NotEmpty(t, wins, "CLI --tag must win over script-level options.tags on collision")

	loses := getSampleValues(t, results, "iterations", map[string]string{"env": "staging"})
	assert.Empty(t, loses, "lower-priority value must not appear when the higher-priority layer overrides it")
}

func runAndReadResults(t *testing.T, ts *GlobalTestState) []byte {
	t.Helper()
	cmd.ExecuteWithGlobalState(ts.GlobalState)
	data, err := fsext.ReadFile(ts.FS, "results.json")
	require.NoError(t, err)
	return data
}
