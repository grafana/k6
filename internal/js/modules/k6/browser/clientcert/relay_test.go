package clientcert

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

const testRequest = "GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"

func TestRelay_BlockedUpstreamWrite(t *testing.T) {
	t.Parallel()

	client, browser := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = browser.Close()
	})
	upstream := newFakeConn()
	blocked := make(chan struct{})
	var blockedOnce sync.Once
	upstream.write = func([]byte) (int, error) {
		blockedOnce.Do(func() { close(blocked) })
		<-upstream.closed
		return 0, net.ErrClosed
	}
	upstream.read = func([]byte) (int, error) {
		<-blocked
		return 0, errors.New("remote error: tls: bad certificate")
	}

	done := startRelay(t.Context(), browser, browser, upstream, upstream, "http/1.1", defaultReplayLimit)
	require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))
	_, err := client.Write([]byte(testRequest))
	require.NoError(t, err)

	status, body := readResponse(t, client)
	_ = client.Close()
	requireReturns(t, done, 2*time.Second, "relay")
	assert.True(t, upstream.isClosed())
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Equal(t, "k6 client-certificate error: remote error: tls: bad certificate", body)
}

func TestRelay_UpstreamWriteFails(t *testing.T) {
	t.Parallel()

	client, browser := net.Pipe()
	t.Cleanup(func() {
		_ = client.Close()
		_ = browser.Close()
	})
	upstream := newFakeConn()
	upstream.write = func([]byte) (int, error) { return 0, errors.New("upstream write failed") }

	done := startRelay(t.Context(), browser, browser, upstream, upstream, "http/1.1", defaultReplayLimit)
	require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))
	_, err := client.Write([]byte(testRequest))
	require.NoError(t, err)

	status, body := readResponse(t, client)
	_ = client.Close()
	requireReturns(t, done, 2*time.Second, "relay")
	assert.True(t, upstream.isClosed())
	assert.Equal(t, http.StatusServiceUnavailable, status)
	assert.Equal(t, "k6 client-certificate error: upstream write failed", body)
}

func TestRelay_ConcurrentEOF(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		protocol    string
		upstreamErr error
	}{
		{name: "http1_upstream_eof", protocol: "http/1.1", upstreamErr: io.EOF},
		{name: "h2_upstream_eof", protocol: "h2", upstreamErr: io.EOF},
		{name: "http1_upstream_reset", protocol: "http/1.1", upstreamErr: errors.New("connection reset by peer")},
		{name: "h2_upstream_reset", protocol: "h2", upstreamErr: errors.New("connection reset by peer")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for i := range 100 {
				start := make(chan struct{})
				var written atomic.Int64
				browser := newFakeConn()
				browser.read = func([]byte) (int, error) {
					<-start
					return 0, io.EOF
				}
				browser.write = func(b []byte) (int, error) {
					written.Add(int64(len(b)))
					return len(b), nil
				}
				upstream := newFakeConn()
				upstream.read = func([]byte) (int, error) {
					<-start
					return 0, tt.upstreamErr
				}

				done := startRelay(t.Context(), browser, browser, upstream, upstream, tt.protocol, defaultReplayLimit)
				close(start)
				requireReturns(t, done, 2*time.Second, "relay")
				require.Zero(t, written.Load(), "iteration %d wrote to the browser", i)
				require.True(t, browser.isClosed(), "iteration %d", i)
				require.True(t, upstream.isClosed(), "iteration %d", i)
			}
		})
	}
}

func TestRelay_BrowserClosesFirst(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		upstreamBytes string
	}{
		{name: "before_first_upstream_byte"},
		{name: "after_first_upstream_byte", upstreamBytes: "response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, browser := tcpPair(t)
			upstream := newFakeConn()
			sent := false
			upstream.read = func(b []byte) (int, error) {
				if !sent && tt.upstreamBytes != "" {
					sent = true
					return copy(b, tt.upstreamBytes), nil
				}
				<-upstream.closedWrite
				return 0, io.EOF
			}

			done := startRelay(t.Context(), browser, browser, upstream, upstream, "http/1.1", defaultReplayLimit)
			require.NoError(t, client.SetDeadline(time.Now().Add(5*time.Second)))
			if tt.upstreamBytes != "" {
				assert.Equal(t, tt.upstreamBytes, string(readN(t, client, len(tt.upstreamBytes))))
			}
			require.NoError(t, client.CloseWrite())

			select {
			case <-upstream.closedWrite:
			case <-time.After(2 * time.Second):
				require.FailNow(t, "upstream was not half-closed")
			}
			requireReturns(t, done, 2*time.Second, "relay")
			assert.True(t, upstream.isClosed())
			_, err := browser.Write([]byte{0})
			assert.ErrorIs(t, err, net.ErrClosed)

			rest, err := io.ReadAll(client)
			require.NoError(t, err)
			assert.Empty(t, rest)
		})
	}

	t.Run("browser_error", func(t *testing.T) {
		t.Parallel()

		upstream := newFakeConn()
		reading := make(chan struct{})
		var readingOnce sync.Once
		upstream.read = func([]byte) (int, error) {
			readingOnce.Do(func() { close(reading) })
			<-upstream.closed
			return 0, net.ErrClosed
		}
		browser := newFakeConn()
		browser.read = func([]byte) (int, error) {
			<-reading
			return 0, errors.New("connection reset by peer")
		}

		done := startRelay(t.Context(), browser, browser, upstream, upstream, "http/1.1", defaultReplayLimit)
		requireReturns(t, done, 2*time.Second, "relay")
		assert.True(t, upstream.isClosed())
		assert.True(t, browser.isClosed())
	})
}

