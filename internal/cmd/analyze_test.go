package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScriptNameFromArgs(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		args     []string
		expected string
	}{
		{
			name:     "empty args",
			args:     []string{},
			expected: "",
		},
		{
			name:     "only flags",
			args:     []string{"-v", "--verbose"},
			expected: "",
		},
		{
			name:     "run with script at end",
			args:     []string{"run", "script.js"},
			expected: "script.js",
		},
		{
			name:     "run with script and flags",
			args:     []string{"run", "script.js", "-v"},
			expected: "script.js",
		},
		{
			name:     "run with flags before script",
			args:     []string{"run", "-v", "script.js"},
			expected: "script.js",
		},
		{
			name:     "run with verbose flag before script",
			args:     []string{"run", "--verbose", "script.js"},
			expected: "script.js",
		},
		{
			name:     "run with flag with value before script",
			args:     []string{"run", "--console-output", "loadtest.log", "script.js"},
			expected: "script.js",
		},
		{
			name:     "run with script before flag with value",
			args:     []string{"run", "script.js", "--console-output", "loadtest2.log"},
			expected: "script.js",
		},
		{
			name:     "cloud run with script",
			args:     []string{"cloud", "run", "archive.tar"},
			expected: "archive.tar",
		},
		{
			name:     "cloud run with script and flags",
			args:     []string{"cloud", "run", "archive.tar", "-v"},
			expected: "archive.tar",
		},
		{
			name:     "cloud with script and flags",
			args:     []string{"cloud", "archive.tar", "-v"},
			expected: "archive.tar",
		},
		{
			name:     "cloud with flags and script",
			args:     []string{"cloud", "--console-output", "loadtest.log", "script.js"},
			expected: "script.js",
		},
		{
			name:     "cloud with script and flags with value",
			args:     []string{"cloud", "script.js", "--console-output", "loadtest2.log"},
			expected: "script.js",
		},
		{
			name:     "flags before command",
			args:     []string{"-v", "run", "script.js"},
			expected: "script.js",
		},
		{
			name:     "complex case with multiple flags",
			args:     []string{"-v", "--quiet", "run", "-o", "output.json", "--console-output", "loadtest.log", "script.js", "--tag", "env=staging"},
			expected: "script.js",
		},
		{
			name:     "no script file",
			args:     []string{"run", "-v", "--quiet"},
			expected: "",
		},
		{
			name:     "non-script file",
			args:     []string{"run", "notascript.txt"},
			expected: "",
		},
		{
			name:     "ts extension",
			args:     []string{"run", "script.ts"},
			expected: "script.ts",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			script := scriptNameFromArgs(tc.args)

			assert.Equal(t, tc.expected, script)
		})
	}
}

func TestIsScriptRequired(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "empty args",
			args:     []string{},
			expected: false,
		},
		{
			name:     "only flags",
			args:     []string{"-h"},
			expected: false,
		},
		{
			name:     "command does not take script",
			args:     []string{"version"},
			expected: false,
		},
		{
			name:     "run command",
			args:     []string{"run", "script.js"},
			expected: true,
		},
		{
			name:     "flag before command",
			args:     []string{"-v", "run", "script.js"},
			expected: true,
		},
		{
			name:     "verbose flag before command",
			args:     []string{"--verbose", "run", "script.js"},
			expected: true,
		},
		{
			name:     "cloud run with flag in the middle",
			args:     []string{"cloud", "-v", "run", "archive.tar"},
			expected: true,
		},
		{
			name:     "cloud command",
			args:     []string{"cloud", "script.js"},
			expected: true,
		},
		{
			name:     "cloud login command",
			args:     []string{"cloud", "login"},
			expected: false,
		},
		{
			name:     "archive command",
			args:     []string{"archive", "script.js"},
			expected: true,
		},
		{
			name:     "inspect command",
			args:     []string{"inspect", "archive.tar"},
			expected: true,
		},
		{
			name:     "complex case with multiple flags",
			args:     []string{"-v", "--quiet", "run", "-o", "output.json", "--console-output", "loadtest.log", "script.js", "--tag", "env=staging"},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual := isScriptRequired(tc.args)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
