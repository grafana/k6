package tests

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/v2/internal/cmd"
)

const featuresRunScript = `
	export const options = {
		scenarios: {
			s: { executor: 'per-vu-iterations', vus: 1, iterations: 1 },
		},
	};
	export default function () {}
`

func TestRunUnknownFeatureNameExitsZero(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(t, featuresRunScript,
		[]string{"--out", "json=results.json", "--no-usage-report", "--features", "not-a-flag"}, 0)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	var sawUnknownError bool
	for _, e := range ts.LoggerHook.Drain() {
		if e.Level == logrus.ErrorLevel &&
			e.Data["feature"] == "not-a-flag" &&
			e.Data["outcome"] == "unknown" &&
			e.Data["source"] == "cli" {
			sawUnknownError = true
		}
	}
	assert.True(t, sawUnknownError, "expected unknown-name ERROR with source=cli")
	assert.Nil(t, ts.Usage.Map()["features"], "unknown name must not contribute to telemetry")
}
