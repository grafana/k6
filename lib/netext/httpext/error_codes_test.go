package httpext

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"

	"go.k6.io/k6/internal/lib/testutils/httpmultibin"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/lib/types"
)

func TestDefaultError(t *testing.T) {
	t.Parallel()
	testErrorCode(t, defaultErrorCode, fmt.Errorf("random error"))
}

func TestDNSErrors(t *testing.T) {
	t.Parallel()
	var (
		defaultDNSError = new(net.DNSError)
		noSuchHostError = new(net.DNSError)
	)

	noSuchHostError.Err = "no such host" // defined as private in go stdlib
	testTable := map[errCode]error{
		defaultDNSErrorCode:    defaultDNSError,
		dnsNoSuchHostErrorCode: noSuchHostError,
	}
	testMapOfErrorCodes(t, testTable)
}

func TestBlackListedIPError(t *testing.T) {
	t.Parallel()
	err := netext.BlackListedIPError{}
	testErrorCode(t, blackListedIPErrorCode, err)
	errorCode, errorMsg := errorCodeForError(err)
	require.NotEqual(t, err.Error(), errorMsg)
	require.Equal(t, blackListedIPErrorCode, errorCode)
}

type timeoutError bool

func (t timeoutError) Timeout() bool {
	return (bool)(t)
}

func (t timeoutError) Error() string {
	return fmt.Sprintf("%t", t)
}

func TestUnknownNetErrno(t *testing.T) {
	t.Parallel()
	err := new(net.OpError)
	err.Op = "write"
	err.Net = "tcp"
	err.Err = syscall.ENOTRECOVERABLE // Highly unlikely to actually need to do anything with this error
	expectedError := fmt.Sprintf(
		"write: unknown errno `%d` on %s with message `%s`",
		syscall.ENOTRECOVERABLE, runtime.GOOS, err.Err)
	errorCode, errorMsg := errorCodeForError(err)
	require.Equal(t, expectedError, errorMsg)
	require.Equal(t, netUnknownErrnoErrorCode, errorCode)
}

func TestTCPErrors(t *testing.T) {
	t.Parallel()
	var (
		nonTCPError       = &net.OpError{Net: "something", Err: errors.New("non tcp error")}
		econnreset        = &net.OpError{Net: "tcp", Op: "write", Err: &os.SyscallError{Err: syscall.ECONNRESET}}
		epipeerror        = &net.OpError{Net: "tcp", Op: "write", Err: &os.SyscallError{Err: syscall.EPIPE}}
		econnrefused      = &net.OpError{Net: "tcp", Op: "dial", Err: &os.SyscallError{Err: syscall.ECONNREFUSED}}
		errnounknown      = &net.OpError{Net: "tcp", Op: "dial", Err: &os.SyscallError{Err: syscall.E2BIG}}
		tcperror          = &net.OpError{Net: "tcp", Err: errors.New("tcp error")}
		notTimeoutedError = &net.OpError{Net: "tcp", Op: "dial", Err: timeoutError(false)}
	)

	testTable := map[errCode]error{
		defaultNetNonTCPErrorCode: nonTCPError,
		tcpResetByPeerErrorCode:   econnreset,
		tcpBrokenPipeErrorCode:    epipeerror,
		tcpDialRefusedErrorCode:   econnrefused,
		tcpDialUnknownErrnoCode:   errnounknown,
		defaultTCPErrorCode:       tcperror,
		tcpDialErrorCode:          notTimeoutedError,
	}

	testMapOfErrorCodes(t, testTable)
}

func testErrorCode(t *testing.T, code errCode, err error) {
	t.Helper()
	result, _ := errorCodeForError(err)
	require.Equalf(t, code, result, "Wrong error code for error `%s`", err)

	result, _ = errorCodeForError(fmt.Errorf("foo: %w", err))
	require.Equalf(t, code, result, "Wrong error code for error `%s`", err)

	result, _ = errorCodeForError(&url.Error{Err: err})
	require.Equalf(t, code, result, "Wrong error code for error `%s`", err)
}

func testMapOfErrorCodes(t *testing.T, testTable map[errCode]error) {
	t.Helper()
	for code, err := range testTable {
		testErrorCode(t, code, err)
	}
}

