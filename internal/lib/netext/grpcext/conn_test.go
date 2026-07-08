package grpcext

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/bufbuild/protocompile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/lib"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const grpcProxyTargetAddr = "grpc.example:443"

type healthcheckmock func(in *healthpb.HealthCheckRequest, out *healthpb.HealthCheckResponse, opts ...grpc.CallOption) error

func (im healthcheckmock) Invoke(_ context.Context, _ string, payload, reply any, opts ...grpc.CallOption) error {
	in, ok := payload.(*healthpb.HealthCheckRequest)
	if !ok {
		return fmt.Errorf("unexpected type for payload")
	}
	out, ok := reply.(*healthpb.HealthCheckResponse)
	if !ok {
		return fmt.Errorf("unexpected type for reply")
	}
	return im(in, out, opts...)
}

func (healthcheckmock) Close() error {
	return nil
}

func (healthcheckmock) NewStream(_ context.Context, _ *grpc.StreamDesc, _ string, _ ...grpc.CallOption) (grpc.ClientStream, error) {
	panic("not implemented")
}

func TestDefaultOptionsUsesHTTPSProxy(t *testing.T) { //nolint:paralleltest // t.Setenv cannot be used in parallel tests.
	proxyAddr := "proxy.example:3128"

	t.Setenv("HTTPS_PROXY", "http://"+proxyAddr)
	t.Setenv("https_proxy", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	proxyReqCh := make(chan proxyRequestResult, 100)
	dialer := &recordingDialer{
		dial: func(_ context.Context, network, addr string) (net.Conn, error) {
			if network != "tcp" {
				return nil, fmt.Errorf("unexpected network %q", network)
			}
			if addr != proxyAddr {
				return nil, fmt.Errorf("unexpected dial to %q", addr)
			}
			return acceptProxyConnect(proxyReqCh), nil
		},
	}
	state := &lib.State{Dialer: dialer}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	opts := append(
		DefaultOptions(func() *lib.State { return state }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	conn, err := Dial(ctx, grpcProxyTargetAddr, new(protoregistry.Types), opts...)
	if conn != nil {
		require.NoError(t, conn.Close())
	}
	require.Error(t, err)

	select {
	case proxyReq := <-proxyReqCh:
		require.NoError(t, proxyReq.err)
		assert.Equal(t, http.MethodConnect, proxyReq.req.Method)
		assert.Equal(t, grpcProxyTargetAddr, proxyReq.req.Host)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for proxy CONNECT request")
	}

	require.NotEmpty(t, dialer.calls())
	assert.Equal(t, proxyAddr, dialer.calls()[0].addr)
}

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	servingsSvc := "serving-service"
	notServingSvc := "not-serving-service"
	healthReply := func(req *healthpb.HealthCheckRequest, out *healthpb.HealthCheckResponse, _ ...grpc.CallOption) error {
		var status healthpb.HealthCheckResponse_ServingStatus
		switch req.Service {
		case "", notServingSvc:
			status = healthpb.HealthCheckResponse_NOT_SERVING
		case servingsSvc:
			status = healthpb.HealthCheckResponse_SERVING
		default:
			status = healthpb.HealthCheckResponse_UNKNOWN
		}

		err := protojson.Unmarshal(fmt.Appendf(nil, `{"status":%d}`, status), out)
		require.NoError(t, err)

		return nil
	}
	c := Conn{raw: healthcheckmock(healthReply)}

	cases := []struct {
		name           string
		svc            string
		expectedStatus healthpb.HealthCheckResponse_ServingStatus
	}{
		{
			name:           "server is not serving",
			svc:            "",
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
		{
			name:           "unknown service",
			svc:            "unknown-service",
			expectedStatus: healthpb.HealthCheckResponse_UNKNOWN,
		},
		{
			name:           "serving service",
			svc:            servingsSvc,
			expectedStatus: healthpb.HealthCheckResponse_SERVING,
		},
		{
			name:           "not serving service",
			svc:            notServingSvc,
			expectedStatus: healthpb.HealthCheckResponse_NOT_SERVING,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			res, err := c.HealthCheck(context.Background(), tc.svc)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedStatus, res.Status)
		})
	}
}

func TestInvoke(t *testing.T) {
	t.Parallel()

	helloReply := func(_, out *dynamicpb.Message, _ ...grpc.CallOption) error {
		err := protojson.Unmarshal([]byte(`{"reply":"text reply"}`), out)
		require.NoError(t, err)

		return nil
	}

	c := Conn{raw: invokemock(helloReply)}
	r := InvokeRequest{
		Method:           "/hello.HelloService/SayHello",
		MethodDescriptor: methodFromProto("SayHello"),
		Message:          []byte(`{"greeting":"text request"}`),
		Metadata:         metadata.New(nil),
	}
	res, err := c.Invoke(context.Background(), r)
	require.NoError(t, err)

	assert.Equal(t, codes.OK, res.Status)
	assert.Equal(t, map[string]any{"reply": "text reply"}, res.Message)
	assert.Empty(t, res.Error)
}

func TestInvokeWithCallOptions(t *testing.T) {
	t.Parallel()

	reply := func(_, _ *dynamicpb.Message, opts ...grpc.CallOption) error {
		assert.Len(t, opts, 3) // two by default plus one injected
		return nil
	}

	c := Conn{raw: invokemock(reply)}
	r := InvokeRequest{
		Method:           "/hello.HelloService/NoOp",
		MethodDescriptor: methodFromProto("NoOp"),
		Message:          []byte(`{}`),
		Metadata:         metadata.New(nil),
	}
	res, err := c.Invoke(context.Background(), r, grpc.UseCompressor("fakeone"))
	require.NoError(t, err)
	assert.NotNil(t, res)
}

func TestInvokeWithDiscardResponseMessage(t *testing.T) {
	t.Parallel()

	reply := func(_, _ *dynamicpb.Message, opts ...grpc.CallOption) error {
		assert.Len(t, opts, 3) // two by default plus one injected
		return nil
	}

	c := Conn{raw: invokemock(reply)}
	r := InvokeRequest{
		Method:                 "/hello.HelloService/NoOp",
		MethodDescriptor:       methodFromProto("NoOp"),
		DiscardResponseMessage: true,
		Message:                []byte(`{}`),
		Metadata:               metadata.New(nil),
	}
	res, err := c.Invoke(context.Background(), r, grpc.UseCompressor("fakeone"))
	require.NoError(t, err)
	assert.NotNil(t, res)
	assert.Nil(t, res.Message)
}

func TestInvokeReturnError(t *testing.T) {
	t.Parallel()

	helloReply := func(_, _ *dynamicpb.Message, _ ...grpc.CallOption) error {
		return fmt.Errorf("test error")
	}

	c := Conn{raw: invokemock(helloReply)}
	r := InvokeRequest{
		Method:           "/hello.HelloService/SayHello",
		MethodDescriptor: methodFromProto("SayHello"),
		Message:          []byte(`{"greeting":"text request"}`),
		Metadata:         metadata.New(nil),
	}
	res, err := c.Invoke(context.Background(), r)
	require.NoError(t, err)

	assert.Equal(t, codes.Unknown, res.Status)
	assert.NotEmpty(t, res.Error)
	assert.Equal(t, map[string]any{"reply": ""}, res.Message)
}

func TestConnInvokeInvalid(t *testing.T) {
	t.Parallel()

	var (
		// valid arguments
		ctx        = context.Background()
		url        = "not-empty-url-for-method"
		md         = metadata.New(nil)
		methodDesc = methodFromProto("SayHello")
		payload    = []byte(`{"greeting":"test"}`)
	)

	tests := []struct {
		name   string
		ctx    context.Context
		req    InvokeRequest
		experr string
	}{
		{
			name:   "EmptyMethod",
			ctx:    ctx,
			req:    InvokeRequest{MethodDescriptor: methodDesc, Message: payload, Metadata: md, Method: ""},
			experr: "url is required",
		},
		{
			name:   "NullMethodDescriptor",
			ctx:    ctx,
			req:    InvokeRequest{Message: payload, Metadata: nil, Method: url},
			experr: "method descriptor is required",
		},
		{
			name:   "NullMessage",
			ctx:    ctx,
			req:    InvokeRequest{MethodDescriptor: methodDesc, Metadata: nil, Method: url},
			experr: "message is required",
		},
		{
			name:   "EmptyMessage",
			ctx:    ctx,
			req:    InvokeRequest{MethodDescriptor: methodDesc, Message: []byte{}, Metadata: nil, Method: url},
			experr: "message is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := Conn{}
			res, err := c.Invoke(tt.ctx, tt.req)
			require.Error(t, err)
			require.Nil(t, res)
			assert.Contains(t, err.Error(), tt.experr)
		})
	}
}

