package cmd

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

func executeCommand(root *cobra.Command, args ...string) (output string, err error) {
	_, output, err = executeCommandC(root, args...)
	return output, err
}

func executeCommandC(root *cobra.Command, args ...string) (c *cobra.Command, output string, err error) {
	buf := new(bytes.Buffer)
	root.SetOutput(buf)
	root.SetArgs(args)
	c, err = root.ExecuteC()
	return c, buf.String(), err
}

func TestCommandReturnErrorWhenServerNotRunning(t *testing.T) {
	tests := []struct {
		command *cobra.Command
		name    string
	}{
		{statusCmd, "status"},
		{statsCmd, "stats"},
	}

	for _, tc := range tests {
		RootCmd.AddCommand(tc.command)
		_, err := executeCommand(RootCmd, tc.name)
		if err == nil {
			t.Errorf("Command %s expected error, got nil", tc.name)
		}
		RootCmd.RemoveCommand(tc.command)
	}
}
