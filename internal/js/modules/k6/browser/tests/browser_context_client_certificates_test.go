package tests

import (
	"crypto/tls"
	"encoding/json"
	"net"
	"net/url"
	"strings"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.k6.io/k6/v2/internal/js/modules/k6/browser/k6ext/k6test"
	"go.k6.io/k6/v2/lib/types"
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

	ca := newClientCertCA(t)
	rsaClient := ca.issueClient(t, "k6-rsa-client", clientCertKeyRSA)
	ecdsaClient := ca.issueClient(t, "k6-ecdsa-client", clientCertKeyECDSA)
	certFor := func(origin string, c clientCertPEM) clientCertOption {
		return clientCertOption{Origin: origin, Cert: c.Cert, Key: c.Key}
	}
	requireCert := clientCertServerOptions{clientAuth: tls.RequireAndVerifyClientCert}

	// The proxy verifies matching origins against the system roots, which do not include
	// the test CA, so cases expecting a certificate to reach the server set ignoreHTTPSErrors.
	for _, tc := range []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "matching_origin_new_context",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, requireCert)
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"ignoreHTTPSErrors":  true,
					"clientCertificates": []clientCertOption{certFor(srv.URL, rsaClient)},
				}, srv.URL)
				assert.Equal(t, clientCertNavigation{Status: 200, Text: "cn:k6-rsa-client proto:1"}, got)
			},
		},
		{
			name: "matching_origin_new_page",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, requireCert)
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newPage", map[string]any{
					"ignoreHTTPSErrors":  true,
					"clientCertificates": []clientCertOption{certFor(srv.URL, rsaClient)},
				}, srv.URL)
				assert.Equal(t, clientCertNavigation{Status: 200, Text: "cn:k6-rsa-client proto:1"}, got)
			},
		},
		{
			name: "localhost_target",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, requireCert)
				origin := "https://localhost:" + clientCertPort(t, srv.URL)
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"ignoreHTTPSErrors":  true,
					"clientCertificates": []clientCertOption{certFor(origin, rsaClient)},
				}, origin+"/")
				assert.Equal(t, clientCertNavigation{Status: 200, Text: "cn:k6-rsa-client proto:1"}, got)
			},
		},
		{
			name: "non_matching_origin",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, clientCertServerOptions{clientAuth: tls.VerifyClientCertIfGiven})
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"clientCertificates": []clientCertOption{certFor("https://other.test", rsaClient)},
				}, srv.URL)
				assert.Equal(t, clientCertNavigation{Status: 200, Text: "cn:none proto:1"}, got)
			},
		},
		{
			name: "untrusted_non_matching_origin_loads",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, clientCertServerOptions{selfSigned: true})
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"clientCertificates": []clientCertOption{certFor("https://other.test", rsaClient)},
				}, srv.URL)
				assert.Equal(t, clientCertNavigation{Status: 200, Text: "cn:none proto:1"}, got)
			},
		},
		{
			name: "untrusted_matching_origin_503",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, clientCertServerOptions{
					clientAuth: tls.RequireAndVerifyClientCert, selfSigned: true,
				})
				tb := newTestBrowser(t)
				certs := []clientCertOption{certFor(srv.URL, rsaClient)}

				got := navigateWithClientCertificates(t, tb, "newContext", map[string]any{
					"clientCertificates": certs,
				}, srv.URL)
				assert.Equal(t, 503, got.Status)
				assert.True(t, strings.HasPrefix(got.Text, "k6 client-certificate error: "), got.Text)

				got = navigateWithClientCertificates(t, tb, "newContext", map[string]any{
					"ignoreHTTPSErrors":  true,
					"clientCertificates": certs,
				}, srv.URL)
				assert.Equal(t, clientCertNavigation{Status: 200, Text: "cn:k6-rsa-client proto:1"}, got)
			},
		},
		{
			name: "h2",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, clientCertServerOptions{
					clientAuth: tls.RequireAndVerifyClientCert, h2: true,
				})
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"ignoreHTTPSErrors":  true,
					"clientCertificates": []clientCertOption{certFor(srv.URL, rsaClient)},
				}, srv.URL)
				assert.Equal(t, clientCertNavigation{Status: 200, Text: "cn:k6-rsa-client proto:2"}, got)
			},
		},
		{
			name: "websocket",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, requireCert)
				options, err := json.Marshal(map[string]any{
					"ignoreHTTPSErrors":  true,
					"clientCertificates": []clientCertOption{certFor(srv.URL, rsaClient)},
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
		{
			name: "via_context_proxy",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, requireCert)
				proxy, connects := newClientCertConnectProxy(t, "", "")
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"ignoreHTTPSErrors":  true,
					"proxy":              map[string]any{"server": proxy.URL},
					"clientCertificates": []clientCertOption{certFor(srv.URL, rsaClient)},
				}, srv.URL)
				assert.Equal(t, clientCertNavigation{Status: 200, Text: "cn:k6-rsa-client proto:1"}, got)
				assert.Contains(t, drainClientCertConnects(connects), "CONNECT "+strings.TrimPrefix(srv.URL, "https://"))
			},
		},
		{
			name: "via_context_proxy_407",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, requireCert)
				proxy, connects := newClientCertConnectProxy(t, "proxy-user", "proxy-pass")
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"ignoreHTTPSErrors":  true,
					"proxy":              map[string]any{"server": proxy.URL},
					"httpCredentials":    map[string]any{"username": "proxy-user", "password": "proxy-pass"},
					"clientCertificates": []clientCertOption{certFor(srv.URL, rsaClient)},
				}, srv.URL)
				assert.Equal(t, clientCertNavigation{Status: 200, Text: "cn:k6-rsa-client proto:1"}, got)
				assert.Contains(t, drainClientCertConnects(connects), "CONNECT "+strings.TrimPrefix(srv.URL, "https://"))
			},
		},
		{
			name: "bypass_host_direct",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, clientCertServerOptions{clientAuth: tls.VerifyClientCertIfGiven})
				proxy, connects := newClientCertConnectProxy(t, "", "")
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"ignoreHTTPSErrors":  true,
					"proxy":              map[string]any{"server": proxy.URL, "bypass": "127.0.0.1"},
					"clientCertificates": []clientCertOption{certFor(srv.URL, rsaClient)},
				}, srv.URL)
				assert.Equal(t, clientCertNavigation{Status: 200, Text: "cn:none proto:1"}, got)
				// Chromium's own background requests may still reach the proxy.
				assert.NotContains(t, drainClientCertConnects(connects), "CONNECT "+strings.TrimPrefix(srv.URL, "https://"))
			},
		},
		{
			name: "k6_hosts_mapping",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, requireCert)
				port := clientCertPort(t, srv.URL)
				tb := newTestBrowser(t)
				hosts, err := types.NewNullHosts(map[string]types.Host{
					"clientcert.test": {IP: net.IPv4(127, 0, 0, 1), Port: srv.Listener.Addr().(*net.TCPAddr).Port},
				})
				require.NoError(t, err)
				tb.vu.StateField.Options.Hosts = hosts

				origin := "https://clientcert.test:" + port
				got := navigateWithClientCertificates(t, tb, "newContext", map[string]any{
					"ignoreHTTPSErrors":  true,
					"clientCertificates": []clientCertOption{certFor(origin, rsaClient)},
				}, origin+"/")
				assert.Equal(t, clientCertNavigation{Status: 200, Text: "cn:k6-rsa-client proto:1"}, got)
			},
		},
		{
			name: "mixed_key_types_tls13",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, clientCertServerOptions{
					clientAuth: tls.RequireAndVerifyClientCert, minVersion: tls.VersionTLS13,
				})
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"ignoreHTTPSErrors": true,
					"clientCertificates": []clientCertOption{
						certFor(srv.URL, ecdsaClient), certFor(srv.URL, rsaClient),
					},
				}, srv.URL)
				assert.Equal(t, clientCertNavigation{Status: 200, Text: "cn:k6-rsa-client proto:1"}, got)
			},
		},
		{
			name: "mixed_key_types_tls12",
			run: func(t *testing.T) {
				srv := newClientCertServer(t, ca, clientCertServerOptions{
					clientAuth: tls.RequireAndVerifyClientCert, maxVersion: tls.VersionTLS12,
				})
				got := navigateWithClientCertificates(t, newTestBrowser(t), "newContext", map[string]any{
					"ignoreHTTPSErrors": true,
					"clientCertificates": []clientCertOption{
						certFor(srv.URL, rsaClient), certFor(srv.URL, ecdsaClient),
					},
				}, srv.URL)
				assert.Equal(t, clientCertNavigation{Status: 200, Text: "cn:k6-ecdsa-client proto:1"}, got)
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

	srv := newClientCertServer(t, newClientCertCA(t), clientCertServerOptions{selfSigned: true})
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
// ("newContext" or "newPage"), navigates it to targetURL, and returns the response status and
// body text.
func navigateWithClientCertificates(
	t *testing.T, tb *testBrowser, method string, options map[string]any, targetURL string,
) clientCertNavigation {
	t.Helper()

	opts, err := json.Marshal(options)
	require.NoError(t, err)

	open := `const context = await browser.newContext(%s); const page = await context.newPage();`
	if method == "newPage" {
		open = `const page = await browser.newPage(%s); const context = page.context();`
	}
	result := runClientCertificatesScript(t, tb, open+`
		try {
			const res = await page.goto(%q, { waitUntil: 'load' });
			return JSON.stringify({ status: res.status(), text: await page.textContent('body') });
		} finally {
			await page.close();
			await context.close();
		}
	`, opts, targetURL)

	var got clientCertNavigation
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

func drainClientCertConnects(connects <-chan string) []string {
	var got []string
	for {
		select {
		case c := <-connects:
			got = append(got, c)
		default:
			return got
		}
	}
}

func clientCertPort(t *testing.T, serverURL string) string {
	t.Helper()

	u, err := url.Parse(serverURL)
	require.NoError(t, err)
	return u.Port()
}