func methodFromProto(method string) protoreflect.MethodDescriptor {
	path := "any-path"

	resolver := &protocompile.SourceResolver{
		Accessor: func(filename string) (io.ReadCloser, error) {
			// a small hack to make sure we are parsing the right file
			// otherwise the parser will try to parse "google/protobuf/descriptor.proto"
			// with exactly the same name as the one we are trying to parse for testing
			if filename != path {
				return nil, nil //nolint:nilnil
			}

			b := `
syntax = "proto3";

package hello;

service HelloService {
  rpc SayHello(HelloRequest) returns (HelloResponse);
  rpc NoOp(Empty) returns (Empty);
  rpc LotsOfReplies(HelloRequest) returns (stream HelloResponse);
  rpc LotsOfGreetings(stream HelloRequest) returns (HelloResponse);
  rpc BidiHello(stream HelloRequest) returns (stream HelloResponse);
}

message HelloRequest {
  string greeting = 1;
}

message HelloResponse {
  string reply = 1;
}

message Empty {

}`

			return io.NopCloser(bytes.NewBufferString(b)), nil
		},
	}

	compiler := protocompile.Compiler{
		Resolver: resolver,
	}

	fds, err := compiler.Compile(context.Background(), path)
	if err != nil {
		panic(err)
	}
	if len(fds) != 1 {
		panic("expected exactly one file descriptor")
	}
	fd := fds[0]

	services := fd.Services()
	if services.Len() == 0 {
		panic("no available services")
	}
	return services.Get(0).Methods().ByName(protoreflect.Name(method))
}

