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

// Package httpmultibin is indended only for use in tests, do not import in production code!
package httpmultibin

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/gorilla/websocket"
	"github.com/klauspost/compress/zstd"
	"github.com/mccutchen/go-httpbin/httpbin"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	grpctest "google.golang.org/grpc/test/grpc_testing"

	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/netext"
	"github.com/loadimpact/k6/lib/netext/httpext"
	"github.com/loadimpact/k6/lib/types"
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
	ServerHTTP2     *httptest.Server
	ServerGRPC      *grpc.Server
	GRPCStub        *GRPCStub
	Replacer        *strings.Replacer
	TLSClientConfig *tls.Config
	Dialer          *netext.Dialer
	HTTPTransport   *http.Transport
	Context         context.Context
	Cleanup         func()
}

type jsonBody struct {
	Header      http.Header `json:"headers"`
	Compression string      `json:"compression"`
}

func getWebsocketHandler(echo bool, closePrematurely bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		if err != nil {
			return
		}
		if echo {
			messageType, r, e := conn.NextReader()
			if e != nil {
				return
			}
			var wc io.WriteCloser
			wc, err = conn.NextWriter(messageType)
			if err != nil {
				return
			}
			if _, err = io.Copy(wc, r); err != nil {
				return
			}
			if err = wc.Close(); err != nil {
				return
			}
		}
		// closePrematurely=true mimics an invalid WS server that doesn't
		// send a close control frame before closing the connection.
		if !closePrematurely {
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
			_ = conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
			// Wait for response control frame
			<-time.After(time.Second)
		}
		err = conn.Close()
		if err != nil {
			return
		}
	})
}

func writeJSON(w io.Writer, v interface{}) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return errors.Wrap(e.Encode(v), "failed to encode JSON")
}

func getEncodedHandler(t testing.TB, compressionType httpext.CompressionType) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		var (
			encoding string
			err      error
			encw     io.WriteCloser
		)

		switch compressionType {
		case httpext.CompressionTypeBr:
			encw = brotli.NewWriter(rw)
			encoding = "br"
		case httpext.CompressionTypeZstd:
			encw, _ = zstd.NewWriter(rw)
			encoding = "zstd"
		}

		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Add("Content-Encoding", encoding)
		data := jsonBody{
			Header:      req.Header,
			Compression: encoding,
		}
		err = writeJSON(encw, data)
		if encw != nil {
			_ = encw.Close()
		}
		if !assert.NoError(t, err) {
			return
		}
	})
}

func getZstdBrHandler(t testing.TB) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		encoding := "zstd, br"
		rw.Header().Set("Content-Type", "application/json")
		rw.Header().Add("Content-Encoding", encoding)
		data := jsonBody{
			Header:      req.Header,
			Compression: encoding,
		}

		bw := brotli.NewWriter(rw)
		zw, _ := zstd.NewWriter(bw)
		defer func() {
			_ = zw.Close()
			_ = bw.Close()
		}()

		require.NoError(t, writeJSON(zw, data))
	})
}

// GRPCStub is an easily customisable TestServiceServer
type GRPCStub struct {
	EmptyCallFunc func(context.Context, *grpctest.Empty) (*grpctest.Empty, error)
	UnaryCallFunc func(context.Context, *grpctest.SimpleRequest) (*grpctest.SimpleResponse, error)
}

// EmptyCall implements the interface for the gRPC TestServiceServer
func (s *GRPCStub) EmptyCall(ctx context.Context, req *grpctest.Empty) (*grpctest.Empty, error) {
	if s.EmptyCallFunc != nil {
		return s.EmptyCallFunc(ctx, req)
	}

	return nil, status.Errorf(codes.Unimplemented, "method EmptyCall not implemented")
}

// UnaryCall implements the interface for the gRPC TestServiceServer
func (s *GRPCStub) UnaryCall(ctx context.Context, req *grpctest.SimpleRequest) (*grpctest.SimpleResponse, error) {
	if s.UnaryCallFunc != nil {
		return s.UnaryCallFunc(ctx, req)
	}

	return nil, status.Errorf(codes.Unimplemented, "method UnaryCall not implemented")
}

// StreamingOutputCall implements the interface for the gRPC TestServiceServer
func (*GRPCStub) StreamingOutputCall(*grpctest.StreamingOutputCallRequest,
	grpctest.TestService_StreamingOutputCallServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamingOutputCall not implemented")
}

// StreamingInputCall implements the interface for the gRPC TestServiceServer
func (*GRPCStub) StreamingInputCall(grpctest.TestService_StreamingInputCallServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamingInputCall not implemented")
}

// FullDuplexCall implements the interface for the gRPC TestServiceServer
func (*GRPCStub) FullDuplexCall(grpctest.TestService_FullDuplexCallServer) error {
	return status.Errorf(codes.Unimplemented, "method FullDuplexCall not implemented")
}

// HalfDuplexCall implements the interface for the gRPC TestServiceServer
func (*GRPCStub) HalfDuplexCall(grpctest.TestService_HalfDuplexCallServer) error {
	return status.Errorf(codes.Unimplemented, "method HalfDuplexCall not implemented")
}

