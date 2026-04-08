package tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.k6.io/k6/internal/cmd"
)

const testScript = `export default function() { console.log('test message'); };`

// TestNoColorEnvironmentVariableWithNonTTY verifies that in non-TTY environments,
// logfmt format is used regardless of NO_COLOR setting.
func TestNoColorEnvironmentVariableWithNonTTY(t *testing.T) {
	t.Parallel()

	script := testScript

	ts := getSingleFileTestState(t, script, []string{"--log-output=stdout"}, 0)
	ts.Env["NO_COLOR"] = "" // Empty value per no-color.org spec

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()

	// In non-TTY (test environment), expect logfmt format
	assert.Contains(t, stdout, `level=info`, "Expected logfmt 'level=info' to be present in non-TTY")

	// Assert k6 format is NOT present in non-TTY
	assert.NotContains(t, stdout, "INFO[0000]", "Expected k6 format 'INFO[0000]' to NOT be present in non-TTY")

	// Assert ANSI color codes are NOT present
	assert.NotContains(t, stdout, "\x1b[", "Expected ANSI color codes to NOT be present")
}

// TestK6NoColorEnvironmentVariableWithNonTTY verifies that in non-TTY environments,
// logfmt format is used regardless of K6_NO_COLOR setting.
func TestK6NoColorEnvironmentVariableWithNonTTY(t *testing.T) {
	t.Parallel()

	script := testScript

	ts := getSingleFileTestState(t, script, []string{"--log-output=stdout"}, 0)
	ts.Env["K6_NO_COLOR"] = "true"

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()

	// In non-TTY (test environment), expect logfmt format
	assert.Contains(t, stdout, `level=info`, "Expected logfmt 'level=info' to be present in non-TTY")

	// Assert k6 format is NOT present in non-TTY
	assert.NotContains(t, stdout, "INFO[0000]", "Expected k6 format 'INFO[0000]' to NOT be present in non-TTY")

	// Assert ANSI color codes are NOT present
	assert.NotContains(t, stdout, "\x1b[", "Expected ANSI color codes to NOT be present")
}

// TestNoColorFlagWithNonTTY verifies that in non-TTY environments,
// logfmt format is used regardless of --no-color flag.
func TestNoColorFlagWithNonTTY(t *testing.T) {
	t.Parallel()

	script := testScript

	ts := getSingleFileTestState(t, script, []string{"--no-color", "--log-output=stdout"}, 0)

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()

	// In non-TTY (test environment), expect logfmt format
	assert.Contains(t, stdout, `level=info`, "Expected logfmt 'level=info' to be present in non-TTY")

	// Assert k6 format is NOT present in non-TTY
	assert.NotContains(t, stdout, "INFO[0000]", "Expected k6 format 'INFO[0000]' to NOT be present in non-TTY")

	// Assert ANSI color codes are NOT present
	assert.NotContains(t, stdout, "\x1b[", "Expected ANSI color codes to NOT be present")
}

// TestNonTTYStripsANSICodesFromConsoleLog verifies that ANSI escape sequences
// embedded in console.log messages are properly stripped in non-TTY environments.
func TestNonTTYStripsANSICodesFromConsoleLog(t *testing.T) {
	t.Parallel()

	// Script with embedded ANSI color codes
	script := `export default function() {
		console.log('\x1b[31mred text\x1b[0m normal');
	};`

	ts := getSingleFileTestState(t, script, []string{"--no-color", "--log-output=stdout"}, 0)

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()

	// In non-TTY (test environment), expect logfmt format
	assert.Contains(t, stdout, `level=info`, "Expected logfmt 'level=info' to be present in non-TTY")

	// Assert ANSI color codes are NOT present (neither from k6 nor from script)
	assert.NotContains(t, stdout, "\x1b[31m", "Expected ANSI color code \\x1b[31m to be stripped")
	assert.NotContains(t, stdout, "\x1b[0m", "Expected ANSI color code \\x1b[0m to be stripped")

	// Assert the actual text content is present (without ANSI codes)
	assert.Contains(t, stdout, "red text", "Expected 'red text' content to be present")
	assert.Contains(t, stdout, "normal", "Expected 'normal' content to be present")
}