// invokemock is a mock for the grpc connection supporting only unary requests.
type invokemock func(in, out *dynamicpb.Message, opts ...grpc.CallOption) error

func (im invokemock) Invoke(_ context.Context, _ string, payload any, reply any, opts ...grpc.CallOption) error {
	in, ok := payload.(*dynamicpb.Message)
	if !ok {
		return fmt.Errorf("unexpected type for payload")
	}
	out, ok := reply.(*dynamicpb.Message)
	if !ok {
		return fmt.Errorf("unexpected type for reply")
	}
	return im(in, out, opts...)
}

func (invokemock) NewStream(_ context.Context, _ *grpc.StreamDesc, _ string, _ ...grpc.CallOption) (grpc.ClientStream, error) {
	panic("not implemented")
}

func (invokemock) Close() error {
	return nil
}

func TestDialGRPCContextUsesHTTPSProxy(t *testing.T) {
	t.Parallel()

	directAddr := "direct.example:443"
	proxyAddr := "proxy.example:80"
	proxyURL, err := url.Parse("http://user:pass@proxy.example")
	require.NoError(t, err)

	proxyFromEnvironment := func(req *http.Request) (*url.URL, error) {
		assert.Equal(t, "https", req.URL.Scheme)

		switch req.URL.Host {
		case grpcProxyTargetAddr:
			return proxyURL, nil
		case directAddr:
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected proxy lookup for %q", req.URL.Host)
		}
	}

	proxyReqCh := make(chan proxyRequestResult, 1)
	dialer := &recordingDialer{
		dial: func(_ context.Context, network, addr string) (net.Conn, error) {
			if network != "tcp" {
				return nil, fmt.Errorf("unexpected network %q", network)
			}
			switch addr {
			case proxyAddr:
				return acceptProxyConnect(proxyReqCh), nil
			case directAddr:
				clientConn, serverConn := net.Pipe()
				_ = serverConn.Close()
				return clientConn, nil
			default:
				return nil, fmt.Errorf("unexpected dial to %q", addr)
			}
		},
	}
	state := &lib.State{Dialer: dialer}

	proxyConn, err := dialGRPCContextWithProxy(context.Background(), state, grpcProxyTargetAddr, proxyFromEnvironment)
	require.NoError(t, err)
	require.NoError(t, proxyConn.Close())

	proxyReq := <-proxyReqCh
	require.NoError(t, proxyReq.err)
	assert.Equal(t, http.MethodConnect, proxyReq.req.Method)
	assert.Equal(t, grpcProxyTargetAddr, proxyReq.req.Host)
	assert.Equal(t, grpcProxyTargetAddr, proxyReq.req.URL.Host)
	assert.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("user:pass")),
		proxyReq.req.Header.Get("Proxy-Authorization"))

	directConn, err := dialGRPCContextWithProxy(context.Background(), state, directAddr, proxyFromEnvironment)
	require.NoError(t, err)
	require.NoError(t, directConn.Close())

	assert.Equal(t, []dialCall{
		{network: "tcp", addr: proxyAddr},
		{network: "tcp", addr: directAddr},
	}, dialer.calls())
}

