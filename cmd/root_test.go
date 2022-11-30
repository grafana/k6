package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
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

type blockingTransport struct {
	fallback       http.RoundTripper
	forbiddenHosts map[string]bool
	counter        uint32
}

func (bt *blockingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Hostname()
	if bt.forbiddenHosts[host] {
		atomic.AddUint32(&bt.counter, 1)
		panic(fmt.Errorf("trying to make forbidden request to %s during test", host))
	}
	return bt.fallback.RoundTrip(req)
}

func TestMain(m *testing.M) {
	exitCode := 1 // error out by default
	defer func() {
		os.Exit(exitCode)
	}()

	bt := &blockingTransport{
		fallback: http.DefaultTransport,
		forbiddenHosts: map[string]bool{
			"ingest.k6.io":    true,
			"cloudlogs.k6.io": true,
			"app.k6.io":       true,
			"reports.k6.io":   true,
		},
	}
	http.DefaultTransport = bt
	defer func() {
		if bt.counter > 0 {
			fmt.Printf("Expected blocking transport count to be 0 but was %d\n", bt.counter) //nolint:forbidigo
			exitCode = 2
		}
	}()

	// TODO: add https://github.com/uber-go/goleak

	exitCode = m.Run()
}

type bufferStringer interface {
	io.ReadWriter
	fmt.Stringer
	Bytes() []byte
}

type globalTestState struct {
	*globalState
	cancel func()

	stdOut, stdErr bufferStringer
	loggerHook     *testutils.SimpleLogrusHook

	cwd string

	expectedExitCode int
}

// A thread-safe buffer implementation.
type safeBuffer struct {
	b bytes.Buffer
	m sync.RWMutex
}

func (b *safeBuffer) Read(p []byte) (n int, err error) {
	b.m.RLock()
	defer b.m.RUnlock()
	return b.b.Read(p)
}

func (b *safeBuffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}

func (b *safeBuffer) String() string {
	b.m.RLock()
	defer b.m.RUnlock()
	return b.b.String()
}

func (b *safeBuffer) Bytes() []byte {
	b.m.RLock()
	defer b.m.RUnlock()
	return b.b.Bytes()
}

type testOSFileW struct {
	io.Writer
}

func (f *testOSFileW) Fd() uintptr {
	return 0
}

type testOSFileR struct {
	io.Reader
}

func (f *testOSFileR) Fd() uintptr {
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
		stdOut:     &safeBuffer{},
		stdErr:     &safeBuffer{},
	}

	osExitCalled := false
	defaultOsExitHandle := func(exitCode int) {
		cancel()
		osExitCalled = true
		assert.Equal(t, ts.expectedExitCode, exitCode)
	}

	t.Cleanup(func() {
		if ts.expectedExitCode > 0 {
			// Ensure that, if we expected to receive an error, our `os.Exit()` mock
			// function was actually called.
			assert.Truef(t, osExitCalled, "expected exit code %d, but the os.Exit() mock was not called", ts.expectedExitCode)
		}
	})

	defaultFlags := getDefaultFlags(".config")
	defaultFlags.address = getFreeBindAddr(t)

	cons := console.New(
		&testOSFileW{ts.stdOut}, &testOSFileW{ts.stdErr},
		&testOSFileR{&safeBuffer{}}, false, "", signal.Notify, signal.Stop)
	cons.SetLogger(logger)

	ts.globalState = &globalState{
		ctx:          ctx,
		fs:           fs,
		console:      cons,
		getwd:        func() (string, error) { return ts.cwd, nil },
		args:         []string{},
		envVars:      map[string]string{"K6_NO_USAGE_REPORT": "true"},
		defaultFlags: defaultFlags,
		flags:        defaultFlags,
		osExit:       defaultOsExitHandle,
		signalNotify: signal.Notify,
		signalStop:   signal.Stop,
		logger:       logger,
	}
	return ts
}

func TestDeprecatedOptionWarning(t *testing.T) {
	t.Parallel()

	ts := newGlobalTestState(t)
	ts.args = []string{"k6", "--logformat", "json", "run", "-"}
	ts.console.Stdin = &testOSFileR{bytes.NewBuffer([]byte(`
		console.log('foo');
		export default function() { console.log('bar'); };
	`))}

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
