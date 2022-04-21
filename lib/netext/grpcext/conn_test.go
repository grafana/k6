package grpcext

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"testing"

	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

func TestInvoke(t *testing.T) {
	t.Parallel()

	helloReply := func(in, out *dynamicpb.Message, _ ...grpc.CallOption) error {
		err := protojson.Unmarshal([]byte(`{"reply":"text reply"}`), out)
		require.NoError(t, err)

		return nil
	}

	c := Conn{raw: invokemock(helloReply)}
	r := Request{
		MethodDescriptor: methodFromProto("SayHello"),
		Message:          []byte(`{"greeting":"text request"}`),
	}
	res, err := c.Invoke(context.Background(), "/hello.HelloService/SayHello", metadata.New(nil), r)
	require.NoError(t, err)

	assert.Equal(t, codes.OK, res.Status)
	assert.Equal(t, map[string]interface{}{"reply": "text reply"}, res.Message)
	assert.Empty(t, res.Error)
}

func TestInvokeWithCallOptions(t *testing.T) {
	t.Parallel()

	reply := func(in, out *dynamicpb.Message, opts ...grpc.CallOption) error {
		assert.Len(t, opts, 3) // two by default plus one injected
		return nil
	}

	c := Conn{raw: invokemock(reply)}
	r := Request{
		MethodDescriptor: methodFromProto("NoOp"),
		Message:          []byte(`{}`),
	}
	res, err := c.Invoke(context.Background(), "/hello.HelloService/NoOp", metadata.New(nil), r, grpc.UseCompressor("fakeone"))
	require.NoError(t, err)
	assert.NotNil(t, res)
}

func TestInvokeReturnError(t *testing.T) {
	t.Parallel()

	helloReply := func(in, out *dynamicpb.Message, _ ...grpc.CallOption) error {
		return fmt.Errorf("test error")
	}

	c := Conn{raw: invokemock(helloReply)}
	r := Request{
		MethodDescriptor: methodFromProto("SayHello"),
		Message:          []byte(`{"greeting":"text request"}`),
	}
	res, err := c.Invoke(context.Background(), "/hello.HelloService/SayHello", metadata.New(nil), r)
	require.NoError(t, err)

	assert.Equal(t, codes.Unknown, res.Status)
	assert.NotEmpty(t, res.Error)
	assert.Equal(t, map[string]interface{}{"reply": ""}, res.Message)
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

	req := Request{
		MethodDescriptor: methodDesc,
		Message:          payload,
	}

	tests := []struct {
		name   string
		ctx    context.Context
		md     metadata.MD
		url    string
		req    Request
		experr string
	}{
		{
			name:   "EmptyMethod",
			ctx:    ctx,
			url:    "",
			md:     md,
			req:    req,
			experr: "url is required",
		},
		{
			name:   "NullMethodDescriptor",
			ctx:    ctx,
			url:    url,
			md:     nil,
			req:    Request{Message: payload},
			experr: "method descriptor is required",
		},
		{
			name:   "NullMessage",
			ctx:    ctx,
			url:    url,
			md:     nil,
			req:    Request{MethodDescriptor: methodDesc},
			experr: "message is required",
		},
		{
			name:   "EmptyMessage",
			ctx:    ctx,
			url:    url,
			md:     nil,
			req:    Request{MethodDescriptor: methodDesc, Message: []byte{}},
			experr: "message is required",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c := Conn{}
			res, err := c.Invoke(tt.ctx, tt.url, tt.md, tt.req)
			require.Error(t, err)
			require.Nil(t, res)
			assert.Contains(t, err.Error(), tt.experr)
		})
	}
}

func TestResolveFileDescriptors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                string
		pkgs                []string
		services            []string
		expectedDescriptors int
	}{
		{
			name:                "SuccessSamePackage",
			pkgs:                []string{"mypkg"},
			services:            []string{"Service1", "Service2", "Service3"},
			expectedDescriptors: 3,
		},
		{
			name:                "SuccessMultiPackages",
			pkgs:                []string{"mypkg1", "mypkg2", "mypkg3"},
			services:            []string{"Service", "Service", "Service"},
			expectedDescriptors: 3,
		},
		{
			name:                "DeduplicateServices",
			pkgs:                []string{"mypkg1"},
			services:            []string{"Service1", "Service2", "Service1"},
			expectedDescriptors: 2,
		},
		{
			name:                "NoServices",
			services:            []string{},
			expectedDescriptors: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var (
				lsr  = &reflectpb.ListServiceResponse{}
				mock = &getServiceFileDescriptorMock{}
			)
			for i, service := range tt.services {
				// if only one package is defined then
				// the package is the same for every service
				pkg := tt.pkgs[0]
				if len(tt.pkgs) > 1 {
					pkg = tt.pkgs[i]
				}

				lsr.Service = append(lsr.Service, &reflectpb.ServiceResponse{
					Name: fmt.Sprintf("%s.%s", pkg, service),
				})
				mock.pkgs = append(mock.pkgs, pkg)
				mock.names = append(mock.names, service)
			}

			rc := reflectionClient{}
			fdset, err := rc.resolveServiceFileDescriptors(mock, lsr)
			require.NoError(t, err)
			assert.Len(t, fdset.File, tt.expectedDescriptors)
		})
	}
}

type getServiceFileDescriptorMock struct {
	pkgs  []string
	names []string
	nreqs int64
}

func (m *getServiceFileDescriptorMock) Send(req *reflectpb.ServerReflectionRequest) error {
	// TODO: check that the sent message is expected,
	// otherwise return an error
	return nil
}

func (m *getServiceFileDescriptorMock) Recv() (*reflectpb.ServerReflectionResponse, error) {
	n := atomic.AddInt64(&m.nreqs, 1)
	ptr := func(s string) (sptr *string) {
		return &s
	}
	index := n - 1
	fdp := &descriptorpb.FileDescriptorProto{
		Package: ptr(m.pkgs[index]),
		Name:    ptr(m.names[index]),
	}
	b, err := proto.Marshal(fdp)
	if err != nil {
		return nil, err
	}
	srr := &reflectpb.ServerReflectionResponse{
		MessageResponse: &reflectpb.ServerReflectionResponse_FileDescriptorResponse{
			FileDescriptorResponse: &reflectpb.FileDescriptorResponse{
				FileDescriptorProto: [][]byte{b},
			},
		},
	}
	return srr, nil
}

func methodFromProto(method string) protoreflect.MethodDescriptor {
	parser := protoparse.Parser{
		InferImportPaths: false,
		Accessor: protoparse.FileAccessor(func(filename string) (io.ReadCloser, error) {
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
		}),
	}

	fds, err := parser.ParseFiles("any-path")
	if err != nil {
		panic(err)
	}

	fd, err := protodesc.NewFile(fds[0].AsFileDescriptorProto(), nil)
	if err != nil {
		panic(err)
	}

	services := fd.Services()
	if services.Len() == 0 {
		panic("no available services")
	}
	return services.Get(0).Methods().ByName(protoreflect.Name(method))
}

// invokemock is a mock for the grpc connection supporting only unary requests.
type invokemock func(in, out *dynamicpb.Message, opts ...grpc.CallOption) error

func (im invokemock) Invoke(ctx context.Context, url string, payload interface{}, reply interface{}, opts ...grpc.CallOption) error {
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

func (invokemock) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	panic("not implemented")
}

func (invokemock) Close() error {
	return nil
}
