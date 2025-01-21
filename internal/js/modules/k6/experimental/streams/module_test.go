package streams

import (
	"bytes"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/js/modulestest"
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
