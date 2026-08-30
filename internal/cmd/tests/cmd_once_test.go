package tests

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/guregu/null.v3"

	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/cmd"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/lib/executor"
)

func TestRunOnce(t *testing.T) {
	t.Parallel()

	script := `
		import { Counter } from 'k6/metrics';

		export const options = {
			tags: { top: 'level' },
			thresholds: { 'hits{top:level}': ['count==1'] },
		};

		const hits = new Counter('hits');

		export default function () {
			hits.add(1);
			console.log('once ran');
		}
	`

	ts := getSingleFileTestState(t, script, []string{"--log-output=stdout", "--once"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	assert.Contains(t, stdout, "1 iterations shared among 1 VUs")
	assert.Equal(t, 1, strings.Count(stdout, "once ran"))
	assert.Contains(t, stdout, "hits{top:level}")
	assert.Contains(t, stdout, "'count==1' count=1")
}

func TestRunOnceRejectsMultipleScenarios(t *testing.T) {
	t.Parallel()

	script := `
		export const options = { scenarios: {
			s1: { executor: 'shared-iterations' },
			s2: { executor: 'shared-iterations' }
		}};
		export default function() {}
	`

	ts := getSingleFileTestState(t, script,
		[]string{"--log-output=stdout", "--once"}, exitcodes.InvalidConfig)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Contains(t, ts.Stdout.String(),
		"--once can run only with a single scenario")
}

const onceCloudScript = `
	export const options = {
		cloud: { name: 'once', projectID: 123456 },
		scenarios: {
			ui: { executor: 'shared-iterations', vus: 3, iterations: 9 }
		}
	};
	export default function() { console.log('once ran'); }
`

func mustExtractSingleSharedIterScenario(t *testing.T, opts lib.Options) executor.SharedIterationsConfig {
	t.Helper()

	require.Len(t, opts.Scenarios, 1)
	for _, sc := range opts.Scenarios {
		shared, ok := sc.(executor.SharedIterationsConfig)
		require.True(t, ok)
		return shared
	}
	return executor.SharedIterationsConfig{}
}

func TestCloudRunOnceLocalExecution(t *testing.T) {
	t.Parallel()

	ts := makeTestState(t, onceCloudScript,
		[]string{"--log-output=stdout", "--once", "--local-execution"})
	setupLocalExecutionProvMock(t, ts)

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()
	assert.Contains(t, stdout, "1 iterations shared among 1 VUs")
	assert.Equal(t, 1, strings.Count(stdout, "once ran"))
}

func TestCloudRunOnceUploadsRewrittenArchive(t *testing.T) {
	t.Parallel()

	_, tarData := uploadAndCaptureArchive(t,
		[]string{"k6", "cloud", "run", "--once", "test.js"}, nil, onceCloudScript)

	arc, err := lib.ReadArchive(bytes.NewReader(tarData))
	require.NoError(t, err)

	sc := mustExtractSingleSharedIterScenario(t, arc.Options)
	assert.Equal(t, null.IntFrom(1), sc.VUs)
	assert.Equal(t, null.IntFrom(1), sc.Iterations)
}
