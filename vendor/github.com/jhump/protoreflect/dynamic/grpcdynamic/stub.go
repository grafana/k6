// Package grpcdynamic provides a dynamic RPC stub. It can be used to invoke RPC
// method where only method descriptors are known. The actual request and response
// messages may be dynamic messages.
package grpcdynamic

import (
	"fmt"
	"io"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
)

// Stub is an RPC client stub, used for dynamically dispatching RPCs to a server.
type Stub struct {
	channel Channel
	mf      *dynamic.MessageFactory
}

// Channel represents the operations necessary to issue RPCs via gRPC. The
// *grpc.ClientConn type provides this interface and will typically the concrete
// type used to construct Stubs. But the use of this interface allows
// construction of stubs that use alternate concrete types as the transport for
// RPC operations.
type Channel interface {
	Invoke(ctx context.Context, method string, args, reply interface{}, opts ...grpc.CallOption) error
	NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error)
}

var _ Channel = (*grpc.ClientConn)(nil)

// NewStub creates a new RPC stub that uses the given channel for dispatching RPCs.
func NewStub(channel Channel) Stub {
	return NewStubWithMessageFactory(channel, nil)
}

// NewStubWithMessageFactory creates a new RPC stub that uses the given channel for
// dispatching RPCs and the given MessageFactory for creating response messages.
func NewStubWithMessageFactory(channel Channel, mf *dynamic.MessageFactory) Stub {
	return Stub{channel: channel, mf: mf}
}

func requestMethod(md *desc.MethodDescriptor) string {
	return fmt.Sprintf("/%s/%s", md.GetService().GetFullyQualifiedName(), md.GetName())
}

// InvokeRpc sends a unary RPC and returns the response. Use this for unary methods.
func (s Stub) InvokeRpc(ctx context.Context, method *desc.MethodDescriptor, request proto.Message, opts ...grpc.CallOption) (proto.Message, error) {
	if method.IsClientStreaming() || method.IsServerStreaming() {
		return nil, fmt.Errorf("InvokeRpc is for unary methods; %q is %s", method.GetFullyQualifiedName(), methodType(method))
	}
	if err := checkMessageType(method.GetInputType(), request); err != nil {
		return nil, err
	}
	resp := s.mf.NewMessage(method.GetOutputType())
	if err := s.channel.Invoke(ctx, requestMethod(method), request, resp, opts...); err != nil {
		return nil, err
	}
	return resp, nil
}

// InvokeRpcServerStream sends a unary RPC and returns the response stream. Use this for server-streaming methods.
func (s Stub) InvokeRpcServerStream(ctx context.Context, method *desc.MethodDescriptor, request proto.Message, opts ...grpc.CallOption) (*ServerStream, error) {
	if method.IsClientStreaming() || !method.IsServerStreaming() {
		return nil, fmt.Errorf("InvokeRpcServerStream is for server-streaming methods; %q is %s", method.GetFullyQualifiedName(), methodType(method))
	}
	if err := checkMessageType(method.GetInputType(), request); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	sd := grpc.StreamDesc{
		StreamName:    method.GetName(),
		ServerStreams: method.IsServerStreaming(),
		ClientStreams: method.IsClientStreaming(),
	}
	if cs, err := s.channel.NewStream(ctx, &sd, requestMethod(method), opts...); err != nil {
		return nil, err
	} else {
		err = cs.SendMsg(request)
		if err != nil {
			cancel()
			return nil, err
		}
		err = cs.CloseSend()
		if err != nil {
			cancel()
			return nil, err
		}
		return &ServerStream{cs, method.GetOutputType(), s.mf}, nil
	}
}

// InvokeRpcClientStream creates a new stream that is used to send request messages and, at the end,
// receive the response message. Use this for client-streaming methods.
func (s Stub) InvokeRpcClientStream(ctx context.Context, method *desc.MethodDescriptor, opts ...grpc.CallOption) (*ClientStream, error) {
	if !method.IsClientStreaming() || method.IsServerStreaming() {
		return nil, fmt.Errorf("InvokeRpcClientStream is for client-streaming methods; %q is %s", method.GetFullyQualifiedName(), methodType(method))
	}
	ctx, cancel := context.WithCancel(ctx)
	sd := grpc.StreamDesc{
		StreamName:    method.GetName(),
		ServerStreams: method.IsServerStreaming(),
		ClientStreams: method.IsClientStreaming(),
	}
	if cs, err := s.channel.NewStream(ctx, &sd, requestMethod(method), opts...); err != nil {
		return nil, err
	} else {
		return &ClientStream{cs, method, s.mf, cancel}, nil
	}
}

// InvokeRpcBidiStream creates a new stream that is used to both send request messages and receive response
// messages. Use this for bidi-streaming methods.
func (s Stub) InvokeRpcBidiStream(ctx context.Context, method *desc.MethodDescriptor, opts ...grpc.CallOption) (*BidiStream, error) {
	if !method.IsClientStreaming() || !method.IsServerStreaming() {
		return nil, fmt.Errorf("InvokeRpcBidiStream is for bidi-streaming methods; %q is %s", method.GetFullyQualifiedName(), methodType(method))
	}
	sd := grpc.StreamDesc{
		StreamName:    method.GetName(),
		ServerStreams: method.IsServerStreaming(),
		ClientStreams: method.IsClientStreaming(),
	}
	if cs, err := s.channel.NewStream(ctx, &sd, requestMethod(method), opts...); err != nil {
		return nil, err
	} else {
		return &BidiStream{cs, method.GetInputType(), method.GetOutputType(), s.mf}, nil
	}
}

