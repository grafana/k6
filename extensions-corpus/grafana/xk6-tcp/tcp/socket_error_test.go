package tcp

import (
	"errors"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
)

func TestTCPError(t *testing.T) {
	t.Parallel()

	err := errors.New("connection refused") //nolint:err113
	tcpErr := newTCPError(err, "connect")

	require.Equal(t, "TCPError", tcpErr.Name)
	require.Equal(t, "connect", tcpErr.Method)
	require.Equal(t, "connection refused", tcpErr.Message)
	require.Contains(t, tcpErr.Error(), "TCP error during connect")
	require.Contains(t, tcpErr.Error(), "connection refused")
}

func TestSocketOnInvalidEvent(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	s := newSocket(mod.log, mod.vu, mod.metrics)

	handler := mod.vu.Runtime().ToValue(func() {})
	callable, _ := sobek.AssertFunction(handler)

	// Should not panic, just log warning
	s.on("invalid_event", callable)

	// Verify handler was not stored
	_, ok := s.handlers.Load("invalid_event")
	require.False(t, ok)
}

func TestSocketOnValidEvents(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	validEvents := []string{"connect", "data", "close", "error", "timeout"}

	for _, event := range validEvents {
		t.Run(event, func(t *testing.T) {
			t.Parallel()

			s := newSocket(mod.log, mod.vu, mod.metrics)

			handler := mod.vu.Runtime().ToValue(func() {})
			callable, _ := sobek.AssertFunction(handler)

			s.on(event, callable)

			// Verify handler was stored
			_, ok := s.handlers.Load(event)
			require.True(t, ok)
		})
	}
}

func TestSocketOnEventOverride(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	s := newSocket(mod.log, mod.vu, mod.metrics)

	handler1 := mod.vu.Runtime().ToValue(func() int { return 1 })
	callable1, _ := sobek.AssertFunction(handler1)

	handler2 := mod.vu.Runtime().ToValue(func() int { return 2 })
	callable2, _ := sobek.AssertFunction(handler2)

	s.on("connect", callable1)

	// Override with second handler - should log warning
	s.on("connect", callable2)

	// Verify second handler is stored
	stored, ok := s.handlers.Load("connect")
	require.True(t, ok)
	require.NotNil(t, stored)
}

func TestStringOrArrayBufferWithString(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	input := mod.vu.Runtime().ToValue("Hello, World!")
	data, err := stringOrArrayBuffer(input, "", mod.vu.Runtime())

	require.NoError(t, err)
	require.Equal(t, []byte("Hello, World!"), data)
}

func TestStringOrArrayBufferWithBytes(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	expected := []byte{0x01, 0x02, 0x03, 0x04}
	input := mod.vu.Runtime().ToValue(expected)
	data, err := stringOrArrayBuffer(input, "", mod.vu.Runtime())

	require.NoError(t, err)
	require.Equal(t, expected, data)
}

func TestStringOrArrayBufferWithInvalidType(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	// Test with number
	input := mod.vu.Runtime().ToValue(123)
	_, err := stringOrArrayBuffer(input, "", mod.vu.Runtime())

	require.Error(t, err)
	require.Contains(t, err.Error(), "String or ArrayBuffer expected")
}

func TestConnectOptionsAddress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     connectOptions
		expected string
	}{
		{
			name:     "localhost with standard port",
			opts:     connectOptions{Host: "localhost", Port: 8080},
			expected: "localhost:8080",
		},
		{
			name:     "IP address with port",
			opts:     connectOptions{Host: "192.168.1.1", Port: 443},
			expected: "192.168.1.1:443",
		},
		{
			name:     "domain with custom port",
			opts:     connectOptions{Host: "example.com", Port: 9999},
			expected: "example.com:9999",
		},
		{
			name:     "IPv6 address",
			opts:     connectOptions{Host: "::1", Port: 80},
			expected: "[::1]:80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.opts.address()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestWriteOptionsDefaults(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	s := newSocket(mod.log, mod.vu, mod.metrics)

	data := mod.vu.Runtime().ToValue("test")
	result, opts, err := s.writePrepare(data, nil)

	require.NoError(t, err)
	require.Equal(t, []byte("test"), result)
	require.NotNil(t, opts)
	require.Empty(t, opts.Encoding)
	require.Nil(t, opts.Tags)
}

func TestWritePrepareWithOptions(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	s := newSocket(mod.log, mod.vu, mod.metrics)

	data := mod.vu.Runtime().ToValue("test")
	opts := &writeOptions{
		Encoding: "utf8",
		Tags:     map[string]string{"type": "message"},
	}

	result, returnedOpts, err := s.writePrepare(data, opts)

	require.NoError(t, err)
	require.Equal(t, []byte("test"), result)
	require.Equal(t, "utf8", returnedOpts.Encoding)
	require.Equal(t, "message", returnedOpts.Tags["type"])
}

func TestConnectPrepareWithPort(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	s := newSocket(mod.log, mod.vu, mod.metrics)

	port := mod.vu.Runtime().ToValue(8080)
	host := mod.vu.Runtime().ToValue("example.com")

	err := s.connectPrepare(port, host)

	require.NoError(t, err)
	require.NotNil(t, s.connectOpts)
	require.Equal(t, 8080, s.connectOpts.Port)
	require.Equal(t, "example.com", s.connectOpts.Host)
}

func TestConnectPrepareWithDefaultHost(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	s := newSocket(mod.log, mod.vu, mod.metrics)

	port := mod.vu.Runtime().ToValue(8080)

	err := s.connectPrepare(port, sobek.Undefined())

	require.NoError(t, err)
	require.NotNil(t, s.connectOpts)
	require.Equal(t, 8080, s.connectOpts.Port)
	require.Equal(t, "localhost", s.connectOpts.Host)
}

func TestConnectPrepareWithInvalidType(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	s := newSocket(mod.log, mod.vu, mod.metrics)

	// Pass invalid type (array instead of int or object)
	invalid := mod.vu.Runtime().ToValue([]string{"invalid"})

	err := s.connectPrepare(invalid, sobek.Undefined())

	require.Error(t, err)
	require.Contains(t, err.Error(), "expected integer or object")
}

func TestWriteNoActiveConnection(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	s := newSocket(mod.log, mod.vu, mod.metrics)
	// Don't set up a connection

	err := s.writeExecute([]byte("test"), &writeOptions{})

	require.Error(t, err)
	require.ErrorIs(t, err, errNoActiveConnection)
}