func TestDialGRPCContextRejectsUnsupportedProxyScheme(t *testing.T) {
	t.Parallel()

	proxyURL, err := url.Parse("https://token:secret@proxy.example:443")
	require.NoError(t, err)

	proxyFromEnvironment := func(*http.Request) (*url.URL, error) {
		return proxyURL, nil
	}

	dialer := &recordingDialer{
		dial: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("dial should not be called")
		},
	}
	state := &lib.State{Dialer: dialer}

	conn, err := dialGRPCContextWithProxy(context.Background(), state, grpcProxyTargetAddr, proxyFromEnvironment)
	require.Error(t, err)
	require.Nil(t, conn)
	assert.Contains(t, err.Error(), `unsupported grpc proxy scheme "https"`)
	assert.NotContains(t, err.Error(), "token")
	assert.NotContains(t, err.Error(), "secret")
	assert.Empty(t, dialer.calls())
}

func TestHTTPConnectHandshakeStopsOnContextCancellation(t *testing.T) {
	t.Parallel()

	proxyReqCh := make(chan proxyRequestResult, 1)
	releaseProxy := make(chan struct{})
	defer close(releaseProxy)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	conn, err := doHTTPConnectHandshake(
		ctx,
		acceptStalledProxyConnect(proxyReqCh, releaseProxy),
		grpcProxyTargetAddr,
		nil,
	)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, conn)
	assert.Less(t, time.Since(start), time.Second)

	proxyReq := <-proxyReqCh
	require.NoError(t, proxyReq.err)
	assert.Equal(t, http.MethodConnect, proxyReq.req.Method)
}

func TestHTTPConnectHandshakeDoesNotDrainSuccessfulTunnel(t *testing.T) {
	t.Parallel()

	proxyReqCh := make(chan proxyRequestResult, 1)
	releaseProxy := make(chan struct{})
	defer close(releaseProxy)

	type handshakeResult struct {
		conn net.Conn
		err  error
	}
	resultCh := make(chan handshakeResult, 1)
	go func() {
		conn, err := doHTTPConnectHandshake(
			context.Background(),
			acceptProxyConnectWithBodyHeader(proxyReqCh, releaseProxy),
			grpcProxyTargetAddr,
			nil,
		)
		resultCh <- handshakeResult{conn: conn, err: err}
	}()

	var result handshakeResult
	select {
	case result = <-resultCh:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for CONNECT handshake to return")
	}
	require.NoError(t, result.err)
	require.NotNil(t, result.conn)
	require.NoError(t, result.conn.Close())

	proxyReq := <-proxyReqCh
	require.NoError(t, proxyReq.err)
	assert.Equal(t, http.MethodConnect, proxyReq.req.Method)
}

type proxyRequestResult struct {
	req *http.Request
	err error
}

func acceptProxyConnect(proxyReqCh chan<- proxyRequestResult) net.Conn {
	clientConn, serverConn := net.Pipe()
	go func() {
		defer func() { _ = serverConn.Close() }()

		req, err := http.ReadRequest(bufio.NewReader(serverConn))
		if err == nil {
			_, err = serverConn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
		}
		proxyReqCh <- proxyRequestResult{req: req, err: err}
	}()
	return clientConn
}

func acceptProxyConnectWithBodyHeader(proxyReqCh chan<- proxyRequestResult, release <-chan struct{}) net.Conn {
	clientConn, serverConn := net.Pipe()
	go func() {
		defer func() { _ = serverConn.Close() }()

		req, err := http.ReadRequest(bufio.NewReader(serverConn))
		if err == nil {
			_, err = serverConn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 1\r\n\r\n"))
		}
		proxyReqCh <- proxyRequestResult{req: req, err: err}
		<-release
	}()
	return clientConn
}

func acceptStalledProxyConnect(proxyReqCh chan<- proxyRequestResult, release <-chan struct{}) net.Conn {
	clientConn, serverConn := net.Pipe()
	go func() {
		defer func() { _ = serverConn.Close() }()

		req, err := http.ReadRequest(bufio.NewReader(serverConn))
		proxyReqCh <- proxyRequestResult{req: req, err: err}
		<-release
	}()
	return clientConn
}

type dialCall struct {
	network string
	addr    string
}

type recordingDialer struct {
	mu    sync.Mutex
	dials []dialCall
	dial  func(context.Context, string, string) (net.Conn, error)
}

func (d *recordingDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	d.mu.Lock()
	d.dials = append(d.dials, dialCall{network: network, addr: addr})
	d.mu.Unlock()

	return d.dial(ctx, network, addr)
}

func (d *recordingDialer) calls() []dialCall {
	d.mu.Lock()
	defer d.mu.Unlock()

	calls := make([]dialCall, len(d.dials))
	copy(calls, d.dials)
	return calls
}