func TestConnReset(t *testing.T) {
	t.Parallel()
	// based on https://gist.github.com/jpittis/4357d817dc425ae99fbf719828ab1800
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr()
	ch := make(chan error, 10)

	go func() {
		defer close(ch)
		// Accept one connection.
		conn, innerErr := ln.Accept()
		if innerErr != nil {
			ch <- innerErr
			return
		}

		// Force an RST
		tcpConn, ok := conn.(*net.TCPConn)
		require.True(t, ok)
		innerErr = tcpConn.SetLinger(0)
		if innerErr != nil {
			ch <- innerErr
		}
		time.Sleep(time.Second) // Give time for the http request to start
		_ = conn.Close()
	}()

	res, err := http.Get("http://" + addr.String()) //nolint:bodyclose,noctx
	require.Nil(t, res)

	code, msg := errorCodeForError(err)
	assert.Equal(t, tcpResetByPeerErrorCode, code)
	assert.Contains(t, msg, fmt.Sprintf(tcpResetByPeerErrorCodeMsg, ""))
	for err := range ch {
		assert.Nil(t, err)
	}
}

func TestDnsResolve(t *testing.T) {
	t.Parallel()
	// this uses the Unwrap path
	// this is not happening in our current codebase as the resolution in our code
	// happens earlier so it doesn't get wrapped, but possibly happens in other cases as well
	_, err := http.Get("http://s.com") //nolint:bodyclose,noctx
	code, msg := errorCodeForError(err)

	assert.Equal(t, dnsNoSuchHostErrorCode, code)
	assert.Equal(t, dnsNoSuchHostErrorCodeMsg, msg)
}

func TestHTTP2StreamError(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	tb.Mux.HandleFunc("/tsr", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Length", "100000")
		rw.WriteHeader(http.StatusOK)

		f, ok := rw.(http.Flusher)
		if !ok {
			panic("expected http.ResponseWriter to be http.Flusher")
		}
		f.Flush()
		time.Sleep(time.Millisecond * 2)
		panic("expected internal error")
	})
	client := http.Client{
		Timeout:   time.Second * 3,
		Transport: tb.HTTPTransport,
	}

	res, err := client.Get(tb.Replacer.Replace("HTTP2BIN_URL/tsr")) //nolint:noctx
	require.NotNil(t, res)
	require.NoError(t, err)
	_, err = io.ReadAll(res.Body)
	_ = res.Body.Close()
	require.Error(t, err)

	code, msg := errorCodeForError(err)
	assert.Equal(t, unknownHTTP2StreamErrorCode+errCode(http2.ErrCodeInternal)+1, code)
	assert.Contains(t, msg, fmt.Sprintf(http2StreamErrorCodeMsg, http2.ErrCodeInternal))
}

func TestX509HostnameError(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	client := http.Client{
		Timeout:   time.Second * 3,
		Transport: tb.HTTPTransport,
	}
	var err error
	badHostname := "somewhere.else"
	badHost, err := types.NewHost(net.ParseIP(tb.Replacer.Replace("HTTPSBIN_IP")), "")
	require.NoError(t, err)

	tb.Dialer.Hosts, err = types.NewHosts(map[string]types.Host{
		badHostname: *badHost,
	})
	require.NoError(t, err)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, tb.Replacer.Replace("https://"+badHostname+":HTTPSBIN_PORT/get"), nil)
	require.NoError(t, err)
	res, err := client.Do(req) //nolint:bodyclose
	require.Nil(t, res)
	require.Error(t, err)

	code, msg := errorCodeForError(err)
	assert.Equal(t, x509HostnameErrorCode, code)
	assert.Contains(t, msg, x509HostnameErrorCodeMsg)
}

func TestX509UnknownAuthorityError(t *testing.T) {
	t.Parallel()
	tb := httpmultibin.NewHTTPMultiBin(t)

	client := http.Client{
		Timeout: time.Second * 3,
		Transport: &http.Transport{
			DialContext: tb.HTTPTransport.DialContext,
		},
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, tb.Replacer.Replace("HTTPSBIN_URL/get"), nil)
	require.NoError(t, err)
	res, err := client.Do(req) //nolint:bodyclose
	require.Nil(t, res)
	require.Error(t, err)

	code, msg := errorCodeForError(err)
	assert.Equal(t, x509UnknownAuthorityErrorCode, code)
	assert.Contains(t, msg, x509UnknownAuthority)
}

func TestDefaultTLSError(t *testing.T) {
	t.Parallel()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() {
		conn, err := l.Accept()
		require.NoError(t, err)
		_, err = conn.Write([]byte("not tls header")) // we just want to get an error
		require.NoError(t, err)
		// wait so it has time to get the tls header error and not the reset socket one
		time.Sleep(time.Second)
	}()

	client := http.Client{
		Timeout: time.Second * 3,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
			},
		},
	}

	_, err = client.Get("https://" + l.Addr().String()) //nolint:bodyclose,noctx
	require.Error(t, err)

	code, msg := errorCodeForError(err)
	assert.Equal(t, tlsHeaderErrorCode, code)
	urlError := new(url.Error)
	require.ErrorAs(t, err, &urlError)
	assert.Equal(t, urlError.Err.Error(), msg)
}

