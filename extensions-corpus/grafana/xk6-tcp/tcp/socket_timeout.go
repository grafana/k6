package tcp

import (
	"time"

	"github.com/grafana/sobek"
)

// setTimeout sets the socket timeout for inactivity.
// Sets the socket to timeout after timeout milliseconds of inactivity on the socket.
// When an idle timeout is triggered, the socket will receive a 'timeout' event but the
// connection will not be severed. The user must manually call destroy() to end the connection.
// If timeout is 0, then the existing idle timeout is disabled.
func (s *socket) setTimeout(timeoutMs int64) (*sobek.Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if timeoutMs <= 0 {
		// Disable timeout
		s.timeout = 0
		s.log.Debug("Timeout disabled")
	} else {
		s.timeout = time.Duration(timeoutMs) * time.Millisecond
		s.log.WithField("timeout", s.timeout).Debug("Timeout set")
	}

	// If we have an active connection, update its deadline.
	if s.conn != nil {
		var deadline time.Time
		if s.timeout > 0 {
			deadline = time.Now().Add(s.timeout)
		}

		if err := s.conn.SetReadDeadline(deadline); err != nil {
			return s.this, err
		}
	}

	return s.this, nil
}
