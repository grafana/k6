package cmd

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/cmd/tests"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/lib/testutils"
)

func TestMain(m *testing.M) {
	tests.Main(m)
}

func TestPanicHandling(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "panic"}
	ts.ExpectedExitCode = int(exitcodes.GoPanic)

	rootCmd := newRootCommand(ts.GlobalState)
	rootCmd.cmd.AddCommand(&cobra.Command{
		Use: "panic",
		RunE: func(cmd *cobra.Command, args []string) error {
			panic("oh no, oh no, oh no,no,no,no,no")
		},
	})
	rootCmd.execute()

	t.Log(ts.Stderr.String())
	logMsgs := ts.LoggerHook.Drain()
	assert.True(t, testutils.LogContains(logMsgs, logrus.ErrorLevel, "unexpected k6 panic: oh no"))
	assert.True(t, testutils.LogContains(logMsgs, logrus.ErrorLevel, "cmd.TestPanicHandling")) // check stacktrace
}
