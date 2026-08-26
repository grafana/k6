package tcp

type socketState string

const (
	socketStateOpening      socketState = "opening"
	socketStateOpen         socketState = "open"
	socketStateDestroyed    socketState = "destroyed"
	socketStateDisconnected socketState = "disconnected"
)

func (s *socket) readyState() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return string(s.state)
}

func (s *socket) isReadable() bool {
	return s.isConnected()
}

func (s *socket) isConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.state == socketStateOpen
}
