package state

import (
	"bytes"
	"context"
	"io"
	"net"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/ui/console"
)

// GlobalTestState is a wrapper around GlobalState for use in tests.
type GlobalTestState struct {
	*GlobalState
	Cancel func()

	Stdout, Stderr *bytes.Buffer
	LoggerHook     *testutils.SimpleLogrusHook

	Cwd string

	ExpectedExitCode int
}

// NewGlobalTestState returns an initialized GlobalTestState, mocking all
// GlobalState fields for use in tests.
func NewGlobalTestState(t *testing.T) *GlobalTestState {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	fs := &afero.MemMapFs{}
	cwd := "/test/" // TODO: Make this relative to the test?
	if runtime.GOOS == "windows" {
		cwd = "c:\\test\\"
	}
	require.NoError(t, fs.MkdirAll(cwd, 0o755))

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.Out = testutils.NewTestOutput(t)
	hook := &testutils.SimpleLogrusHook{HookedLevels: logrus.AllLevels}
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
		assert.Equal(t, ts.ExpectedExitCode, exitCode)
	}

	t.Cleanup(func() {
		if ts.ExpectedExitCode > 0 {
			// Ensure that, if we expected to receive an error, our `os.Exit()` mock
			// function was actually called.
			assert.Truef(t,
				osExitCalled,
				"expected exit code %d, but the os.Exit() mock was not called",
				ts.ExpectedExitCode,
			)
		}
	})

	outMutex := &sync.Mutex{}
	defaultFlags := GetDefaultFlags(".config")
	defaultFlags.Address = getFreeBindAddr(t)

	ts.GlobalState = &GlobalState{
		Ctx:          ctx,
		FS:           fs,
		Getwd:        func() (string, error) { return ts.Cwd, nil },
		CmdArgs:      []string{},
		Env:          map[string]string{"K6_NO_USAGE_REPORT": "true"},
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
		FallbackLogger: testutils.NewLogger(t).WithField("fallback", true),
	}

	return ts
}

// TestOSFileW is the mock implementation of stdout/stderr.
type TestOSFileW struct {
	io.Writer
}

// Fd returns a mock file descriptor ID.
func (f *TestOSFileW) Fd() uintptr {
	return 0
}

// TestOSFileR is the mock implementation of stdin.
type TestOSFileR struct {
	io.Reader
}

// Fd returns a mock file descriptor ID.
func (f *TestOSFileR) Fd() uintptr {
	return 0
}

var portRangeStart uint64 = 6565 //nolint:gochecknoglobals

func getFreeBindAddr(t *testing.T) string {
	for i := 0; i < 100; i++ {
		port := atomic.AddUint64(&portRangeStart, 1)
		addr := net.JoinHostPort("localhost", strconv.FormatUint(port, 10))

		listener, err := net.Listen("tcp", addr)
		if err != nil {
			continue // port was busy for some reason
		}
		defer func() {
			assert.NoError(t, listener.Close())
		}()
		return addr
	}

	t.Fatal("could not get a free port")
	return ""
}
