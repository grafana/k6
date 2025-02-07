package grpc_test

import (
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/grafana/sobek"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/internal/lib/testutils/httpmultibin"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/metrics"
	"gopkg.in/guregu/null.v3"

	xk6grpc "go.k6.io/k6/internal/js/modules/k6/grpc"
)

const isWindows = runtime.GOOS == "windows"

// codeBlock represents an execution of a k6 script.
type codeBlock struct {
	code       string
	val        interface{}
	err        string
	windowsErr string
	asserts    func(*testing.T, *httpmultibin.HTTPMultiBin, chan metrics.SampleContainer, error)
}

type testcase struct {
	name       string
	setup      func(*httpmultibin.HTTPMultiBin)
	initString codeBlock // runs in the init context
	vuString   codeBlock // runs in the vu context
}

// callRecorder a helper type that records all calls
type callRecorder struct {
	sync.Mutex
	calls []string
}

// Call records a call
func (r *callRecorder) Call(text string) {
	r.Lock()
	defer r.Unlock()

	r.calls = append(r.calls, text)
}

// Len just returns the length of the calls
func (r *callRecorder) Len() int {
	r.Lock()
	defer r.Unlock()

	return len(r.calls)
}

// Recorded returns the recorded calls
func (r *callRecorder) Recorded() []string {
	r.Lock()
	defer r.Unlock()

	result := []string{}
	result = append(result, r.calls...)

	return result
}

type testState struct {
	*modulestest.Runtime
	httpBin      *httpmultibin.HTTPMultiBin
	samples      chan metrics.SampleContainer
	logger       logrus.FieldLogger
	loggerHook   *testutils.SimpleLogrusHook
	callRecorder *callRecorder
}

// Run replaces the httpbin address and runs the code.
func (ts *testState) Run(code string) (sobek.Value, error) {
	return ts.VU.Runtime().RunString(ts.httpBin.Replacer.Replace(code))
}

// RunOnEventLoop replaces the httpbin address and run the code on event loop
func (ts *testState) RunOnEventLoop(code string) (sobek.Value, error) {
	return ts.Runtime.RunOnEventLoop(ts.httpBin.Replacer.Replace(code))
}

// newTestState creates a new test state.
func newTestState(t *testing.T) testState {
	t.Helper()

	tb := httpmultibin.NewHTTPMultiBin(t)

	samples := make(chan metrics.SampleContainer, 1000)
	testRuntime := modulestest.NewRuntime(t)

	cwd, err := os.Getwd() //nolint:forbidigo
	require.NoError(t, err)
	fs := fsext.NewOsFs()

	if isWindows {
		fs = fsext.NewTrimFilePathSeparatorFs(fs)
	}
	testRuntime.VU.InitEnvField.CWD = &url.URL{Scheme: "file", Path: filepath.ToSlash(cwd)}
	testRuntime.VU.InitEnvField.FileSystems = map[string]fsext.Fs{"file": fs}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.Out = io.Discard

	hook := testutils.NewLogHook()
	logger.AddHook(hook)

	recorder := &callRecorder{
		calls: make([]string, 0),
	}

	ts := testState{
		Runtime:      testRuntime,
		httpBin:      tb,
		samples:      samples,
		logger:       logger,
		loggerHook:   hook,
		callRecorder: recorder,
	}

	m, ok := xk6grpc.New().NewModuleInstance(ts.VU).(*xk6grpc.ModuleInstance)
	require.True(t, ok)
	require.NoError(t, ts.VU.Runtime().Set("grpc", m.Exports().Named))
	require.NoError(t, ts.VU.Runtime().Set("call", recorder.Call))

	return ts
}

// ToVUContext moves the test state to the VU context.
func (ts *testState) ToVUContext() {
	registry := metrics.NewRegistry()

	state := &lib.State{
		Dialer:    ts.httpBin.Dialer,
		TLSConfig: ts.httpBin.TLSClientConfig,
		Samples:   ts.samples,
		Options: lib.Options{
			SystemTags: metrics.NewSystemTagSet(
				metrics.TagName,
				metrics.TagURL,
			),
			UserAgent: null.StringFrom("k6-test"),
		},
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(registry),
		Tags:           lib.NewVUStateTags(registry.RootTagSet()),
		Logger:         ts.logger,
	}

	ts.MoveToVUContext(state)
}
