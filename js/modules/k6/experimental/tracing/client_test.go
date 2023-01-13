package tracing

import (
	"net/http"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modulestest"
)

// traceParentHeaderName is the normalized trace header name.
// Although the traceparent header is case insensitive, the
// Go http.Header sets it capitalized.
const traceparentHeaderName string = "Traceparent"

// testTraceID is a valid trace ID used in tests.
const testTraceID string = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00"

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

type tracingClientTestCase struct {
	t                  *testing.T
	testSetup          *modulestest.Runtime
	client             Client
	traceContextHeader http.Header
}

func newTestCase(t *testing.T) *tracingClientTestCase {
	testSetup := modulestest.NewRuntime(t)
	client := Client{vu: testSetup.VU}
	traceContextHeader := http.Header{}
	traceContextHeader.Add(traceparentHeaderName, testTraceID)

	return &tracingClientTestCase{
		t:                  t,
		testSetup:          testSetup,
		client:             client,
		traceContextHeader: traceContextHeader,
	}
}
