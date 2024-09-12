package tests

import (
	"testing"

	"go.k6.io/k6/errext/exitcodes"

	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/cmd"
)

func TestK6CloudRun(t *testing.T) {
	t.Parallel()
	runCloudTests(t, setupK6CloudRunCmd)
}

func setupK6CloudRunCmd(cliFlags []string) []string {
	return append([]string{"k6", "cloud", "run"}, append(cliFlags, "test.js")...)
}

func TestCloudRunCommandIncompatibleFlags(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		cliArgs            []string
		wantStderrContains string
	}{
		{
			name:               "using --linger should be incompatible with k6 cloud run",
			cliArgs:            []string{"--linger"},
			wantStderrContains: "the --linger flag can only be used in conjunction with the --local-execution flag",
		},
		{
			name:               "using --exit-on-running should be incompatible with k6 cloud run --local-execution",
			cliArgs:            []string{"--local-execution", "--exit-on-running"},
			wantStderrContains: "the --local-execution flag is not compatible with the --exit-on-running flag",
		},
		{
			name:               "using --show-logs should be incompatible with k6 cloud run --local-execution",
			cliArgs:            []string{"--local-execution", "--show-logs"},
			wantStderrContains: "the --local-execution flag is not compatible with the --show-logs flag",
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ts := getSimpleCloudTestState(t, nil, setupK6CloudRunCmd, tc.cliArgs, nil, nil)
			ts.ExpectedExitCode = int(exitcodes.InvalidConfig)
			cmd.ExecuteWithGlobalState(ts.GlobalState)

			stderr := ts.Stderr.String()
			assert.Contains(t, stderr, tc.wantStderrContains)
		})
	}
}