func methodType(md *desc.MethodDescriptor) string {
	if md.IsClientStreaming() && md.IsServerStreaming() {
		return "bidi-streaming"
	} else if md.IsClientStreaming() {
		return "client-streaming"
	} else if md.IsServerStreaming() {
		return "server-streaming"
	} else {
		return "unary"
	}
}

func checkMessageType(md *desc.MessageDescriptor, msg proto.Message) error {
	var typeName string
	if dm, ok := msg.(*dynamic.Message); ok {
		typeName = dm.GetMessageDescriptor().GetFullyQualifiedName()
	} else {
		typeName = proto.MessageName(msg)
	}
	if typeName != md.GetFullyQualifiedName() {
		return fmt.Errorf("expecting message of type %s; got %s", md.GetFullyQualifiedName(), typeName)
	}
	return nil
}

// ServerStream represents a response stream from a server. Messages in the stream can be queried
// as can header and trailer metadata sent by the server.
type ServerStream struct {
	stream   grpc.ClientStream
	respType *desc.MessageDescriptor
	mf       *dynamic.MessageFactory
}

// Header returns any header metadata sent by the server (blocks if necessary until headers are
// received).
func (s *ServerStream) Header() (metadata.MD, error) {
	return s.stream.Header()
}

// Trailer returns the trailer metadata sent by the server. It must only be called after
// RecvMsg returns a non-nil error (which may be EOF for normal completion of stream).
func (s *ServerStream) Trailer() metadata.MD {
	return s.stream.Trailer()
}

// Context returns the context associated with this streaming operation.
func (s *ServerStream) Context() context.Context {
	return s.stream.Context()
}

// RecvMsg returns the next message in the response stream or an error. If the stream
// has completed normally, the error is io.EOF. Otherwise, the error indicates the
// nature of the abnormal termination of the stream.
func (s *ServerStream) RecvMsg() (proto.Message, error) {
	resp := s.mf.NewMessage(s.respType)
	if err := s.stream.RecvMsg(resp); err != nil {
		return nil, err
	} else {
		return resp, nil
	}
}

// ClientStream represents a response stream from a client. Messages in the stream can be sent
// and, when done, the unary server message and header and trailer metadata can be queried.
type ClientStream struct {
	stream grpc.ClientStream
	method *desc.MethodDescriptor
	mf     *dynamic.MessageFactory
	cancel context.CancelFunc
}

// Header returns any header metadata sent by the server (blocks if necessary until headers are
// received).
func (s *ClientStream) Header() (metadata.MD, error) {
	return s.stream.Header()
}

// Trailer returns the trailer metadata sent by the server. It must only be called after
// RecvMsg returns a non-nil error (which may be EOF for normal completion of stream).
func (s *ClientStream) Trailer() metadata.MD {
	return s.stream.Trailer()
}

// Context returns the context associated with this streaming operation.
func (s *ClientStream) Context() context.Context {
	return s.stream.Context()
}

// SendMsg sends a request message to the server.
func (s *ClientStream) SendMsg(m proto.Message) error {
	if err := checkMessageType(s.method.GetInputType(), m); err != nil {
		return err
	}
	return s.stream.SendMsg(m)
}

// CloseAndReceive closes the outgoing request stream and then blocks for the server's response.
func (s *ClientStream) CloseAndReceive() (proto.Message, error) {
	if err := s.stream.CloseSend(); err != nil {
		return nil, err
	}
	resp := s.mf.NewMessage(s.method.GetOutputType())
	if err := s.stream.RecvMsg(resp); err != nil {
		return nil, err
	}
	// make sure we get EOF for a second message
	if err := s.stream.RecvMsg(resp); err != io.EOF {
		if err == nil {
			s.cancel()
			return nil, fmt.Errorf("client-streaming method %q returned more than one response message", s.method.GetFullyQualifiedName())
		} else {
			return nil, err
		}
	}
	return resp, nil
}

// BidiStream represents a bi-directional stream for sending messages to and receiving
// messages from a server. The header and trailer metadata sent by the server can also be
// queried.
type BidiStream struct {
	stream   grpc.ClientStream
	reqType  *desc.MessageDescriptor
	respType *desc.MessageDescriptor
	mf       *dynamic.MessageFactory
}

// Header returns any header metadata sent by the server (blocks if necessary until headers are
// received).
func (s *BidiStream) Header() (metadata.MD, error) {
	return s.stream.Header()
}

// Trailer returns the trailer metadata sent by the server. It must only be called after
// RecvMsg returns a non-nil error (which may be EOF for normal completion of stream).
func (s *BidiStream) Trailer() metadata.MD {
	return s.stream.Trailer()
}

// Context returns the context associated with this streaming operation.
func (s *BidiStream) Context() context.Context {
	return s.stream.Context()
}

// SendMsg sends a request message to the server.
func (s *BidiStream) SendMsg(m proto.Message) error {
	if err := checkMessageType(s.reqType, m); err != nil {
		return err
	}
	return s.stream.SendMsg(m)
}

// CloseSend indicates the request stream has ended. Invoke this after all request messages
// are sent (even if there are zero such messages).
func (s *BidiStream) CloseSend() error {
	return s.stream.CloseSend()
}

// RecvMsg returns the next message in the response stream or an error. If the stream
// has completed normally, the error is io.EOF. Otherwise, the error indicates the
// nature of the abnormal termination of the stream.
func (s *BidiStream) RecvMsg() (proto.Message, error) {
	resp := s.mf.NewMessage(s.respType)
	if err := s.stream.RecvMsg(resp); err != nil {
		return nil, err
	} else {
		return resp, nil
	}
}
