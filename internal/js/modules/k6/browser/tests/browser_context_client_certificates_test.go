package tests

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/v2/internal/lib/testutils/tlstest"
)

// clientCertNavigation is the outcome of navigating a page to a URL.
type clientCertNavigation struct {
	Status int    `json:"status"`
	Text   string `json:"text"`
}

// clientCertOption is one clientCertificates entry.
type clientCertOption struct {
	Origin string `json:"origin"`
	Cert   string `json:"cert"`
	Key    string `json:"key"`
}

func TestBrowserContextClientCertificates(t *testing.T) {
	t.Parallel()

	chain := tlstest.NewChain(t)
	rsaCert, rsaKey := chain.ClientPEM(t, "k6-rsa-client", tlstest.KeyRSA)
	rsaFor := func(origin string) clientCertOption {
		return clientCertOption{Origin: origin, Cert: string(rsaCert), Key: string(rsaKey)}
	}
	requireCert := clientCertServerOptions{clientAuth: tls.RequireAndVerifyClientCert}
	rsaPage := clientCertNavigation{Status: 200, Text: "cn:k6-rsa-client proto:1"}

	// The proxy verifies matching origins against the system roots, which do not include
	// the test chain's root, so cases expecting a certificate to reach the server set
	// ignoreHTTPSErrors.
	for _, tc := range []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "matching_origin_new_context",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, chain, requireCert)
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"ignoreHTTPSErrors":  true,
					"clientCertificates": []clientCertOption{rsaFor(srv.URL)},
				}, srv.URL)
				assert.Equal(t, []clientCertNavigation{rsaPage}, got)
			},
		},
		{
			name: "matching_origin_new_page",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, chain, requireCert)
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newPage", map[string]any{
					"ignoreHTTPSErrors":  true,
					"clientCertificates": []clientCertOption{rsaFor(srv.URL)},
				}, srv.URL)
				assert.Equal(t, []clientCertNavigation{rsaPage}, got)
			},
		},
		{
			name: "localhost_target",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, chain, requireCert)
				origin := "https://localhost:" + clientCertPort(t, srv.URL)
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"ignoreHTTPSErrors":  true,
					"clientCertificates": []clientCertOption{rsaFor(origin)},
				}, origin+"/")
				assert.Equal(t, []clientCertNavigation{rsaPage}, got)
			},
		},
		{
			name: "non_matching_origin",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, chain, clientCertServerOptions{clientAuth: tls.VerifyClientCertIfGiven})
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"ignoreHTTPSErrors":  true,
					"clientCertificates": []clientCertOption{rsaFor("https://other.test")},
				}, srv.URL)
				assert.Equal(t, []clientCertNavigation{{Status: 200, Text: "cn:none proto:1"}}, got)
			},
		},
		{
			name: "untrusted_non_matching_origin_loads",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, chain, clientCertServerOptions{})
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"clientCertificates": []clientCertOption{rsaFor("https://other.test")},
				}, srv.URL)
				assert.Equal(t, []clientCertNavigation{{Status: 200, Text: "cn:none proto:1"}}, got)
			},
		},
		{
			name: "untrusted_matching_origin_503",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, chain, requireCert)
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"clientCertificates": []clientCertOption{rsaFor(srv.URL)},
				}, srv.URL)
				require.Len(t, got, 1)
				assert.Equal(t, 503, got[0].Status)
				assert.True(t, strings.HasPrefix(got[0].Text, "k6 client-certificate error: "), got[0].Text)
			},
		},
		{
			name: "h2",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, chain, clientCertServerOptions{
					clientAuth: tls.RequireAndVerifyClientCert, h2: true,
				})
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"ignoreHTTPSErrors":  true,
					"clientCertificates": []clientCertOption{rsaFor(srv.URL)},
				}, srv.URL)
				assert.Equal(t, []clientCertNavigation{{Status: 200, Text: "cn:k6-rsa-client proto:2"}}, got)
			},
		},
		{
			name: "wss_echo",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, chain, requireCert)
				options, err := json.Marshal(map[string]any{
					"ignoreHTTPSErrors":  true,
					"clientCertificates": []clientCertOption{rsaFor(srv.URL)},
				})
				require.NoError(t, err)
				wsURL := "wss://" + strings.TrimPrefix(srv.URL, "https://") + "/ws"

				got := runClientCertificatesScript(t, newTestBrowser(t), `
					const context = await browser.newContext(%s);
					const page = await context.newPage();
					try {
						await page.goto(%q, { waitUntil: 'load' });
						return await page.evaluate((u) => new Promise((resolve, reject) => {
							const ws = new WebSocket(u);
							ws.onopen = () => ws.send('ping');
							ws.onmessage = (e) => { resolve(e.data); ws.close(); };
							ws.onerror = () => reject(new Error('websocket error'));
						}), %q);
					} finally {
						await page.close();
						await context.close();
					}
				`, options, srv.URL, wsURL)
				assert.Equal(t, "cn:k6-rsa-client:ping", got)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}
}

