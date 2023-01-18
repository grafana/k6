package cmd

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/cmd/tests"
	"go.k6.io/k6/lib/testutils"
)

func TestMain(m *testing.M) {
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
