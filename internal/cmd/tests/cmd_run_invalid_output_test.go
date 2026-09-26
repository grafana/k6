package tests

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/internal/cmd"
)

func TestRunInvalidOutputExitCode(t *testing.T) {
	t.Parallel()

	ts := getSingleFileTestState(
		t,
		`export default function() {}`,
		[]string{"--out", "foo"},
		exitcodes.InvalidConfig,
	)
	cmd.ExecuteWithGlobalState(ts.GlobalState)

	assert.Contains(t, ts.Stderr.String(), "invalid output type 'foo'")
}