func TestHTTP2ConnectionError(t *testing.T) {
	t.Parallel()
	tb := getHTTP2ServerWithCustomConnContext(t)

	// Pre-configure the HTTP client transport with the dialer and TLS config (incl. HTTP2 support)
	tb.Mux.HandleFunc("/tsr", func(_ http.ResponseWriter, req *http.Request) {
		conn := req.Context().Value(connKey).(*tls.Conn)
		f := http2.NewFramer(conn, conn)
		require.NoError(t, f.WriteData(3213, false, []byte("something")))
	})
	client := http.Client{
		Timeout:   time.Second * 5,
		Transport: tb.HTTPTransport,
	}

	_, err := client.Get(tb.Replacer.Replace("HTTP2BIN_URL/tsr")) //nolint:bodyclose,noctx
	code, msg := errorCodeForError(err)
	assert.Equal(t, unknownHTTP2ConnectionErrorCode+errCode(http2.ErrCodeProtocol)+1, code)
	assert.Equal(t, fmt.Sprintf(http2ConnectionErrorCodeMsg, http2.ErrCodeProtocol), msg)
}

func TestHTTP2GoAwayError(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("Skipped due to https://github.com/grafana/k6/issues/4098")
	}
	tb := getHTTP2ServerWithCustomConnContext(t)
	tb.Mux.HandleFunc("/tsr", func(_ http.ResponseWriter, req *http.Request) {
		conn := req.Context().Value(connKey).(*tls.Conn)
		f := http2.NewFramer(conn, conn)
		require.NoError(t, f.WriteGoAway(4, http2.ErrCodeInadequateSecurity, []byte("whatever")))
		require.NoError(t, conn.CloseWrite())
	})
	client := http.Client{
		Timeout:   time.Second * 5,
		Transport: tb.HTTPTransport,
	}

	_, err := client.Get(tb.Replacer.Replace("HTTP2BIN_URL/tsr")) //nolint:bodyclose,noctx

	require.Error(t, err)
	code, msg := errorCodeForError(err)
	assert.Equal(t, unknownHTTP2GoAwayErrorCode+errCode(http2.ErrCodeInadequateSecurity)+1, code)
	assert.Equal(t, fmt.Sprintf(http2GoAwayErrorCodeMsg, http2.ErrCodeInadequateSecurity), msg)
}

type connKeyT int32

const connKey connKeyT = 2

func getHTTP2ServerWithCustomConnContext(t *testing.T) *httpmultibin.HTTPMultiBin {
	const http2Domain = "example.com"
	mux := http.NewServeMux()
	http2Srv := httptest.NewUnstartedServer(mux)
	http2Srv.EnableHTTP2 = true
	http2Srv.Config.ConnContext = func(ctx context.Context, c net.Conn) context.Context {
		return context.WithValue(ctx, connKey, c)
	}
	http2Srv.StartTLS()
	t.Cleanup(http2Srv.Close)
	tlsConfig := httpmultibin.GetTLSClientConfig(t, http2Srv)

	http2URL, err := url.Parse(http2Srv.URL)
	require.NoError(t, err)
	http2IP := net.ParseIP(http2URL.Hostname())
	require.NotNil(t, http2IP)
	http2DomainValue, err := types.NewHost(http2IP, "")
	require.NoError(t, err)

	// Set up the dialer with shorter timeouts and the custom domains
	dialer := netext.NewDialer(net.Dialer{
		Timeout:   2 * time.Second,
		KeepAlive: 10 * time.Second,
	}, netext.NewResolver(net.LookupIP, 0, types.DNSfirst, types.DNSpreferIPv4))
	dialer.Hosts, err = types.NewHosts(map[string]types.Host{
		http2Domain: *http2DomainValue,
	})
	require.NoError(t, err)

	transport := &http.Transport{
		DialContext:     dialer.DialContext,
		TLSClientConfig: tlsConfig,
	}
	require.NoError(t, http2.ConfigureTransport(transport))
	return &httpmultibin.HTTPMultiBin{
		Mux:         mux,
		ServerHTTP2: http2Srv,
		Replacer: strings.NewReplacer(
			"HTTP2BIN_IP_URL", http2Srv.URL,
			"HTTP2BIN_DOMAIN", http2Domain,
			"HTTP2BIN_URL", fmt.Sprintf("https://%s", net.JoinHostPort(http2Domain, http2URL.Port())),
			"HTTP2BIN_IP", http2IP.String(),
			"HTTP2BIN_PORT", http2URL.Port(),
		),
		TLSClientConfig: tlsConfig,
		Dialer:          dialer,
		HTTPTransport:   transport,
	}
}
