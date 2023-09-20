package grpc_test

import (
	"errors"
	"io"
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gopkg.in/guregu/null.v3"

	k6grpc "go.k6.io/k6/js/modules/k6/grpc"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/fsext"
	"go.k6.io/k6/lib/testutils"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/metrics"
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

type testState struct {
	*modulestest.Runtime
	httpBin    *httpmultibin.HTTPMultiBin
	samples    chan metrics.SampleContainer
	logger     logrus.FieldLogger
	loggerHook *testutils.SimpleLogrusHook
}

func newTestState(t *testing.T) testState {
	t.Helper()

	tb := httpmultibin.NewHTTPMultiBin(t)
	samples := make(chan metrics.SampleContainer, 1000)
	testRuntime := modulestest.NewRuntime(t)

	cwd, err := os.Getwd() //nolint:golint,forbidigo
	require.NoError(t, err)
	fs := fsext.NewOsFs()
	if isWindows {
		fs = fsext.NewTrimFilePathSeparatorFs(fs)
	}
	testRuntime.VU.InitEnvField.CWD = &url.URL{Path: cwd}
	testRuntime.VU.InitEnvField.FileSystems = map[string]fsext.Fs{"file": fs}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	logger.Out = io.Discard

	hook := testutils.NewLogHook()
	logger.AddHook(hook)

	ts := testState{
		Runtime:    testRuntime,
		httpBin:    tb,
		samples:    samples,
		logger:     logger,
		loggerHook: hook,
	}

	m, ok := k6grpc.New().NewModuleInstance(ts.VU).(*k6grpc.ModuleInstance)
	require.True(t, ok)
	require.NoError(t, ts.VU.Runtime().Set("grpc", m.Exports().Named))

	return ts
}

// ToInitContext moves the test state to the VU context.
func (ts *testState) ToVUContext() {
	registry := metrics.NewRegistry()
	root, err := lib.NewGroup("", nil)
	if err != nil {
		panic(err)
	}

	state := &lib.State{
		Group:     root,
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

// Run replaces the httpbin address and runs the code.
func (ts *testState) Run(code string) (goja.Value, error) {
	return ts.VU.Runtime().RunString(ts.httpBin.Replacer.Replace(code))
}

func assertMetricEmitted(
	t *testing.T,
	metricName string, //nolint:unparam
	sampleContainers []metrics.SampleContainer,
	url string,
) {
	seenMetric := false

	for _, sampleContainer := range sampleContainers {
		for _, sample := range sampleContainer.GetSamples() {
			surl, ok := sample.Tags.Get("url")
			assert.True(t, ok)
			if surl == url {
				if sample.Metric.Name == metricName {
					seenMetric = true
				}
			}
		}
	}
	assert.True(t, seenMetric, "url %s didn't emit %s", url, metricName)
}

func assertResponse(t *testing.T, cb codeBlock, err error, val goja.Value, ts testState) {
	if isWindows && cb.windowsErr != "" && err != nil {
		err = errors.New(strings.ReplaceAll(err.Error(), cb.windowsErr, cb.err))
	}
	if cb.err == "" {
		assert.NoError(t, err)
	} else {
		require.Error(t, err)
		assert.Contains(t, err.Error(), cb.err)
	}
	if cb.val != nil {
		require.NotNil(t, val)
		assert.Equal(t, cb.val, val.Export())
	}
	if cb.asserts != nil {
		cb.asserts(t, ts.httpBin, ts.samples, err)
	}
}
