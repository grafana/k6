package cmd

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/cmd/tests"
	"go.k6.io/k6/errext/exitcodes"
	"go.k6.io/k6/ext"
	"go.k6.io/k6/lib/testutils"
)

func TestMain(m *testing.M) {
	RegisterExtension(testCommandExtensionName, testCommandExtension)
	tests.Main(m)
}

func TestDeprecatedOptionWarning(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "--logformat", "json", "run", "-"}
	ts.Stdin = bytes.NewBuffer([]byte(`
		console.log('foo');
		export default function() { console.log('bar'); };
	`))

	newRootCommand(ts.GlobalState).execute()

	logMsgs := ts.LoggerHook.Drain()
	assert.True(t, testutils.LogContains(logMsgs, logrus.InfoLevel, "foo"))
	assert.True(t, testutils.LogContains(logMsgs, logrus.InfoLevel, "bar"))
	assert.Contains(t, ts.Stderr.String(), `"level":"info","msg":"foo","source":"console"`)
	assert.Contains(t, ts.Stderr.String(), `"level":"info","msg":"bar","source":"console"`)

	// TODO: after we get rid of cobra, actually emit this message to stderr
	// and, ideally, through the log, not just print it...
	assert.False(t, testutils.LogContains(logMsgs, logrus.InfoLevel, "logformat"))
	assert.Contains(t, ts.Stdout.String(), `--logformat has been deprecated`)
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

func TestCommandExtensionHandling(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", testCommandExtensionName}
	ts.ExpectedExitCode = 0

	rootCmd := newRootCommand(ts.GlobalState)
	rootCmd.execute()

	t.Log(ts.Stderr.String())
	logMsgs := ts.LoggerHook.Drain()
	assert.True(t, testutils.LogContains(logMsgs, logrus.InfoLevel, "Hello Extension Command!"))
}

func Test_getCommandExtensions(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "foo"}
	ts.ExpectedExitCode = 0

	rootCmd := newRootCommand(ts.GlobalState)

	all := ext.Get(ext.CommandExtension)

	exts := map[string]*ext.Extension{
		"bar": all[testCommandExtensionName],
	}

	exts["bar"].Name = "bar"

	ctors, err := getCommandExtensions(rootCmd.cmd, exts)

	assert.NoError(t, err)
	assert.NotEmpty(t, ctors)
	assert.Len(t, ctors, 1)
}

func Test_getCommandExtensions_Error_builtin(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "dummy"}
	ts.ExpectedExitCode = 0

	rootCmd := newRootCommand(ts.GlobalState)

	ver := getCmdVersion(ts.GlobalState)
	ver.Use = testCommandExtensionName
	rootCmd.cmd.AddCommand(ver)

	_, err := getCommandExtensions(rootCmd.cmd, ext.Get(ext.CommandExtension))

	assert.Error(t, err)
	assert.ErrorContains(t, err, "built-in command with the same name already exists")
}

func Test_getCommandExtensions_Error_invalid_ctor(t *testing.T) {
	t.Parallel()

	ts := tests.NewGlobalTestState(t)
	ts.CmdArgs = []string{"k6", "dummy"}
	ts.ExpectedExitCode = 0

	rootCmd := newRootCommand(ts.GlobalState)

	exts := map[string]*ext.Extension{
		"foo": {
			Name:    "foo",
			Path:    "",
			Version: "",
			Type:    ext.CommandExtension,
			Module:  TestMain,
		},
	}

	_, err := getCommandExtensions(rootCmd.cmd, exts)

	assert.Error(t, err)
	assert.ErrorContains(t, err, "unexpected command extension type")
}

func testCommandExtension(gs *state.GlobalState) (*cobra.Command, error) {
	return &cobra.Command{
		Use: testCommandExtensionName,
		Run: func(cmd *cobra.Command, args []string) {
			gs.Logger.Info("Hello Extension Command!")
		},
	}, nil
}

const testCommandExtensionName = "command-from-extension"
