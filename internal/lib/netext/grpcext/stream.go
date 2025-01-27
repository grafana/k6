package grpcext

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Stream is the wrapper around the grpc.ClientStream
// with some handy methods.
type Stream struct {
	method                 string
	methodDescriptor       protoreflect.MethodDescriptor
	discardResponseMessage bool
	raw                    grpc.ClientStream
	marshaler              protojson.MarshalOptions
}

// ErrCanceled canceled by client (k6)
var ErrCanceled = errors.New("canceled by client (k6)")

// ReceiveConverted receives a converted message from the stream
// if the stream has been closed successfully, it returns io.EOF
// if the stream has been cancelled, it returns ErrCanceled
func (s *Stream) ReceiveConverted() (interface{}, error) {
	raw, err := s.receive()
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}

	if s.discardResponseMessage {
		return struct{}{}, err
	}

	msg, errConv := convert(s.marshaler, raw)
	if errConv != nil {
		return nil, errConv
	}

	return msg, err
}

func (s *Stream) receive() (msg *dynamicpb.Message, err error) {
	if s.discardResponseMessage {
		msg = dynamicpb.NewMessage((&emptypb.Empty{}).ProtoReflect().Descriptor())
	} else {
		msg = dynamicpb.NewMessage(s.methodDescriptor.Output())
	}

	err = s.raw.RecvMsg(msg)

	// io.EOF means that the stream has been closed successfully
	if err == nil || errors.Is(err, io.EOF) {
		return msg, err
	}

	sterr := status.Convert(err)
	if sterr.Code() == codes.Canceled {
		return nil, ErrCanceled
	}

	return nil, err
}

// convert converts the message to the interface{}
// which could be returned to the JS
// there is a lot of marshaling/unmarshaling here, but if we just pass the dynamic message
// the default Marshaller would be used, which would strip any zero/default values from the JSON.
// eg. given this message:
//
//	message Point {
//	   double x = 1;
//		  double y = 2;
//		  double z = 3;
//	}
//
// and a value like this:
// msg := Point{X: 6, Y: 4, Z: 0}
// would result in JSON output:
// {"x":6,"y":4}
// rather than the desired:
// {"x":6,"y":4,"z":0}
func convert(marshaler protojson.MarshalOptions, msg *dynamicpb.Message) (interface{}, error) {
	// TODO(olegbespalov): add the test that checks that message is not nil

	raw, err := marshaler.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the message: %w", err)
	}

	var back interface{}

	err = json.Unmarshal(raw, &back)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal the message: %w", err)
	}

	return back, err
}

// CloseSend closes the stream
func (s *Stream) CloseSend() error {
	return s.raw.CloseSend()
}

// BuildMessage builds a message from the input
func (s *Stream) buildMessage(b []byte) (*dynamicpb.Message, error) {
	msg := dynamicpb.NewMessage(s.methodDescriptor.Input())
	if err := protojson.Unmarshal(b, msg); err != nil {
		return nil, fmt.Errorf("can't serialise request object to protocol buffer: %w", err)
	}

	return msg, nil
}

// Send sends the message to the stream
func (s *Stream) Send(b []byte) error {
	msg, err := s.buildMessage(b)
	if err != nil {
		return err
	}

	return s.raw.SendMsg(msg)
}
