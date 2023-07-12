package tracing

import (
	"math/rand"
	"net/http"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modulestest"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/testutils/httpmultibin"
	"go.k6.io/k6/metrics"
)

// traceParentHeaderName is the normalized trace header name.
// Although the traceparent header is case insensitive, the
// Go http.Header sets it capitalized.
const traceparentHeaderName string = "Traceparent"

// testTraceID is a valid trace ID used in tests.
const testTraceID string = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00"

// testMetadataTraceIDRandomness is the randomness part of the test trace ID encoded
// as hexadecimal.
//
// It is used to test the randomness of the trace ID. As we use a fixed time
// as the test setup's randomness source, we can can assert that the randomness
// part of the trace ID is the same.
//
// Although the randomness part doesn't have a fixed size. We can assume that
// it will be 4 bytes, as the Go time.Time is encoded as nanoseconds, and
// the trace ID is encoded as 8 bytes.
const testMetadataTraceIDRandomness string = "0194fdc2"

// testTracePrefix is the prefix of the test trace ID encoded as hexadecimal.
// It is equivalent to the first 2 bytes of the trace ID, which is always
// set to `k6Prefix` in the context of tests.
const testTracePrefix string = "dc07"

// testTraceCode is the code of the test trace ID encoded as hexadecimal.
// It is equivalent to the third byte of the trace ID, which is always set to `k6CloudCode`
// in the context of tests.
const testTraceCode string = "18"

func TestClientInstrumentArguments(t *testing.T) {
	t.Parallel()

	t.Run("no args should fail", func(t *testing.T) {
		t.Parallel()

		testCase := newTestCase(t)

		_, err := testCase.client.instrumentArguments(testCase.traceContextHeader)
		require.Error(t, err)
	})

	t.Run("1 arg should initialize params successfully", func(t *testing.T) {
		t.Parallel()

		testCase := newTestCase(t)
		rt := testCase.testSetup.VU.Runtime()

		gotArgs, gotErr := testCase.client.instrumentArguments(testCase.traceContextHeader, goja.Null())

		assert.NoError(t, gotErr)
		assert.Len(t, gotArgs, 2)
		assert.Equal(t, goja.Null(), gotArgs[0])

		gotParams := gotArgs[1].ToObject(rt)
		assert.NotNil(t, gotParams)
		gotHeaders := gotParams.Get("headers").ToObject(rt)
		assert.NotNil(t, gotHeaders)
		gotTraceParent := gotHeaders.Get(traceparentHeaderName)
		assert.NotNil(t, gotTraceParent)
		assert.Equal(t, testTraceID, gotTraceParent.String())
	})

	t.Run("2 args with null params should initialize it", func(t *testing.T) {
		t.Parallel()

		testCase := newTestCase(t)
		rt := testCase.testSetup.VU.Runtime()

		gotArgs, gotErr := testCase.client.instrumentArguments(testCase.traceContextHeader, goja.Null(), goja.Null())

		assert.NoError(t, gotErr)
		assert.Len(t, gotArgs, 2)
		assert.Equal(t, goja.Null(), gotArgs[0])

		gotParams := gotArgs[1].ToObject(rt)
		assert.NotNil(t, gotParams)
		gotHeaders := gotParams.Get("headers").ToObject(rt)
		assert.NotNil(t, gotHeaders)
		gotTraceParent := gotHeaders.Get(traceparentHeaderName)
		assert.NotNil(t, gotTraceParent)
		assert.Equal(t, testTraceID, gotTraceParent.String())
	})

	t.Run("2 args with undefined params should initialize it", func(t *testing.T) {
		t.Parallel()

		testCase := newTestCase(t)
		rt := testCase.testSetup.VU.Runtime()

		gotArgs, gotErr := testCase.client.instrumentArguments(testCase.traceContextHeader, goja.Null(), goja.Undefined())

		assert.NoError(t, gotErr)
		assert.Len(t, gotArgs, 2)
		assert.Equal(t, goja.Null(), gotArgs[0])

		gotParams := gotArgs[1].ToObject(rt)
		assert.NotNil(t, gotParams)
		gotHeaders := gotParams.Get("headers").ToObject(rt)
		assert.NotNil(t, gotHeaders)
		gotTraceParent := gotHeaders.Get(traceparentHeaderName)
		assert.NotNil(t, gotTraceParent)
		assert.Equal(t, testTraceID, gotTraceParent.String())
	})

	t.Run("2 args with predefined params and headers updates them successfully", func(t *testing.T) {
		t.Parallel()

		testCase := newTestCase(t)
		rt := testCase.testSetup.VU.Runtime()

		wantHeaders := rt.NewObject()
		require.NoError(t, wantHeaders.Set("X-Test-Header", "testvalue"))
		wantParams := rt.NewObject()
		require.NoError(t, wantParams.Set("headers", wantHeaders))

		gotArgs, gotErr := testCase.client.instrumentArguments(testCase.traceContextHeader, goja.Null(), wantParams)

		assert.NoError(t, gotErr)
		assert.Len(t, gotArgs, 2)
		assert.Equal(t, goja.Null(), gotArgs[0])
		assert.Equal(t, wantParams, gotArgs[1])

		gotHeaders := gotArgs[1].ToObject(rt).Get("headers").ToObject(rt)

		gotTraceParent := gotHeaders.Get(traceparentHeaderName)
		assert.NotNil(t, gotTraceParent)
		assert.Equal(t, testTraceID, gotTraceParent.String())

		gotTestHeader := gotHeaders.Get("X-Test-Header")
		assert.NotNil(t, gotTestHeader)
		assert.Equal(t, "testvalue", gotTestHeader.String())
	})

	t.Run("2 args with predefined params and no headers sets and updates them successfully", func(t *testing.T) {
		t.Parallel()

		testCase := newTestCase(t)
		rt := testCase.testSetup.VU.Runtime()
		wantParams := rt.NewObject()

		gotArgs, gotErr := testCase.client.instrumentArguments(testCase.traceContextHeader, goja.Null(), wantParams)

		assert.NoError(t, gotErr)
		assert.Len(t, gotArgs, 2)
		assert.Equal(t, goja.Null(), gotArgs[0])
		assert.Equal(t, wantParams, gotArgs[1])

		gotHeaders := gotArgs[1].ToObject(rt).Get("headers").ToObject(rt)

		gotTraceParent := gotHeaders.Get(traceparentHeaderName)
		assert.NotNil(t, gotTraceParent)
		assert.Equal(t, testTraceID, gotTraceParent.String())
	})
}

