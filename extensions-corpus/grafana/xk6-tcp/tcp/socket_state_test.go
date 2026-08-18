package tcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSocketStateTransitions(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	tests := []struct {
		name          string
		initialState  socketState
		expectedState socketState
		setupConn     bool
		action        func(*socket)
	}{
		{
			name:          "initial state is disconnected",
			initialState:  socketStateDisconnected,
			expectedState: socketStateDisconnected,
			setupConn:     false,
			action:        func(_ *socket) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newSocket(mod.log, mod.vu, mod.metrics)
			s.state = tt.initialState
			s.this = mod.vu.Runtime().NewObject()

			// Set up cancellation
			s.cancel = func() {}
			s.vu = mod.vu

			tt.action(s)

			require.Equal(t, tt.expectedState, s.state)
		})
	}
}

func TestSocketReadyState(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	tests := []struct {
		name     string
		state    socketState
		expected string
	}{
		{"disconnected", socketStateDisconnected, "disconnected"},
		{"opening", socketStateOpening, "opening"},
		{"open", socketStateOpen, "open"},
		{"destroyed", socketStateDestroyed, "destroyed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newSocket(mod.log, mod.vu, mod.metrics)
			s.state = tt.state

			result := s.readyState()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSocketIsConnected(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	tests := []struct {
		name     string
		state    socketState
		expected bool
	}{
		{"disconnected is not connected", socketStateDisconnected, false},
		{"opening is not connected", socketStateOpening, false},
		{"open is connected", socketStateOpen, true},
		{"destroyed is not connected", socketStateDestroyed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newSocket(mod.log, mod.vu, mod.metrics)
			s.state = tt.state

			result := s.isConnected()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSocketIsReadable(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	tests := []struct {
		name     string
		state    socketState
		expected bool
	}{
		{"disconnected is not readable", socketStateDisconnected, false},
		{"opening is not readable", socketStateOpening, false},
		{"open is readable", socketStateOpen, true},
		{"destroyed is not readable", socketStateDestroyed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := newSocket(mod.log, mod.vu, mod.metrics)
			s.state = tt.state

			result := s.isReadable()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSocketBytesCounters(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	s := newSocket(mod.log, mod.vu, mod.metrics)

	// Initial values
	require.Equal(t, int64(0), s.bytesWritten())
	require.Equal(t, int64(0), s.bytesRead())

	// Simulate some writes
	s.totalWritten = 100
	require.Equal(t, int64(100), s.bytesWritten())

	// Simulate some reads
	s.totalRead = 200
	require.Equal(t, int64(200), s.bytesRead())
}

func TestMetricsCreation(t *testing.T) {
	t.Parallel()

	mod := newTestModuleInstance(t)

	m := newTCPMetrics(mod.vu)

	require.NotNil(t, m)
	require.NotNil(t, m.tcpConnecting)
	require.NotNil(t, m.tcpResolving)
	require.NotNil(t, m.tcpDuration)
	require.NotNil(t, m.tcpSockets)
	require.NotNil(t, m.tcpReads)
	require.NotNil(t, m.tcpWrites)
	require.NotNil(t, m.tcpErrors)
	require.NotNil(t, m.tcpTimeouts)

	require.Equal(t, tcpConnecting, m.tcpConnecting.Name())
	require.Equal(t, tcpResolving, m.tcpResolving.Name())
	require.Equal(t, tcpDuration, m.tcpDuration.Name())
	require.Equal(t, tcpSockets, m.tcpSockets.Name())
	require.Equal(t, tcpReads, m.tcpReads.Name())
	require.Equal(t, tcpWrites, m.tcpWrites.Name())
	require.Equal(t, tcpErrors, m.tcpErrors.Name())
	require.Equal(t, tcpTimeouts, m.tcpTimeouts.Name())
}
