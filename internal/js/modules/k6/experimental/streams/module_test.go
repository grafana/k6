package streams

import (
	"bytes"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/js/modulestest"
)

func TestNewReadableStreamFromReader(t *testing.T) {
	t.Parallel()

	// The value to be streamed.
	exp := "Hello, World!"

	// We initialize the runtime, with the ReadableStream(rs) accessible in JS.
	r := modulestest.NewRuntime(t)
	rs := NewReadableStreamFromReader(r.VU, bytes.NewReader([]byte(exp)))
	require.NoError(t, r.VU.Runtime().Set("rs", rs))

	// Then, we run some JS code that reads from the ReadableStream(rs).
	var ret sobek.Value
	err := r.EventLoop.Start(func() (err error) {
		ret, err = r.VU.Runtime().RunString(`(async () => {
  const reader = rs.getReader();
  const {value} = await reader.read();
  return value;
})()`)
		return err
	})
	assert.NoError(t, err)

	// Finally, we expect the returned promise to resolve
	// to the expected value (the one we streamed).
	p, ok := ret.Export().(*sobek.Promise)
	require.True(t, ok)
	assert.Equal(t, exp, p.Result().String())
}

func TestNewReadableStreamFromReaderUsesRuntimeIntrinsics(t *testing.T) {
	t.Parallel()

	r := newStreamsRuntime(t)
	rs := NewReadableStreamFromReader(r.VU, bytes.NewReader(nil))
	require.NoError(t, r.VU.Runtime().Set("rs", rs))

	value, err := r.RunOnEventLoop(`
JSON.stringify([
  rs instanceof ReadableStream,
  Object.getPrototypeOf(rs) === ReadableStream.prototype,
  Object.getPrototypeOf(rs.getReader()) === ReadableStreamDefaultReader.prototype,
]);
`)
	require.NoError(t, err)
	require.Equal(t, `[true,true,true]`, value.String())
}

func TestReadableStreamModuleReusesPreexistingRuntimeIntrinsics(t *testing.T) {
	t.Parallel()

	r := modulestest.NewRuntime(t)
	rs := NewReadableStreamFromReader(r.VU, bytes.NewReader(nil))
	m := New().NewModuleInstance(r.VU)
	for name, value := range m.Exports().Named {
		require.NoError(t, r.VU.Runtime().Set(name, value))
	}
	require.NoError(t, r.VU.Runtime().Set("rs", rs))

	value, err := r.RunOnEventLoop(`
JSON.stringify([
  rs instanceof ReadableStream,
  rs.constructor === ReadableStream,
  Object.getPrototypeOf(rs.getReader()) === ReadableStreamDefaultReader.prototype,
]);
`)
	require.NoError(t, err)
	require.Equal(t, `[true,true,true]`, value.String())
}
