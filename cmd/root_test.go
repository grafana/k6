package cmd

import (
	"bytes"
	"context"
	"os/signal"
	"runtime"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/testutils"
)

type globalTestState struct {
	*globalState
	cancel func()

	stdOut, stdErr *bytes.Buffer
	loggerHook     *testutils.SimpleLogrusHook

	cwd string

	expectedExitCode int
}

func newGlobalTestState(t *testing.T) *globalTestState {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fs := &afero.MemMapFs{}
	cwd := "/test/"
	if runtime.GOOS == "windows" {
		cwd = "c:\\test\\"
	}
	require.NoError(t, fs.MkdirAll(cwd, 0o755))

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.Out = testutils.NewTestOutput(t)
	hook := &testutils.SimpleLogrusHook{HookedLevels: logrus.AllLevels}
	logger.AddHook(hook)

	ts := &globalTestState{
		cwd:        cwd,
		cancel:     cancel,
		loggerHook: hook,
		stdOut:     new(bytes.Buffer),
		stdErr:     new(bytes.Buffer),
	}

	defaultOsExitHandle := func(exitCode int) {
		cancel()
		require.Equal(t, ts.expectedExitCode, exitCode)
	}

	outMutex := &sync.Mutex{}
	defaultFlags := getDefaultFlags(".config")

	// Set an empty REST API address by default so that `k6 run` dosen't try to
	// bind to it, which will result in parallel integration tests trying to use
	// the same port and a warning message in every one.
	defaultFlags.address = ""

	ts.globalState = &globalState{
		ctx:            ctx,
		fs:             fs,
		getwd:          func() (string, error) { return ts.cwd, nil },
		args:           []string{},
		envVars:        map[string]string{},
		defaultFlags:   defaultFlags,
		flags:          defaultFlags,
		outMutex:       outMutex,
		stdOut:         &consoleWriter{nil, ts.stdOut, false, outMutex, nil},
		stdErr:         &consoleWriter{nil, ts.stdErr, false, outMutex, nil},
		stdIn:          new(bytes.Buffer),
		osExit:         defaultOsExitHandle,
		signalNotify:   signal.Notify,
		signalStop:     signal.Stop,
		logger:         logger,
		fallbackLogger: testutils.NewLogger(t).WithField("fallback", true),
	}
	return ts
}

func TestDeprecatedOptionWarning(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)
	ts.args = []string{"k6", "--logformat", "json", "run", "-"}
	ts.stdIn = bytes.NewBuffer([]byte(`
		console.log('foo');
		export default function() { console.log('bar'); };
	`))

	newRootCommand(ts.globalState).execute()

	logMsgs := ts.loggerHook.Drain()
	assert.True(t, testutils.LogContains(logMsgs, logrus.InfoLevel, "foo"))
	assert.True(t, testutils.LogContains(logMsgs, logrus.InfoLevel, "bar"))
	assert.Contains(t, ts.stdErr.String(), `"level":"info","msg":"foo","source":"console"`)
	assert.Contains(t, ts.stdErr.String(), `"level":"info","msg":"bar","source":"console"`)

	// TODO: after we get rid of cobra, actually emit this message to stderr
	// and, ideally, through the log, not just print it...
	assert.False(t, testutils.LogContains(logMsgs, logrus.InfoLevel, "logformat"))
	assert.Contains(t, ts.stdOut.String(), `--logformat has been deprecated`)
}
