/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2018 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

// Package testutils is indended only for use in tests, do not import in production code!
package testutils

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/mccutchen/go-httpbin/httpbin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

// GetTLSClientConfig returns a TLS config that trusts the supplied
// httptest.Server certificate as well as all the system root certificates
func GetTLSClientConfig(t testing.TB, srv *httptest.Server) *tls.Config {
	var err error

	certs := x509.NewCertPool()

	if runtime.GOOS != "windows" {
		certs, err = x509.SystemCertPool()
		require.NoError(t, err)
	}

	for _, c := range srv.TLS.Certificates {
		roots, err := x509.ParseCertificates(c.Certificate[len(c.Certificate)-1])
		require.NoError(t, err)
		for _, root := range roots {
			certs.AddCert(root)
		}
	}
	return &tls.Config{
		RootCAs:            certs,
		InsecureSkipVerify: false,
		Renegotiation:      tls.RenegotiateFreelyAsClient,
	}
}

const httpDomain = "httpbin.local"

// We have to use example.com if we want a real HTTPS domain with a valid
// certificate because the default httptest certificate is for example.com:
// https://golang.org/src/net/http/internal/testcert.go?s=399:410#L10
const httpsDomain = "example.com"

// HTTPMultiBin can be used as a local alternative of httpbin.org. It offers both http and https servers, as well as real domains
type HTTPMultiBin struct {
	Mux             *http.ServeMux
	ServerHTTP      *httptest.Server
	ServerHTTPS     *httptest.Server
	Replacer        *strings.Replacer
	TLSClientConfig *tls.Config
	Dialer          *netext.Dialer
	HTTPTransport   *http.Transport
	Cleanup         func()
}

func getWebsocketEchoHandler(t testing.TB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Logf("[%p %s] Upgrading to websocket connection...", req, req.URL)
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		if !assert.NoError(t, err) {
			return
		}
		t.Logf("[%p %s] Upgraded...", req, req.URL)

		mt, message, err := conn.ReadMessage()
		t.Logf("[%p %s] Read message '%s' of type %d (error '%v')", req, req.URL, message, mt, err)
		assert.NoError(t, err)
		assert.NoError(t, conn.WriteMessage(mt, message))
		assert.NoError(t, conn.Close())
		t.Logf("[%p %s] Wrote back message '%s' of type %d and closed the connection", req, req.URL, message, mt)
	})
}
func getWebsocketCloserHandler(t testing.TB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		if !assert.NoError(t, err) {
			return
		}
		assert.NoError(t, conn.Close())
	})
}

// NewHTTPMultiBin returns a fully configured and running HTTPMultiBin
func NewHTTPMultiBin(t testing.TB) *HTTPMultiBin {
	// Create a http.ServeMux and set the httpbin handler as the default
	mux := http.NewServeMux()
	mux.Handle("/ws-echo", getWebsocketEchoHandler(t))
	mux.Handle("/ws-close", getWebsocketCloserHandler(t))
	mux.Handle("/", httpbin.New().Handler())

	// Initialize the HTTP server and get its details
	httpSrv := httptest.NewServer(mux)
	httpURL, err := url.Parse(httpSrv.URL)
	require.NoError(t, err)
	httpIP := net.ParseIP(httpURL.Hostname())
	require.NotNil(t, httpIP)

	// Initialize the HTTPS server and get its details and tls config
	httpsSrv := httptest.NewTLSServer(mux)
	httpsURL, err := url.Parse(httpsSrv.URL)
	require.NoError(t, err)
	httpsIP := net.ParseIP(httpsURL.Hostname())
	require.NotNil(t, httpsIP)
	tlsConfig := GetTLSClientConfig(t, httpsSrv)

	// Set up the dialer with shorter timeouts and the custom domains
	dialer := netext.NewDialer(net.Dialer{
		Timeout:   2 * time.Second,
		KeepAlive: 10 * time.Second,
		DualStack: true,
	})
	dialer.Hosts = map[string]net.IP{
		httpDomain:  httpIP,
		httpsDomain: httpsIP,
	}

	// Pre-configure the HTTP client transport with the dialer and TLS config (incl. HTTP2 support)
	transport := &http.Transport{
		DialContext:     dialer.DialContext,
		TLSClientConfig: tlsConfig,
	}
	require.NoError(t, http2.ConfigureTransport(transport))

	return &HTTPMultiBin{
		Mux:         mux,
		ServerHTTP:  httpSrv,
		ServerHTTPS: httpsSrv,
		Replacer: strings.NewReplacer(
			"HTTPBIN_IP_URL", httpSrv.URL,
			"HTTPBIN_DOMAIN", httpDomain,
			"HTTPBIN_URL", fmt.Sprintf("http://%s:%s", httpDomain, httpURL.Port()),
			"HTTPBIN_IP", httpIP.String(),
			"HTTPBIN_PORT", httpURL.Port(),
			"HTTPSBIN_IP_URL", httpsSrv.URL,
			"HTTPSBIN_DOMAIN", httpsDomain,
			"HTTPSBIN_URL", fmt.Sprintf("https://%s:%s", httpsDomain, httpsURL.Port()),
			"HTTPSBIN_IP", httpsIP.String(),
			"HTTPSBIN_PORT", httpsURL.Port(),
		),
		TLSClientConfig: tlsConfig,
		Dialer:          dialer,
		HTTPTransport:   transport,
		Cleanup: func() {
			httpsSrv.Close()
			httpSrv.Close()
		},
	}
}
