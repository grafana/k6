package tests

import (
	"bytes"
	"context"
	"net"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/cmd/state"
	"go.k6.io/k6/internal/event"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/ui/console"
	"go.k6.io/k6/lib/fsext"
)

// GlobalTestState is a wrapper around GlobalState for use in tests.
type GlobalTestState struct {
	*state.GlobalState
	Cancel func()

	Stdout, Stderr *bytes.Buffer
	LoggerHook     *testutils.SimpleLogrusHook

	Cwd string

	ExpectedExitCode int
}

// NewGlobalTestState returns an initialized GlobalTestState, mocking all
// GlobalState fields for use in tests.
func NewGlobalTestState(tb testing.TB) *GlobalTestState {
	ctx, cancel := context.WithCancel(context.Background())
	tb.Cleanup(cancel)

	fs := fsext.NewMemMapFs()
	cwd := "/test/" // TODO: Make this relative to the test?
	if runtime.GOOS == "windows" {
		cwd = "c:\\test\\"
	}
	require.NoError(tb, fs.MkdirAll(cwd, 0o755))

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.Out = testutils.NewTestOutput(tb)
	hook := testutils.NewLogHook()
	logger.AddHook(hook)

	ts := &GlobalTestState{
		Cwd:        cwd,
		Cancel:     cancel,
		LoggerHook: hook,
		Stdout:     new(bytes.Buffer),
		Stderr:     new(bytes.Buffer),
	}

	osExitCalled := false
	defaultOsExitHandle := func(exitCode int) {
		cancel()
		osExitCalled = true
		assert.Equal(tb, ts.ExpectedExitCode, exitCode)
	}

	tb.Cleanup(func() {
		if ts.ExpectedExitCode > 0 {
			// Ensure that, if we expected to receive an error, our `os.Exit()` mock
			// function was actually called.
			assert.Truef(tb,
				osExitCalled,
				"expected exit code %d, but the os.Exit() mock was not called",
				ts.ExpectedExitCode,
			)
		}
	})

	outMutex := &sync.Mutex{}
	defaultFlags := state.GetDefaultFlags(".config")
	defaultFlags.Address = getFreeBindAddr(tb)

	ts.GlobalState = &state.GlobalState{
		Ctx:          ctx,
		FS:           fs,
		Getwd:        func() (string, error) { return ts.Cwd, nil },
		BinaryName:   "k6",
		CmdArgs:      []string{},
		Env:          map[string]string{"K6_NO_USAGE_REPORT": "true"},
		Events:       event.NewEventSystem(100, logger),
		DefaultFlags: defaultFlags,
		Flags:        defaultFlags,
		OutMutex:     outMutex,
		Stdout: &console.Writer{
			Mutex:  outMutex,
			Writer: ts.Stdout,
			IsTTY:  false,
		},
		Stderr: &console.Writer{
			Mutex:  outMutex,
			Writer: ts.Stderr,
			IsTTY:  false,
		},
		Stdin:          new(bytes.Buffer),
		OSExit:         defaultOsExitHandle,
		SignalNotify:   signal.Notify,
		SignalStop:     signal.Stop,
		Logger:         logger,
		FallbackLogger: testutils.NewLogger(tb).WithField("fallback", true),
	}

	return ts
}

var portRangeStart uint64 = 6565 //nolint:gochecknoglobals

func getFreeBindAddr(tb testing.TB) string {
	for i := 0; i < 100; i++ {
		port := atomic.AddUint64(&portRangeStart, 1)
		addr := net.JoinHostPort("localhost", strconv.FormatUint(port, 10))

		listener, err := net.Listen("tcp", addr)
		if err != nil {
			continue // port was busy for some reason
		}
		defer func() {
			assert.NoError(tb, listener.Close())
		}()
		return addr
	}

	tb.Fatal("could not get a free port")
	return ""
}
