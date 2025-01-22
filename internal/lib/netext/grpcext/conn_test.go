package grpcext

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/jhump/protoreflect/desc/protoparse" //nolint:staticcheck // FIXME: #4035
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

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
	assert.Equal(t, map[string]interface{}{"reply": "text reply"}, res.Message)
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
		tt := tt
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
	parser := protoparse.Parser{
		InferImportPaths: false,
		Accessor: protoparse.FileAccessor(func(filename string) (io.ReadCloser, error) {
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
		}),
	}

	fds, err := parser.ParseFiles(path)
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

func (im invokemock) Invoke(_ context.Context, _ string, payload interface{}, reply interface{}, opts ...grpc.CallOption) error {
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
