// Package dynamic implements a dynamic grpc client
// It requires the server to be registered to the grpc reflection Service
package dynamic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/grpcreflect"
	"google.golang.org/grpc"
)

// Client represents a client to a grpc service
type Client struct {
	target     string
	service    string
	descriptor *desc.ServiceDescriptor
	dialOps    []grpc.DialOption
}

// NewClient creates a new Client given a target and a service name with defaults connection options
func NewClient(target string, service string) (*Client, error) {
	client := &Client{
		target:  target,
		service: service,
		dialOps: []grpc.DialOption{},
	}

	return client, nil
}

// NewClientWithDialOptions creates a new Client given a target and a service name using the  grpc.DialOptions
func NewClientWithDialOptions(target string, service string, opts ...grpc.DialOption) (*Client, error) {
	client := &Client{
		target:  target,
		service: service,
		dialOps: opts,
	}

	return client, nil
}

// Connect connects the client to the service
func (c *Client) Connect(ctx context.Context) error {
	if c.descriptor != nil {
		return fmt.Errorf("client already connected")
	}

	conn, err := grpc.DialContext(
		ctx,
		c.target,
		c.dialOps...,
	)
	if err != nil {
		return err
	}
	rc := grpcreflect.NewClientAuto(ctx, conn)
	defer rc.Reset()

	desc, err := rc.ResolveService(c.service)
	if err != nil {
		return err
	}

	c.descriptor = desc
	return nil
}

// Invoke invokes a method in the service
func (c *Client) Invoke(
	ctx context.Context,
	method string,
	input [][]byte,
	callOpts ...grpc.CallOption,
) ([]byte, error) {
	if c.descriptor == nil {
		return nil, fmt.Errorf("client is not connected")
	}
	methodDesc, err := c.findMethod(method)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.DialContext(
		ctx,
		c.target,
		c.dialOps...,
	)
	if err != nil {
		return nil, err
	}

	sd := grpc.StreamDesc{
		StreamName:    methodDesc.GetName(),
		ServerStreams: methodDesc.IsServerStreaming(),
		ClientStreams: methodDesc.IsClientStreaming(),
	}

	methodName := fmt.Sprintf("/%s/%s", methodDesc.GetService().GetFullyQualifiedName(), methodDesc.GetName())
	stream, err := conn.NewStream(ctx, &sd, methodName, callOpts...)
	if err != nil {
		return nil, err
	}

	resultChan := make(chan []byte)
	errChan := make(chan error, 1)
	go c.sendToServer(stream, methodDesc.GetInputType(), input, errChan)
	go c.readFromServer(stream, methodDesc.GetOutputType(), resultChan, errChan)

	for {
		select {
		case result := <-resultChan:
			return result, nil
		case err := <-errChan:
			return nil, err
		}
	}
}

func (c *Client) findMethod(method string) (*desc.MethodDescriptor, error) {
	for _, m := range c.descriptor.GetMethods() {
		if strings.EqualFold(method, m.GetName()) {
			return m, nil
		}
	}
	return nil, fmt.Errorf("method not found %s", method)
}

func (c *Client) sendToServer(
	stream grpc.ClientStream,
	inputType *desc.MessageDescriptor,
	input [][]byte,
	errChan chan error,
) {
	defer func() {
		if serr := stream.CloseSend(); serr != nil {
			errChan <- serr
		}
	}()

	for _, message := range input {
		msg := dynamic.NewMessage(inputType)
		err := c.unmarshalMessage(msg, message)
		if err != nil {
			errChan <- fmt.Errorf("invalid input: %w", err)
			return
		}

		err = stream.SendMsg(msg)
		if err != nil && !errors.Is(err, io.EOF) {
			errChan <- err
			return
		}
	}
}

func (c *Client) readFromServer(
	stream grpc.ClientStream,
	outputType *desc.MessageDescriptor,
	result chan []byte,
	errChan chan error,
) {
	for {
		m := dynamic.NewMessage(outputType)
		err := stream.RecvMsg(m)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				errChan <- err
			} else {
				close(errChan)
			}

			close(result)
			break
		}

		resMsg, err := c.marshalMessage(m)
		if err != nil {
			errChan <- err
			close(result)
			break
		}
		result <- resMsg
	}
}

func (c *Client) marshalMessage(msg *dynamic.Message) ([]byte, error) {
	return msg.MarshalJSONPB(&jsonpb.Marshaler{
		EmitDefaults: true,
		Indent:       "  ",
		OrigName:     false,
		AnyResolver:  nil,
	})
}

func (c *Client) unmarshalMessage(msg *dynamic.Message, buffer []byte) error {
	return msg.UnmarshalJSONPB(
		&jsonpb.Unmarshaler{
			AnyResolver: nil,
		},
		buffer,
	)
}
