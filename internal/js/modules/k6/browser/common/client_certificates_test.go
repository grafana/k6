package common

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/target"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/v2/internal/js/modules/k6/browser/log"
)

func TestBrowserNewContextClientCertificateProxyRules(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name           string
		proxy          *ProxyOptions
		wantHTTPPrefix string
		wantBypass     string
	}{
		{
			name:       "without_proxy",
			wantBypass: "<-loopback>",
		},
		{
			name:           "with_proxy",
			proxy:          &ProxyOptions{Server: " http://proxy.test:3128 "},
			wantHTTPPrefix: "http=http://proxy.test:3128;",
			wantBypass:     "<-loopback>",
		},
		{
			name:           "with_proxy_bypass",
			proxy:          &ProxyOptions{Server: "proxy.test:3128", Bypass: " .internal.test "},
			wantHTTPPrefix: "http=proxy.test:3128;",
			wantBypass:     "<-loopback>,.internal.test",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b, created := newClientCertificateTestBrowser(t, nil, nil)
			opts := clientCertificateTestOptions(t)
			opts.Proxy = tc.proxy
			bc, err := b.NewContext(opts)
			require.NoError(t, err)
			t.Cleanup(func() { _ = bc.cleanup() })

			params := receiveCreateBrowserContext(t, created)
			assert.Equal(t, tc.wantHTTPPrefix+"https="+bc.clientCertificateProxy.Addr(), params.ProxyServer)
			assert.Equal(t, tc.wantBypass, params.ProxyBypassList)
		})
	}
}

func TestBrowserNewContextClientCertificates_CreateFails(t *testing.T) {
	t.Parallel()

	createErr := errors.New("create failed")
	b, created := newClientCertificateTestBrowser(t, createErr, nil)

	bc, err := b.NewContext(clientCertificateTestOptions(t))
	require.ErrorIs(t, err, createErr)
	require.Nil(t, bc)

	proxyServer := receiveCreateBrowserContext(t, created).ProxyServer
	addr, ok := strings.CutPrefix(proxyServer, "https=")
	require.True(t, ok, proxyServer)
	assertRefusesConnections(t, addr)
}

// newClientCertificateTestBrowser returns a browser whose CDP connection sends the
// CreateBrowserContext parameters to the returned channel, fails CreateBrowserContext with
// createErr, and fails DisposeBrowserContext with disposeErr.
func newClientCertificateTestBrowser(
	t *testing.T, createErr, disposeErr error,
) (*Browser, <-chan *target.CreateBrowserContextParams) {
	t.Helper()

	vu := k6test.NewVU(t)
	vu.ActivateVU()
	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)
	b := newBrowser(ctx, k6ext.WithVU(ctx, vu), cancel, nil, NewLocalBrowserOptions(), log.NewNullLogger())
	created := make(chan *target.CreateBrowserContextParams, 1)
	b.conn = fakeConn{
		execute: func(_ context.Context, method string, params, res any) error {
			switch method {
			case target.CommandCreateBrowserContext:
				p, ok := params.(*target.CreateBrowserContextParams)
				require.True(t, ok)
				created <- p
				if createErr != nil {
					return createErr
				}
				result, ok := res.(*target.CreateBrowserContextResult)
				require.True(t, ok)
				result.BrowserContextID = "client-certificate-context"
			case target.CommandDisposeBrowserContext:
				return disposeErr
			}
			return nil
		},
	}

	return b, created
}

// receiveCreateBrowserContext returns the parameters of the CreateBrowserContext call.
func receiveCreateBrowserContext(
	t *testing.T, created <-chan *target.CreateBrowserContextParams,
) *target.CreateBrowserContextParams {
	t.Helper()

	select {
	case p := <-created:
		return p
	default:
		require.FailNow(t, "CreateBrowserContext was not called")
		return nil
	}
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