func TestRelay_ContextCancelled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	browser, upstream := newFakeConn(), newFakeConn()
	done := startRelay(ctx, browser, browser, upstream, upstream, "http/1.1", defaultReplayLimit)
	cancel()

	requireReturns(t, done, 2*time.Second, "relay")
	assert.True(t, browser.isClosed())
	assert.True(t, upstream.isClosed())
}

func TestRelayRecord_ReleasesReplayAfterFirstUpstreamByte(t *testing.T) {
	t.Parallel()

	r := &relayState{protocol: "h2", replayLimit: defaultReplayLimit}
	r.record([]byte("preface and settings"))
	require.NotEmpty(t, r.replay)

	r.firstByte.Store(true)
	r.record([]byte("next frame"))

	assert.Nil(t, r.replay, "a long-lived h2 connection must not keep the replay buffer")
	assert.False(t, r.replayOverflow)
}

func TestServe503H2_Terminates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		request bool
	}{
		{name: "client_keeps_connection_open", request: true},
		{name: "browser_closes_before_request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client, browser, browserRaw := tlsPipe(t, "h2")
			done := make(chan struct{})
			go func() {
				defer close(done)
				serve503H2(browser, browserRaw, nil, errors.New("upstream failed"))
			}()

			if !tt.request {
				require.NoError(t, client.Close())
				requireReturns(t, done, 3*time.Second, "serve503H2")
				return
			}

			var resp h2Response
			select {
			case resp = <-sendH2Request(t, client, nil):
			case <-time.After(5 * time.Second):
				require.FailNow(t, "no response")
			}

			requireReturns(t, done, 3*time.Second, "serve503H2")
			require.NoError(t, resp.err)
			assert.Equal(t, "503", resp.status)
			assert.True(t, strings.HasPrefix(resp.contentType, "text/html"), resp.contentType)
			assert.Equal(t, "k6 client-certificate error: upstream failed", resp.body)
		})
	}
}

func TestClientCertificateErrorBody(t *testing.T) {
	t.Parallel()

	got := clientCertificateErrorBody(errors.New("<script>\"x\"\nsecond line"))
	assert.Equal(t, "k6 client-certificate error: &lt;script&gt;&#34;x&#34; <br>second line", got)
}

// readResponse reads an HTTP/1.1 response from conn and returns its status code and body.
func readResponse(t *testing.T, conn net.Conn) (int, string) {
	t.Helper()
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(b)
}

type h2Response struct {
	status      string
	contentType string
	body        string
	err         error
}

// sendH2Request sends a request on stream 1 of conn, a GET without body or a POST with it,
// and delivers the response on the returned channel. Unlike Go's HTTP/2 client, it keeps
// reading conn afterwards and never closes it, even after a GOAWAY.
func sendH2Request(t *testing.T, conn net.Conn, body []byte) <-chan h2Response {
	t.Helper()
	fr := http2.NewFramer(conn, conn)
	fr.ReadMetaHeaders = hpack.NewDecoder(4096, nil)

	result := make(chan h2Response, 1)
	go func() {
		var resp h2Response
		for {
			f, err := fr.ReadFrame()
			if err != nil {
				resp.err = err
				result <- resp
				return
			}
			switch f := f.(type) {
			case *http2.MetaHeadersFrame:
				resp.status = f.PseudoValue("status")
				for _, hf := range f.RegularFields() {
					if hf.Name == "content-type" {
						resp.contentType = hf.Value
					}
				}
			case *http2.DataFrame:
				resp.body += string(f.Data())
				if f.StreamEnded() {
					result <- resp
					for {
						if _, err := fr.ReadFrame(); err != nil {
							return
						}
					}
				}
			}
		}
	}()

	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	var block bytes.Buffer
	enc := hpack.NewEncoder(&block)
	for _, hf := range []hpack.HeaderField{
		{Name: ":method", Value: method},
		{Name: ":scheme", Value: "https"},
		{Name: ":authority", Value: "example.com"},
		{Name: ":path", Value: "/"},
	} {
		require.NoError(t, enc.WriteField(hf))
	}
	_, err := io.WriteString(conn, http2.ClientPreface)
	require.NoError(t, err)
	require.NoError(t, fr.WriteSettings())
	require.NoError(t, fr.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      1,
		BlockFragment: block.Bytes(),
		EndStream:     body == nil,
		EndHeaders:    true,
	}))
	const maxFrameSize = 16 << 10
	for len(body) > 0 {
		chunk := body[:min(len(body), maxFrameSize)]
		body = body[len(chunk):]
		require.NoError(t, fr.WriteData(1, len(body) == 0, chunk))
	}
	return result
}
