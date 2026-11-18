package common

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/clientcert"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
	k6netext "go.k6.io/k6/v2/lib/netext"
	k6types "go.k6.io/k6/v2/lib/types"
)

func TestClientCertificateDirectDialer_HalfClose(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		dial func(t *testing.T, direct clientcert.DialFunc) clientcert.DialFunc
	}{
		{
			name: "direct",
			dial: func(_ *testing.T, direct clientcert.DialFunc) clientcert.DialFunc { return direct },
		},
		{
			name: "through_connect_proxy",
			dial: func(t *testing.T, direct clientcert.DialFunc) clientcert.DialFunc {
				proxy := newHalfCloseConnectProxy(t)
				dial, err := clientcert.NewUpstreamDialer(clientcert.UpstreamConfig{
					ProxyServer: proxy.URL,
					Credentials: func() (string, string) { return "", "" },
					GOOS:        runtime.GOOS,
				}, direct)
				require.NoError(t, err)
				return dial
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			echoAddr := newEchoAfterEOFServer(t)
			resolver, err := newResolver(k6types.DefaultDNSConfig())
			require.NoError(t, err)
			direct := clientCertificateDirectDial(k6netext.NewDialer(net.Dialer{}, resolver))
			dial := tc.dial(t, direct)

			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			conn, err := dial(ctx, "tcp", echoAddr)
			require.NoError(t, err)
			defer conn.Close() //nolint:errcheck
			require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))

			_, err = io.WriteString(conn, "hello")
			require.NoError(t, err)
			cw, ok := conn.(interface{ CloseWrite() error })
			require.True(t, ok, "%T cannot be half-closed", conn)
			require.NoError(t, cw.CloseWrite())

			got, err := io.ReadAll(conn)
			require.NoError(t, err)
			assert.Equal(t, "echo:hello", string(got))
		})
	}
}

// newEchoAfterEOFServer starts a TCP server that reads until EOF and then writes back what it
// read, prefixed with "echo:".
func newEchoAfterEOFServer(t *testing.T) string {
	t.Helper()

	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close() //nolint:errcheck
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				got, err := io.ReadAll(conn)
				if err != nil {
					return
				}
				_, _ = conn.Write(append([]byte("echo:"), got...))
			}()
		}
	}()

	return ln.Addr().String()
}

// newHalfCloseConnectProxy starts an HTTP CONNECT proxy that propagates half-closes.
func newHalfCloseConnectProxy(t *testing.T) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT only", http.StatusMethodNotAllowed)
			return
		}
		upstream, err := (&net.Dialer{}).DialContext(r.Context(), "tcp", r.Host)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		hj, ok := w.(http.Hijacker)
		if !assert.True(t, ok) {
			_ = upstream.Close()
			return
		}
		client, buf, err := hj.Hijack()
		if !assert.NoError(t, err) {
			_ = upstream.Close()
			return
		}
		if _, err := io.WriteString(client, "HTTP/1.1 200 Connection established\r\n\r\n"); err != nil {
			_ = client.Close()
			_ = upstream.Close()
			return
		}
		go func() {
			done := make(chan struct{}, 2)
			half := func(dst net.Conn, src io.Reader) {
				_, _ = io.Copy(dst, src)
				if cw, ok := dst.(interface{ CloseWrite() error }); ok {
					_ = cw.CloseWrite()
				}
				done <- struct{}{}
			}
			go half(upstream, buf.Reader)
			go half(client, upstream)
			<-done
			<-done
			_ = client.Close()
			_ = upstream.Close()
		}()
	}))
	t.Cleanup(srv.Close)

	return srv
}

// newClientCertificateTestBrowser returns a browser whose CDP connection accepts
// CreateBrowserContext and fails DisposeBrowserContext with disposeErr.
func newClientCertificateTestBrowser(t *testing.T, disposeErr error) *Browser {
	t.Helper()

	vu := k6test.NewVU(t)
	vu.ActivateVU()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	b := newBrowser(ctx, k6ext.WithVU(ctx, vu), cancel, nil, NewLocalBrowserOptions(), log.NewNullLogger())
	b.conn = fakeConn{
		execute: func(_ context.Context, method string, _, res any) error {
			switch method {
			case target.CommandCreateBrowserContext:
				result, ok := res.(*target.CreateBrowserContextResult)
				require.True(t, ok)
				result.BrowserContextID = "client-certificate-context"
			case target.CommandDisposeBrowserContext:
				return disposeErr
			}
			return nil
		},
	}

	return b
}

// clientCertificateTestOptions returns browser context options with one client certificate.
func clientCertificateTestOptions(t *testing.T) *BrowserContextOptions {
	t.Helper()

	opts := DefaultBrowserContextOptions()
	opts.DownloadsPath = t.TempDir()
	opts.ClientCertificates = map[string][]tls.Certificate{"https://x.test": {{}}}
	return opts
}

func requireAcceptsConnections(t *testing.T, addr string) {
	t.Helper()

	conn, err := (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", addr)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}

func assertRefusesConnections(t *testing.T, addr string) {
	t.Helper()

	conn, err := (&net.Dialer{Timeout: time.Second}).DialContext(t.Context(), "tcp", addr)
	if err == nil {
		_ = conn.Close()
	}
	assert.Error(t, err, "proxy at %s still accepts connections", addr)
}
