package tests

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/cmd"
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
