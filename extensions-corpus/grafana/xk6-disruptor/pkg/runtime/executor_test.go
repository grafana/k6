package runtime

import (
	"testing"
)

func Test_Exec(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title        string
		cmd          string
		args         []string
		expectError  bool
		expectOutput string
	}{
		{
			title:        "return output",
			cmd:          "echo",
			args:         []string{"-n", "hello world"},
			expectError:  false,
			expectOutput: "hello world",
		},
		{
			title:        "return stderr",
			cmd:          "sh",
			args:         []string{"-c", "echo hello world 2>&1"},
			expectError:  false,
			expectOutput: "hello world\n",
		},
		{
			title:        "do not return output",
			cmd:          "true",
			expectError:  false,
			expectOutput: "",
		},
		{
			title:        "command return error code",
			cmd:          "false",
			expectError:  true,
			expectOutput: "",
		},
		{
			title:        "return error code and stderr",
			cmd:          "sh",
			args:         []string{"-c", "echo hello world 2>&1; kill -KILL $$"},
			expectError:  true,
			expectOutput: "hello world\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			executor := DefaultExecutor()
			out, err := executor.Exec(tc.cmd, tc.args...)
			if err != nil {
				t.Logf("error: %v", err)
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error %v", err)
				return
			}

			if string(out) != tc.expectOutput {
				t.Errorf(
					"returned output does not match expected value.\nExpected: %s\nActual: %s",
					tc.expectOutput,
					string(out),
				)
				return
			}
		})
	}
}
