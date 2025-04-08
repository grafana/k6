package launcher

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScriptArg(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name      string
		args      []string
		expected  string
		hasScript bool
	}{
		{
			name:      "empty args",
			args:      []string{},
			expected:  "",
			hasScript: false,
		},
		{
			name:      "only flags",
			args:      []string{"-v", "--verbose"},
			expected:  "",
			hasScript: false,
		},
		{
			name:      "invalid command",
			args:      []string{"invalid", "script.js"},
			expected:  "",
			hasScript: false,
		},
		{
			name:      "run with script at end",
			args:      []string{"run", "script.js"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "run with script and flags",
			args:      []string{"run", "script.js", "-v"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "run with flags before script",
			args:      []string{"run", "-v", "script.js"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "run with verbose flag before script",
			args:      []string{"run", "--verbose", "script.js"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "run with flag with value before script",
			args:      []string{"run", "--console-output", "loadtest.log", "script.js"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "run with script before flag with value",
			args:      []string{"run", "script.js", "--console-output", "loadtest2.log"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "cloud run with script",
			args:      []string{"cloud", "run", "archive.tar"},
			expected:  "archive.tar",
			hasScript: true,
		},
		{
			name:      "cloud run with script and flags",
			args:      []string{"cloud", "run", "archive.tar", "-v"},
			expected:  "archive.tar",
			hasScript: true,
		},
		{
			name:      "cloud with script and flags",
			args:      []string{"cloud", "archive.tar", "-v"},
			expected:  "archive.tar",
			hasScript: true,
		},
		{
			name:      "cloud with flags and script",
			args:      []string{"cloud", "--console-output", "loadtest.log", "script.js"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "cloud with script and flags with value",
			args:      []string{"cloud", "script.js", "--console-output", "loadtest2.log"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "flags before command",
			args:      []string{"-v", "run", "script.js"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "complex case with multiple flags",
			args:      []string{"-v", "--quiet", "run", "-o", "output.json", "--console-output", "loadtest.log", "script.js", "--tag", "env=staging"},
			expected:  "script.js",
			hasScript: true,
		},
		{
			name:      "no script file",
			args:      []string{"run", "-v", "--quiet"},
			expected:  "",
			hasScript: false,
		},
		{
			name:      "non-script file",
			args:      []string{"run", "notascript.txt"},
			expected:  "",
			hasScript: false,
		},
		{
			name:      "ts extension",
			args:      []string{"run", "script.ts"},
			expected:  "script.ts",
			hasScript: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			script, hasScript := inputArg(tc.args)

			assert.Equal(t, tc.hasScript, hasScript)
			assert.Equal(t, tc.expected, script)
		})
	}
}
