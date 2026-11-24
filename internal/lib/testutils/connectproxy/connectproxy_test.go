package connectproxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	t.Parallel()

	auth := Options{Username: "user", Password: "pass"}
	tests := []struct {
		name       string
		opts       Options
		userinfo   *url.Userinfo
		wantStatus int
	}{
		{name: "no_auth", wantStatus: http.StatusOK},
		{name: "no_credentials", opts: auth, wantStatus: http.StatusProxyAuthRequired},
		{name: "wrong_credentials", opts: auth, userinfo: url.UserPassword("user", "wrong"), wantStatus: http.StatusProxyAuthRequired},
		{name: "right_credentials", opts: auth, userinfo: url.UserPassword("user", "pass"), wantStatus: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = io.WriteString(w, "hello")
			}))
			t.Cleanup(target.Close)
			proxy := New(t, tt.opts)
			proxyURL, err := url.Parse(proxy.URL())
			require.NoError(t, err)
			proxyURL.User = tt.userinfo

			var connectResp *http.Response
			tr := target.Client().Transport.(*http.Transport)
			tr.Proxy = http.ProxyURL(proxyURL)
			tr.OnProxyConnectResponse = func(_ context.Context, _ *url.URL, _ *http.Request, resp *http.Response) error {
				connectResp = resp
				return nil
			}
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, target.URL, nil)
			require.NoError(t, err)
			var body []byte
			resp, err := target.Client().Do(req)
			if err == nil {
				body, err = io.ReadAll(resp.Body)
				_ = resp.Body.Close()
			}

			require.NotNil(t, connectResp)
			assert.Equal(t, tt.wantStatus, connectResp.StatusCode)
			if tt.wantStatus != http.StatusOK {
				require.Error(t, err)
				assert.Equal(t, `Basic realm="test"`, connectResp.Header.Get("Proxy-Authenticate"))
				assert.Empty(t, proxy.Targets())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "hello", string(body))
			assert.Equal(t, []string{target.Listener.Addr().String()}, proxy.Targets())
		})
	}
}

func TestServer_HalfClose(t *testing.T) {
	t.Parallel()

	// The target echoes everything it reads once the client half-closes.
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		if conn, err := ln.Accept(); err == nil {
			data, _ := io.ReadAll(conn)
			_, _ = conn.Write(data)
			_ = conn.Close()
		}
	}()
	proxy := New(t, Options{})

	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", strings.TrimPrefix(proxy.URL(), "http://"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	addr := ln.Addr().String()
	_, err = io.WriteString(conn, "CONNECT "+addr+" HTTP/1.1\r\nHost: "+addr+"\r\n\r\n")
	require.NoError(t, err)
	const established = "HTTP/1.1 200 Connection established\r\n\r\n"
	reply := make([]byte, len(established))
	_, err = io.ReadFull(conn, reply)
	require.NoError(t, err)
	require.Equal(t, established, string(reply))

	_, err = io.WriteString(conn, "ping")
	require.NoError(t, err)
	require.NoError(t, conn.(*net.TCPConn).CloseWrite())
	got, err := io.ReadAll(conn)
	require.NoError(t, err)
	assert.Equal(t, "ping", string(got))
}