func TestBrowserContextWithoutClientCertificates(t *testing.T) {
	t.Parallel()

	srv := newClientCertServer(t, tlstest.NewChain(t), clientCertServerOptions{})
	tb := newTestBrowser(t)
	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)
	defer tb.vu.EndIteration(t)

	gv, err := tb.vu.RunAsync(t, `
		const context = await browser.newContext();
		const page = await context.newPage();
		try {
			await page.goto(%q);
		} finally {
			await page.close();
			await context.close();
		}
	`, srv.URL)

	got := k6test.ToPromise(t, gv)
	assert.ErrorContains(t, err, "net::ERR_CERT_AUTHORITY_INVALID")
	assert.Equal(t, sobek.PromiseStateRejected, got.State())
}

// navigateWithClientCertificates creates a page with options through browser.<method>
// ("newContext" or "newPage"), navigates it to each of targetURLs in turn, and returns the
// responses' status and body text.
func navigateWithClientCertificates(
	t *testing.T, tb *testBrowser, method string, options map[string]any, targetURLs ...string,
) []clientCertNavigation {
	t.Helper()

	opts, err := json.Marshal(options)
	require.NoError(t, err)
	urls, err := json.Marshal(targetURLs)
	require.NoError(t, err)

	open := `const context = await browser.newContext(%s); const page = await context.newPage();`
	if method == "newPage" {
		open = `const page = await browser.newPage(%s); const context = page.context();`
	}
	result := runClientCertificatesScript(t, tb, open+`
		try {
			const got = [];
			for (const u of %s) {
				const res = await page.goto(u, { waitUntil: 'load' });
				// page.goto can resolve to null even though the page loaded (an existing race in
				// k6's navigation handling), so fall back to the page's own record of the status.
				const status = res ? res.status() : await page.evaluate(
					() => performance.getEntriesByType('navigation')[0].responseStatus);
				got.push({ status, text: await page.textContent('body') });
			}
			return JSON.stringify(got);
		} finally {
			await page.close();
			await context.close();
		}
	`, opts, urls)

	var got []clientCertNavigation
	require.NoError(t, json.Unmarshal([]byte(result), &got))
	return got
}

// runClientCertificatesScript runs script in an iteration and returns its fulfilled result.
func runClientCertificatesScript(t *testing.T, tb *testBrowser, script string, args ...any) string {
	t.Helper()

	tb.vu.ActivateVU()
	tb.vu.StartIteration(t)
	defer tb.vu.EndIteration(t)

	gv, err := tb.vu.RunAsync(t, script, args...)
	require.NoError(t, err)
	got := k6test.ToPromise(t, gv)
	require.Equal(t, sobek.PromiseStateFulfilled, got.State())
	return got.Result().String()
}

func clientCertPort(t *testing.T, serverURL string) string {
	t.Helper()

	u, err := url.Parse(serverURL)
	require.NoError(t, err)
	return u.Port()
}

type clientCertServerOptions struct {
	clientAuth tls.ClientAuthType
	h2         bool
}

// newClientCertServer starts an HTTPS server with chain's leaf, for localhost and 127.0.0.1,
// accepting client certificates issued by chain. Its pages report the client certificate CN
// (or "none") and the HTTP major version as "cn:<cn> proto:<major>"; /ws echoes WebSocket
// messages prefixed with "cn:<cn>:".
func newClientCertServer(t *testing.T, chain *tlstest.Chain, opts clientCertServerOptions) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer conn.Close() //nolint:errcheck
		for {
			mt, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if err := conn.WriteMessage(mt, []byte("cn:"+clientCertCN(r)+":"+string(msg))); err != nil {
				return
			}
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := fmt.Fprintf(w,
			`<!doctype html><html><head><link rel="icon" href="data:,"></head><body>cn:%s proto:%d</body></html>`,
			clientCertCN(r), r.ProtoMajor)
		assert.NoError(t, err)
	})

	srv := httptest.NewUnstartedServer(mux)
	srv.EnableHTTP2 = opts.h2
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{chain.FullChainTLSCertificate(t)},
		ClientAuth:   opts.clientAuth,
		ClientCAs:    chain.RootPool,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	return srv
}

func clientCertCN(r *http.Request) string {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return "none"
	}
	return r.TLS.PeerCertificates[0].Subject.CommonName
}
