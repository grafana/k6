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
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.k6.io/k6/lib/netext"
	grpcanytesting "go.k6.io/k6/lib/testutils/httpmultibin/grpc_any_testing"
	grpctest "go.k6.io/k6/lib/testutils/httpmultibin/grpc_testing"
	"go.k6.io/k6/lib/types"
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
		MinVersion:         tls.VersionTLS10,
	}
}

const httpDomain = "httpbin.local"

// We have to use example.com if we want a real HTTPS domain with a valid
// certificate because the default httptest certificate is for example.com:
// https://golang.org/src/net/http/internal/testcert.go?s=399:410#L10
const httpsDomain = "example.com"

const localhostCert = `-----BEGIN CERTIFICATE-----
MIIDOTCCAiGgAwIBAgIQSRJrEpBGFc7tNb1fb5pKFzANBgkqhkiG9w0BAQsFADAS
MRAwDgYDVQQKEwdBY21lIENvMCAXDTcwMDEwMTAwMDAwMFoYDzIwODQwMTI5MTYw
MDAwWjASMRAwDgYDVQQKEwdBY21lIENvMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A
MIIBCgKCAQEA6Gba5tHV1dAKouAaXO3/ebDUU4rvwCUg/CNaJ2PT5xLD4N1Vcb8r
bFSW2HXKq+MPfVdwIKR/1DczEoAGf/JWQTW7EgzlXrCd3rlajEX2D73faWJekD0U
aUgz5vtrTXZ90BQL7WvRICd7FlEZ6FPOcPlumiyNmzUqtwGhO+9ad1W5BqJaRI6P
YfouNkwR6Na4TzSj5BrqUfP0FwDizKSJ0XXmh8g8G9mtwxOSN3Ru1QFc61Xyeluk
POGKBV/q6RBNklTNe0gI8usUMlYyoC7ytppNMW7X2vodAelSu25jgx2anj9fDVZu
h7AXF5+4nJS4AAt0n1lNY7nGSsdZas8PbQIDAQABo4GIMIGFMA4GA1UdDwEB/wQE
AwICpDATBgNVHSUEDDAKBggrBgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MB0GA1Ud
DgQWBBStsdjh3/JCXXYlQryOrL4Sh7BW5TAuBgNVHREEJzAlggtleGFtcGxlLmNv
bYcEfwAAAYcQAAAAAAAAAAAAAAAAAAAAATANBgkqhkiG9w0BAQsFAAOCAQEAxWGI
5NhpF3nwwy/4yB4i/CwwSpLrWUa70NyhvprUBC50PxiXav1TeDzwzLx/o5HyNwsv
cxv3HdkLW59i/0SlJSrNnWdfZ19oTcS+6PtLoVyISgtyN6DpkKpdG1cOkW3Cy2P2
+tK/tKHRP1Y/Ra0RiDpOAmqn0gCOFGz8+lqDIor/T7MTpibL3IxqWfPrvfVRHL3B
grw/ZQTTIVjjh4JBSW3WyWgNo/ikC1lrVxzl4iPUGptxT36Cr7Zk2Bsg0XqwbOvK
5d+NTDREkSnUbie4GeutujmX3Dsx88UiV6UY/4lHJa6I5leHUNOHahRbpbWeOfs/
WkBKOclmOV2xlTVuPw==
-----END CERTIFICATE-----`

const localhostKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQDoZtrm0dXV0Aqi
4Bpc7f95sNRTiu/AJSD8I1onY9PnEsPg3VVxvytsVJbYdcqr4w99V3AgpH/UNzMS
gAZ/8lZBNbsSDOVesJ3euVqMRfYPvd9pYl6QPRRpSDPm+2tNdn3QFAvta9EgJ3sW
URnoU85w+W6aLI2bNSq3AaE771p3VbkGolpEjo9h+i42TBHo1rhPNKPkGupR8/QX
AOLMpInRdeaHyDwb2a3DE5I3dG7VAVzrVfJ6W6Q84YoFX+rpEE2SVM17SAjy6xQy
VjKgLvK2mk0xbtfa+h0B6VK7bmODHZqeP18NVm6HsBcXn7iclLgAC3SfWU1jucZK
x1lqzw9tAgMBAAECggEABWzxS1Y2wckblnXY57Z+sl6YdmLV+gxj2r8Qib7g4ZIk
lIlWR1OJNfw7kU4eryib4fc6nOh6O4AWZyYqAK6tqNQSS/eVG0LQTLTTEldHyVJL
dvBe+MsUQOj4nTndZW+QvFzbcm2D8lY5n2nBSxU5ypVoKZ1EqQzytFcLZpTN7d89
EPj0qDyrV4NZlWAwL1AygCwnlwhMQjXEalVF1ylXwU3QzyZ/6MgvF6d3SSUlh+sq
XefuyigXw484cQQgbzopv6niMOmGP3of+yV4JQqUSb3IDmmT68XjGd2Dkxl4iPki
6ZwXf3CCi+c+i/zVEcufgZ3SLf8D99kUGE7v7fZ6AQKBgQD1ZX3RAla9hIhxCf+O
3D+I1j2LMrdjAh0ZKKqwMR4JnHX3mjQI6LwqIctPWTU8wYFECSh9klEclSdCa64s
uI/GNpcqPXejd0cAAdqHEEeG5sHMDt0oFSurL4lyud0GtZvwlzLuwEweuDtvT9cJ
Wfvl86uyO36IW8JdvUprYDctrQKBgQDycZ697qutBieZlGkHpnYWUAeImVA878sJ
w44NuXHvMxBPz+lbJGAg8Cn8fcxNAPqHIraK+kx3po8cZGQywKHUWsxi23ozHoxo
+bGqeQb9U661TnfdDspIXia+xilZt3mm5BPzOUuRqlh4Y9SOBpSWRmEhyw76w4ZP
OPxjWYAgwQKBgA/FehSYxeJgRjSdo+MWnK66tjHgDJE8bYpUZsP0JC4R9DL5oiaA
brd2fI6Y+SbyeNBallObt8LSgzdtnEAbjIH8uDJqyOmknNePRvAvR6mP4xyuR+Bv
m+Lgp0DMWTw5J9CKpydZDItc49T/mJ5tPhdFVd+am0NAQnmr1MCZ6nHxAoGABS3Y
LkaC9FdFUUqSU8+Chkd/YbOkuyiENdkvl6t2e52jo5DVc1T7mLiIrRQi4SI8N9bN
/3oJWCT+uaSLX2ouCtNFunblzWHBrhxnZzTeqVq4SLc8aESAnbslKL4i8/+vYZlN
s8xtiNcSvL+lMsOBORSXzpj/4Ot8WwTkn1qyGgECgYBKNTypzAHeLE6yVadFp3nQ
Ckq9yzvP/ib05rvgbvrne00YeOxqJ9gtTrzgh7koqJyX1L4NwdkEza4ilDWpucn0
xiUZS4SoaJq6ZvcBYS62Yr1t8n09iG47YL8ibgtmH3L+svaotvpVxVK+d7BLevA/
ZboOWVe3icTy64BT3OQhmg==
-----END RSA PRIVATE KEY-----`

// trivial password: abc123
const localhostEncryptedKey = `-----BEGIN RSA PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: AES-256-CBC,B2557B8662FBEC979823E6F51B8ED777

