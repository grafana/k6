package tests

import (
	"bytes"
	"context"
	"os/signal"
	"regexp"
	"runtime"
	"sync"
	"testing"
	_ "unsafe" //nolint:revive,nolintlint // needed for the go:linkname and nolintlint is buggy

	"go.k6.io/k6/v2/lib"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/internal/event"
	"go.k6.io/k6/v2/internal/lib/testutils"
	"go.k6.io/k6/v2/internal/ui/console"
	"go.k6.io/k6/v2/internal/usage"
	"go.k6.io/k6/v2/lib/fsext"
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
	tb.Helper()
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
	defaultFlags := state.GetDefaultFlags(".config", ".cache")

	ts.GlobalState = &state.GlobalState{
		Ctx:             ctx,
		FS:              fs,
		Getwd:           func() (string, error) { return ts.Cwd, nil },
		UserOSConfigDir: ".config",
		BinaryName:      "k6",
		CmdArgs:         []string{},
		Env: map[string]string{
			"K6_NO_USAGE_REPORT": "true",
			// Keep `k6 x` command-tree construction off the real registry.k6.io
			// when tests don't explicitly point it at an httptest server.
			state.ProvisionCatalogURL: "http://127.0.0.1:1/unreachable",
		},
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
		Usage:          usage.New(),
		TestStatus:     lib.NewTestStatus(),
	}

	return ts
}

// ReparseFlags reparses flags so it can take into account changes to env variables and arguments
func (ts *GlobalTestState) ReparseFlags() {
	defaultFlags := getFlags(ts.DefaultFlags, ts.Env, ts.CmdArgs)
	ts.DefaultFlags = defaultFlags
	ts.Flags = defaultFlags
}

// TODO(@mstoykov): Figure out how to not do it this way and not have more public APIs
// Also use this for testing more of the GlobalState.Flags
//
//go:linkname getFlags go.k6.io/k6/v2/cmd/state.getFlags
func getFlags(defaultFlags state.GlobalFlags, env map[string]string, args []string) state.GlobalFlags

// apiServerAddrPattern matches the debug log line k6 writes when it starts the REST API server.
var apiServerAddrPattern = regexp.MustCompile(`Starting the REST API server on ([^"\s]+)`) //nolint:gochecknoglobals

// apiServerAddr returns the address the REST API server is listening on, as k6 logged it.
//
// Tests start k6 with --address localhost:0, so the OS picks a free port when k6 listens.
// Nothing has to find a free port up front and release it again, which raced with other
// tests and processes taking the same port. The test must run k6 with -v and
// --log-output=stdout, and call this after the server has started.
func apiServerAddr(tb testing.TB, ts *GlobalTestState) string {
	tb.Helper()

	ts.OutMutex.Lock()
	stdout := ts.Stdout.String()
	ts.OutMutex.Unlock()

	match := apiServerAddrPattern.FindStringSubmatch(stdout)
	require.NotNil(tb, match, "the REST API server address was not logged")
	return match[1]
}
