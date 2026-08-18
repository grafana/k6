package tcp

import (
	"errors"
	"io"
	"net"
	"sync/atomic"
	"time"
)

func (s *socket) read() {
	defer s.destroy() // Cleanup when read loop exits

	s.log.Debug("Starting read loop")

	for s.isReadable() {
		// copy conn and timeout under lock to avoid holding mutex during blocking Read
		s.mu.RLock()
		conn := s.conn
		timeout := s.timeout
		s.mu.RUnlock()

		if conn == nil {
			break
		}

		if !s.readLoopStep(conn, timeout) {
			break
		}
	}
}

func (s *socket) readLoopStep(conn net.Conn, timeout time.Duration) bool {
	// Set read deadline if timeout is configured
	if timeout > 0 {
		deadline := time.Now().Add(timeout)
		if err := conn.SetReadDeadline(deadline); err != nil {
			s.log.Error("Failed to set read deadline", "error", err)
		}
	}

	// Read into reusable buffer (zero allocations)
	n, err := conn.Read(s.readBuf[:])
	if n > 0 {
		s.addCounterMetrics(s.metrics.tcpReads, s.currentTags())

		atomic.AddInt64(&s.totalRead, int64(n))

		// Copy the buffer because the read buffer is reused by the next read.
		s.fire("data", append([]byte(nil), s.readBuf[:n]...))
		s.log.Debug("Read from TCP connection", "bytes", n)
	}

	// is closed by other goroutine
	if !s.isReadable() {
		return false
	}

	if err == nil {
		return true
	}

	// handle EOF separately to allow clean socket closure
	if errors.Is(err, io.EOF) {
		return false
	}

	// Check if this is a timeout error
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		s.log.Debug("Socket timeout occurred")
		// Fire timeout event but don't close the connection
		s.fire("timeout")
		// Continue reading after timeout
		return true
	}

	e := s.handleError(err, "read", s.currentTags())
	if e != nil {
		s.log.Error("error in error handler", "error", e)
	}

	return false
}