// NewHTTPMultiBin returns a fully configured and running HTTPMultiBin
func NewHTTPMultiBin(t testing.TB) *HTTPMultiBin {
	// Create a http.ServeMux and set the httpbin handler as the default
	mux := http.NewServeMux()
	mux.Handle("/brotli", getEncodedHandler(t, httpext.CompressionTypeBr))
	mux.Handle("/ws-echo", getWebsocketHandler(true, false))
	mux.Handle("/ws-echo-invalid", getWebsocketHandler(true, true))
	mux.Handle("/ws-close", getWebsocketHandler(false, false))
	mux.Handle("/ws-close-invalid", getWebsocketHandler(false, true))
	mux.Handle("/zstd", getEncodedHandler(t, httpext.CompressionTypeZstd))
	mux.Handle("/zstd-br", getZstdBrHandler(t))
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

	// Initialize the gRPC server
	grpcSrv := grpc.NewServer()
	stub := &GRPCStub{}
	grpctest.RegisterTestServiceServer(grpcSrv, stub)

	cmux := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcSrv.ServeHTTP(w, r)

			return
		}
		mux.ServeHTTP(w, r)
	})

	// Initialize the HTTP2 server, with a copy of the https tls config
	http2Srv := httptest.NewUnstartedServer(cmux)
	http2Srv.EnableHTTP2 = true
	http2Srv.TLS = &(*tlsConfig) // copy it
	http2Srv.TLS.NextProtos = []string{http2.NextProtoTLS}
	require.NoError(t, err)
	http2Srv.StartTLS()
	http2URL, err := url.Parse(http2Srv.URL)
	require.NoError(t, err)
	http2IP := net.ParseIP(http2URL.Hostname())
	require.NotNil(t, http2IP)

	httpDomainValue, err := lib.NewHostAddress(httpIP, "")
	require.NoError(t, err)
	httpsDomainValue, err := lib.NewHostAddress(httpsIP, "")
	require.NoError(t, err)

	// Set up the dialer with shorter timeouts and the custom domains
	dialer := netext.NewDialer(net.Dialer{
		Timeout:   2 * time.Second,
		KeepAlive: 10 * time.Second,
		DualStack: true,
	}, netext.NewResolver(net.LookupIP, 0, types.DNSfirst, types.DNSpreferIPv4))
	dialer.Hosts = map[string]*lib.HostAddress{
		httpDomain:  httpDomainValue,
		httpsDomain: httpsDomainValue,
	}

	// Pre-configure the HTTP client transport with the dialer and TLS config (incl. HTTP2 support)
	transport := &http.Transport{
		DialContext:     dialer.DialContext,
		TLSClientConfig: tlsConfig,
	}
	require.NoError(t, http2.ConfigureTransport(transport))

	ctx, ctxCancel := context.WithCancel(context.Background())

	return &HTTPMultiBin{
		Mux:         mux,
		ServerHTTP:  httpSrv,
		ServerHTTPS: httpsSrv,
		ServerHTTP2: http2Srv,
		ServerGRPC:  grpcSrv,
		GRPCStub:    stub,
		Replacer: strings.NewReplacer(
			"HTTPBIN_IP_URL", httpSrv.URL,
			"HTTPBIN_DOMAIN", httpDomain,
			"HTTPBIN_URL", fmt.Sprintf("http://%s:%s", httpDomain, httpURL.Port()),
			"WSBIN_URL", fmt.Sprintf("ws://%s:%s", httpDomain, httpURL.Port()),
			"HTTPBIN_IP", httpIP.String(),
			"HTTPBIN_PORT", httpURL.Port(),
			"HTTPSBIN_IP_URL", httpsSrv.URL,
			"HTTPSBIN_DOMAIN", httpsDomain,
			"HTTPSBIN_URL", fmt.Sprintf("https://%s:%s", httpsDomain, httpsURL.Port()),
			"WSSBIN_URL", fmt.Sprintf("wss://%s:%s", httpsDomain, httpsURL.Port()),
			"HTTPSBIN_IP", httpsIP.String(),
			"HTTPSBIN_PORT", httpsURL.Port(),

			"HTTP2BIN_IP_URL", http2Srv.URL,
			"HTTP2BIN_DOMAIN", httpsDomain,
			"HTTP2BIN_URL", fmt.Sprintf("https://%s:%s", httpsDomain, http2URL.Port()),
			"HTTP2BIN_IP", http2IP.String(),
			"HTTP2BIN_PORT", http2URL.Port(),

			"GRPCBIN_ADDR", fmt.Sprintf("%s:%s", httpsDomain, http2URL.Port()),
		),
		TLSClientConfig: tlsConfig,
		Dialer:          dialer,
		HTTPTransport:   transport,
		Context:         ctx,
		Cleanup: func() {
			grpcSrv.Stop()
			http2Srv.Close()
			httpsSrv.Close()
			httpSrv.Close()
			ctxCancel()
		},
	}
}