9xXt7ZCYHjYz501uiQKPLpxmz1qGNwu/u2VCwu/dFql2BLGfKrk5j4ZvaoKqUwVB
QfUaisSv1g++Rh13qDOOvRO38TF7aQPxImqCw2ew/fFC0JTiPWpSaQtIcWOxASpa
s84Z4LIolfeLxXyOG3JwWeKG/WQCMf1QNM+LXlfCJU6vdY0KeoDUcp4CkFNDdUrx
qGaF4xJxaQfLSSLuXlWTYmz+IlPMy/xOUCn3eSemWd4ZBdFUIwsSsuQkZHzthjJg
DbvAzuEzvQENXInnfYHkA4XHM+SMV+4d+aZgPNUSWv3YfXeOhapdpeAjVq6Q2TiX
xkFOjFYUKWO6sLWS5WfB1eqwwh9vNkZVHbmYYUvJbc2Aw0EJlQWxeQ6Vj0d6TIev
EPr6jFROaJ5kTd2XxG4HzJcGWsV27q4r159GGGmrZk1GjGZKImWP6Y5f3t4u+uJZ
EHVGx+SkilEaOM0ar1sXGgtIif741GRYYibd9hD1+0hSOxyVpehtEeb/wJRG4Vd6
i07ANkqwOop4K/nW5OOXKEz6eDrXAAJ2gzjN74WCyR5nJ2XoTjUa9Vo9hNrBcmtL
dRoeHOu9BhN+mu8YPwdisjtK6AJorsf0bQWqGpnexFw9Fq/XMpCTBT6P5X6gsNAs
RKvS5bwai7e9pJqZY+iJjdCTnFflDTX/r/lt4SxIkoZvGoGEVeMJsc6yFZP6yDCx
zjZkk0R3WsVusheATVBJJJcsHLdaUR707TU5lmhVFx0BYcBVrfKNc1h0E+alSM5R
ij3T88ipk8BN4/e6gUpQCptMtda7wFCxiAIU+l1skGmM29ZvPB8BNAbMFCxioBGk
Me6QWcqOTLzoLwFHH2hSPYhZKeadCyz53OKbyIK/m06BXMVpFaIxesJZx6qW3aew
gShlUwr7yu9QJlxGZX0wIC1dyT69lRcLsbqzqnp6EMspSEZYvysq2k0GKZyAqLnr
e9CClm0wMnj45SK34/s1BWZNbBgXkDlzTKwMMN6RRY09seLoooJ64QPXgvW8T96P
0my7xtvtmt3h7krsudV68JbgaMotjvzV2vOxgD+s93QQIByIU+mTef5giEshERTL
8cOw4jp99p0wswH9hbn8TQsSf1UPFsL8P3HbD6HiNKKA1YyijgSIEQ6z0H0ALujb
hvweCPpwvOQAXxg+cpumn0bu7oLonWhdy+pkfYEvw/UWNX/7Qd7EIp8v84FI6J0U
jX2iIIBm8rbA+lF10jo7GobPoQ4bGDEQOsNxuUSYvc07HoMpEVTH9Kg8dOZmvRQp
pwyG4/o2+5LWXw8c1+1KNvdlhM8iMrCzz/0gok7UHLvisb3MruZE9c6Ujoua09Tu
shPGfJzXelJiRUwajFFAeBS/TPPBqi8KjFrz+sjYA8rFk7rHZZYW2p1n11Z+SLWj
MwBqQ5yCLohZe5UELdei8h0OuUOgfvnmJWcNk0vhlC1RxzjgUE1ZuQgD+yVbnWEu
XtcpRl5KCY7LnKflxpY5flhLdL0I4pH3coBcWn+87F8TCwxE6xt9Db/ny0Upoupf
iZ1HoCyF0iJj75Duu9Ssr61gR8Gd/R6agXEhi19o517yeK7x+a+UPAinojjROQGD
-----END RSA PRIVATE KEY-----
`

// HTTPMultiBin can be used as a local alternative of httpbin.org. It offers both http and https servers, as well as real domains
type HTTPMultiBin struct {
	Mux             *http.ServeMux
	ServerHTTP      *httptest.Server
	ServerHTTPS     *httptest.Server
	ServerHTTP2     *httptest.Server
	ServerGRPC      *grpc.Server
	GRPCStub        *GRPCStub
	GRPCAnyStub     *GRPCAnyStub
	Replacer        *strings.Replacer
	TLSClientConfig *tls.Config
	Dialer          *netext.Dialer
	HTTPTransport   *http.Transport
	Context         context.Context
}

type jsonBody struct {
	Header      http.Header `json:"headers"`
	Compression string      `json:"compression"`
}

// autocloseHandler handles requests just opening
// then closing the connection without waiting for the client input.
// It simulates the server-side closing operation.
func autocloseHandler(t testing.TB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		require.NoError(t, err)

		err = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
			time.Now().Add(time.Second))
		require.NoError(t, err)

		err = conn.Close()
		require.NoError(t, err)
	})
}

// echoHandler handles requests proxying the same request input to the client.
// If closePrematurely is false then it waits for the client's request to close the connection.
// If closePrematurely is true then it closes the connection in a brutally
// without respecting the protocol.
func echoHandler(t testing.TB, closePrematurely bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, req, w.Header())
		require.NoError(t, err)

		defer func() {
			_ = conn.Close()
		}()

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

		// closePrematurely=true mimics an invalid WS server that doesn't
		// send a close control frame before closing the connection.
		if !closePrematurely {
			// Closing is delegated to the client,
			// it waits the control message for closing.
			closeReceived := make(chan struct{})
			defaultCloseHandler := conn.CloseHandler()
			conn.SetCloseHandler(func(code int, text string) error {
				close(closeReceived)
				return defaultCloseHandler(code, text)
			})

			for {
				_, _, e := conn.ReadMessage()
				if e != nil {
					break
				}
			}
			<-closeReceived
		}
	})
}

func writeJSON(w io.Writer, v interface{}) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	if err := e.Encode(v); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}
	return nil
}

func getEncodedHandler(t testing.TB, compressionType string) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		var (
			encoding string
			err      error
			encw     io.WriteCloser
		)

		switch compressionType {
		case "br":
			encw = brotli.NewWriter(rw)
			encoding = "br"
		case "zstd":
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
		require.NoError(t, err)
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
	grpctest.TestServiceServer
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
	grpctest.TestService_StreamingOutputCallServer,
) error {
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

// GRPCAnyStub is an easily customisable AnyTestServiceServer
type GRPCAnyStub struct {
	grpcanytesting.AnyTestServiceServer
	SumFunc func(context.Context, *grpcanytesting.SumRequest) (*grpcanytesting.SumReply, error)
}

// Sum implements the interface for the gRPC AnyTestServiceServer
func (s *GRPCAnyStub) Sum(ctx context.Context, req *grpcanytesting.SumRequest) (*grpcanytesting.SumReply, error) {
	if s.SumFunc != nil {
		return s.SumFunc(ctx, req)
	}

	return nil, status.Errorf(codes.Unimplemented, "method Sum not implemented")
}

// NewHTTPMultiBin returns a fully configured and running HTTPMultiBin
//
//nolint:funlen
func NewHTTPMultiBin(t testing.TB) *HTTPMultiBin {
	// Create a http.ServeMux and set the httpbin handler as the default
	mux := http.NewServeMux()
	mux.Handle("/brotli", getEncodedHandler(t, "br"))
	mux.Handle("/ws-echo", echoHandler(t, false))
	mux.Handle("/ws-echo-invalid", echoHandler(t, true))
	mux.Handle("/ws-close", autocloseHandler(t))
	mux.Handle("/ws-close-invalid", echoHandler(t, true))
	mux.Handle("/zstd", getEncodedHandler(t, "zstd"))
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
	anyStub := &GRPCAnyStub{}
	grpcanytesting.RegisterAnyTestServiceServer(grpcSrv, anyStub)

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

	httpDomainValue, err := types.NewHost(httpIP, "")
	require.NoError(t, err)
	httpsDomainValue, err := types.NewHost(httpsIP, "")
	require.NoError(t, err)

	// Set up the dialer with shorter timeouts and the custom domains
	dialer := netext.NewDialer(net.Dialer{
		Timeout:   2 * time.Second,
		KeepAlive: 10 * time.Second,
		DualStack: true,
	}, netext.NewResolver(net.LookupIP, 0, types.DNSfirst, types.DNSpreferIPv4))
	dialer.Hosts, err = types.NewHosts(map[string]types.Host{
		httpDomain:  *httpDomainValue,
		httpsDomain: *httpsDomainValue,
	})
	require.NoError(t, err)

	// Pre-configure the HTTP client transport with the dialer and TLS config (incl. HTTP2 support)
	transport := &http.Transport{
		DialContext:     dialer.DialContext,
		TLSClientConfig: tlsConfig,
	}
	require.NoError(t, http2.ConfigureTransport(transport))

	ctx, ctxCancel := context.WithCancel(context.Background())

	result := &HTTPMultiBin{
		Mux:         mux,
		ServerHTTP:  httpSrv,
		ServerHTTPS: httpsSrv,
		ServerHTTP2: http2Srv,
		ServerGRPC:  grpcSrv,
		GRPCStub:    stub,
		GRPCAnyStub: anyStub,
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
			"LOCALHOST_CERT", strings.ReplaceAll(localhostCert, "\n", "\\n"),
			"LOCALHOST_KEY", strings.ReplaceAll(localhostKey, "\n", "\\n"),
			"LOCALHOST_ENCRYPTED_KEY", strings.ReplaceAll(localhostEncryptedKey, "\n", "\\n"),
		),
		TLSClientConfig: tlsConfig,
		Dialer:          dialer,
		HTTPTransport:   transport,
		Context:         ctx,
	}

	t.Cleanup(func() {
		grpcSrv.Stop()
		http2Srv.Close()
		httpsSrv.Close()
		httpSrv.Close()
		ctxCancel()
	})
	return result
}