func TestClientInstrumentedCall(t *testing.T) {
	t.Parallel()

	testCase := newTestCase(t)
	testCase.testSetup.MoveToVUContext(&lib.State{
		Tags: lib.NewVUStateTags(&metrics.TagSet{}),
	})
	testCase.client.propagator = NewW3CPropagator(NewAlwaysOnSampler())

	callFn := func(args ...goja.Value) error {
		gotMetadataTraceID, gotTraceIDKey := testCase.client.vu.State().Tags.GetCurrentValues().Metadata["trace_id"]
		assert.True(t, gotTraceIDKey)
		assert.NotEmpty(t, gotMetadataTraceID)
		assert.Equal(t, testTracePrefix, gotMetadataTraceID[:len(testTracePrefix)])
		assert.Equal(t, testTraceCode, gotMetadataTraceID[len(testTracePrefix):len(testTracePrefix)+len(testTraceCode)])
		assert.Equal(t, testMetadataTraceIDRandomness, gotMetadataTraceID[len(gotMetadataTraceID)-len(testMetadataTraceIDRandomness):])

		return nil
	}

	// Assert there is no trace_id key in vu metadata before using intrumentedCall
	_, hasTraceIDKey := testCase.client.vu.State().Tags.GetCurrentValues().Metadata["trace_id"]
	assert.False(t, hasTraceIDKey)

	// The callFn will assert that the trace_id key is present in vu metadata
	// before returning
	_ = testCase.client.instrumentedCall(callFn)

	// Assert there is no trace_id key in vu metadata after using intrumentedCall
	_, hasTraceIDKey = testCase.client.vu.State().Tags.GetCurrentValues().Metadata["trace_id"]
	assert.False(t, hasTraceIDKey)
}