// TestNonTTYUsesLogfmtAcrossLogLevels verifies that logfmt format
// is used across different log levels (info, warn, error) in non-TTY environments.
func TestNonTTYUsesLogfmtAcrossLogLevels(t *testing.T) {
	t.Parallel()

	script := `
		export default function() {
			console.log('info level');
			console.warn('warning level');
			console.error('error level');
		};
	`

	ts := getSingleFileTestState(t, script, []string{"--no-color", "--log-output=stdout"}, 0)

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()

	// In non-TTY, expect logfmt format for all log levels
	assert.Contains(t, stdout, `level=info`, "Expected logfmt 'level=info' to be present in non-TTY")
	assert.Contains(t, stdout, `level=warning`, "Expected logfmt 'level=warning' to be present in non-TTY")

	// Assert k6 format is NOT present in non-TTY
	assert.NotContains(t, stdout, "INFO[0000]", "Expected k6 format 'INFO[0000]' to NOT be present in non-TTY")
	assert.NotContains(t, stdout, "WARN[0000]", "Expected k6 format 'WARN[0000]' to NOT be present in non-TTY")
}

// TestNonTTYUsesLogfmtByDefault is a regression test to ensure non-TTY
// environments use logfmt format by default.
func TestNonTTYUsesLogfmtByDefault(t *testing.T) {
	t.Parallel()

	script := testScript

	ts := getSingleFileTestState(t, script, []string{"--log-output=stdout"}, 0)
	// Deliberately NOT setting NO_COLOR

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()

	// In non-TTY (test environment), expect logfmt format by default
	assert.Contains(t, stdout, `level=info`, "Expected logfmt 'level=info' to be present in non-TTY")

	// Assert k6 format is NOT present in non-TTY
	assert.NotContains(t, stdout, "INFO[0000]", "Expected k6 format 'INFO[0000]' to NOT be present in non-TTY")
}

// TestNonTTYUsesLogfmtWithDifferentLogOutputs verifies that non-TTY environments
// use logfmt format across different log output targets (stderr and stdout).
func TestNonTTYUsesLogfmtWithDifferentLogOutputs(t *testing.T) {
	t.Parallel()

	script := testScript

	t.Run("stderr output", func(t *testing.T) {
		t.Parallel()
		ts := getSingleFileTestState(t, script, []string{"--no-color", "--log-output=stderr"}, 0)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stderr := ts.Stderr.String()
		// In non-TTY, expect logfmt format
		assert.Contains(t, stderr, `level=info`, "Expected logfmt 'level=info' in stderr for non-TTY")
		assert.NotContains(t, stderr, "INFO[0000]", "Expected k6 format 'INFO[0000]' to NOT be in stderr for non-TTY")
		assert.NotContains(t, stderr, "\x1b[", "Expected ANSI codes to NOT be in stderr")
	})

	t.Run("stdout output", func(t *testing.T) {
		t.Parallel()
		ts := getSingleFileTestState(t, script, []string{"--no-color", "--log-output=stdout"}, 0)

		cmd.ExecuteWithGlobalState(ts.GlobalState)

		stdout := ts.Stdout.String()
		// In non-TTY, expect logfmt format
		assert.Contains(t, stdout, `level=info`, "Expected logfmt 'level=info' in stdout for non-TTY")
		assert.NotContains(t, stdout, "INFO[0000]", "Expected k6 format 'INFO[0000]' to NOT be in stdout for non-TTY")
		assert.NotContains(t, stdout, "\x1b[", "Expected ANSI codes to NOT be in stdout")
	})
}

// TestNoColorDoesNotAffectJSONFormat verifies that NO_COLOR only affects text format
// and doesn't interfere with JSON logging format.
func TestNoColorDoesNotAffectJSONFormat(t *testing.T) {
	t.Parallel()

	script := testScript

	ts := getSingleFileTestState(t, script, []string{"--no-color", "--log-output=stdout", "--log-format=json"}, 0)
	ts.Env["NO_COLOR"] = ""

	cmd.ExecuteWithGlobalState(ts.GlobalState)

	stdout := ts.Stdout.String()

	// Assert JSON format is still present
	assert.Contains(t, stdout, `"level":"info"`, "Expected JSON 'level' field to be present")
	assert.Contains(t, stdout, `"msg":"test message"`, "Expected JSON 'msg' field with test message")

	// Assert it's valid JSON by unmarshaling a line
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	foundValidJSON := false
	for _, line := range lines {
		if strings.Contains(line, "test message") {
			var logEntry map[string]any
			err := json.Unmarshal([]byte(line), &logEntry)
			if err == nil {
				foundValidJSON = true
				assert.Equal(t, "info", logEntry["level"], "Expected JSON level to be 'info'")
				assert.Contains(t, logEntry["msg"], "test message", "Expected JSON msg to contain 'test message'")
				break
			}
		}
	}
	assert.True(t, foundValidJSON, "Expected to find at least one valid JSON log entry")
}
