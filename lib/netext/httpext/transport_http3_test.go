package httpext

import (
	"crypto/tls"
	"net"
	"net/http/httptrace"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoneWithFallbackHTTPTraceFired(t *testing.T) {
	t.Parallel()

	// Simulate httptrace hooks having fired (gotConn != 0)
	tracer := &Tracer{}
	tracer.GetConn("example.com:443")
	tracer.ConnectStart("tcp", "1.2.3.4:443")
	tracer.ConnectDone("tcp", "1.2.3.4:443", nil)
	tracer.TLSHandshakeStart()
	tracer.TLSHandshakeDone(tls.ConnectionState{}, nil)
	tracer.GotConn(httptrace.GotConnInfo{
		Conn: &net.TCPConn{},
	})
	tracer.WroteRequest(httptrace.WroteRequestInfo{})
	time.Sleep(time.Millisecond)
	tracer.GotFirstResponseByte()

	now := time.Now().UnixNano()
	trail := tracer.DoneWithFallback(now-int64(10*time.Millisecond), now-int64(5*time.Millisecond))

	// When httptrace hooks fired, DoneWithFallback should delegate to Done()
	// and produce normal timing metrics
	require.NotNil(t, trail)
	assert.True(t, trail.Duration > 0, "Duration should be > 0")
}

func TestDoneWithFallbackNoHTTPTrace(t *testing.T) {
	t.Parallel()

	// Simulate HTTP/3 where httptrace hooks don't fire
	tracer := &Tracer{}

	rtStart := time.Now().UnixNano()
	time.Sleep(5 * time.Millisecond)
	rtEnd := time.Now().UnixNano()
	time.Sleep(2 * time.Millisecond) // simulate body reading

	trail := tracer.DoneWithFallback(rtStart, rtEnd)

	require.NotNil(t, trail)
	assert.True(t, trail.Waiting > 0, "Waiting should be > 0 for HTTP/3 fallback")
	assert.True(t, trail.Receiving > 0, "Receiving should be > 0 for HTTP/3 fallback")
	assert.True(t, trail.Duration > 0, "Duration should be > 0")
	assert.Equal(t, trail.Duration, trail.Waiting+trail.Receiving)
	// No connection-level metrics in HTTP/3 fallback
	assert.Equal(t, time.Duration(0), trail.Blocked)
	assert.Equal(t, time.Duration(0), trail.Connecting)
	assert.Equal(t, time.Duration(0), trail.TLSHandshaking)
	assert.Equal(t, time.Duration(0), trail.Sending)
}