// This test ensures that the trace_id is added to the vu metadata when
// and instrumented request is called; and that we can find it in the
// produced samples.
//
// It also ensures that the trace_id is removed from the vu metadata
// after the request is done.
func TestCallingInstrumentedRequestEmitsTraceIdMetadata(t *testing.T) {
	t.Parallel()

	testCase := newTestSetup(t)
	rt := testCase.TestRuntime.VU.Runtime()

	// Making sure the instrumentHTTP is called in the init context
	// before the test.
	_, err := rt.RunString(`
		let http = require('k6/http')
		instrumentHTTP({propagator: 'w3c'})
	`)
	require.NoError(t, err)

	// Move to VU context. Setup in way that the produced samples are
	// written to a channel we own.
	samples := make(chan metrics.SampleContainer, 1000)
	httpBin := httpmultibin.NewHTTPMultiBin(t)
	testCase.TestRuntime.MoveToVUContext(&lib.State{
		BuiltinMetrics: metrics.RegisterBuiltinMetrics(testCase.TestRuntime.VU.InitEnvField.Registry),
		Tags:           lib.NewVUStateTags(testCase.TestRuntime.VU.InitEnvField.Registry.RootTagSet()),
		Transport:      httpBin.HTTPTransport,
		BufferPool:     lib.NewBufferPool(),
		Samples:        samples,
		Options:        lib.Options{SystemTags: &metrics.DefaultSystemTagSet},
	})

	// Inject a function in the JS runtime to assert the trace_id key
	// is present in the vu metadata.
	err = rt.Set("assert_has_trace_id_metadata", func(expected bool, expectedTraceID string) {
		gotTraceID, hasTraceID := testCase.TestRuntime.VU.State().Tags.GetCurrentValues().Metadata["trace_id"]
		require.Equal(t, expected, hasTraceID)

		if expectedTraceID != "" {
			assert.Equal(t, testTracePrefix, gotTraceID[:len(testTracePrefix)])
			assert.Equal(t, testTraceCode, gotTraceID[len(testTracePrefix):len(testTracePrefix)+len(testTraceCode)])
			assert.Equal(t, testMetadataTraceIDRandomness, gotTraceID[len(gotTraceID)-len(testMetadataTraceIDRandomness):])
		}
	})
	require.NoError(t, err)

	// Assert there is no trace_id key in vu metadata before calling an instrumented
	// function, and that it's cleaned up after the call.
	t.Cleanup(testCase.TestRuntime.EventLoop.WaitOnRegistered)
	_, err = testCase.TestRuntime.RunOnEventLoop(httpBin.Replacer.Replace(`
		assert_has_trace_id_metadata(false)
		http.request("GET", "HTTPBIN_URL")
		assert_has_trace_id_metadata(false)
	`))
	require.NoError(t, err)
	close(samples)

	var sampleRead bool
	for sampleContainer := range samples {
		for _, sample := range sampleContainer.GetSamples() {
			require.NotEmpty(t, sample.Metadata["trace_id"])
			sampleRead = true
		}
	}
	require.True(t, sampleRead)
}

type tracingClientTestCase struct {
	t                  *testing.T
	testSetup          *modulestest.Runtime
	client             Client
	traceContextHeader http.Header
}

func newTestCase(t *testing.T) *tracingClientTestCase {
	testSetup := modulestest.NewRuntime(t)
	// Here we provide the client with a fixed seed to ensure that the
	// generated trace IDs random part is deterministic.
	client := Client{vu: testSetup.VU, randSource: rand.New(rand.NewSource(0))} //nolint:gosec
	traceContextHeader := http.Header{}
	traceContextHeader.Add(traceparentHeaderName, testTraceID)

	return &tracingClientTestCase{
		t:                  t,
		testSetup:          testSetup,
		client:             client,
		traceContextHeader: traceContextHeader,
	}
}
