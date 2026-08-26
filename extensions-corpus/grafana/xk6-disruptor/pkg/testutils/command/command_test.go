package command

import (
	"testing"
)

func Test_CompareCommands(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title    string
		expected string
		actual   string
		result   bool
	}{
		{
			title:    "identical commands",
			expected: "cmd subcmd -a a -b b arg",
			actual:   "cmd subcmd -a a -b b arg",
			result:   true,
		},
		{
			title:    "same options",
			expected: "cmd subcmd -a a -b b arg",
			actual:   "cmd -a a -b b subcmd arg",
			result:   true,
		},
		{
			title:    "multiple spaces",
			expected: "cmd subcmd -a a -b b arg",
			actual:   "cmd subcmd -a a -b b  arg",
			result:   true,
		},
		{
			title:    "different order of flags",
			expected: "cmd subcmd -a a -b b arg",
			actual:   "cmd subcmd -b b -a a arg",
			result:   true,
		},
		{
			title:    "different order of options",
			expected: "cmd subcmd -a a -b b arg",
			actual:   "cmd -a a -b b arg subcommand",
			result:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			if AssertCmdEquals(tc.expected, tc.actual) != tc.result {
				t.Errorf("expected: %s actual: %s result %t", tc.expected, tc.actual, tc.result)
			}
		})
	}
}
