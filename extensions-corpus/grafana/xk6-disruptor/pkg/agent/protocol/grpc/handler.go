package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func clientStreamDescForProxy() *grpc.StreamDesc {
	return &grpc.StreamDesc{
		ServerStreams: true,
		ClientStreams: true,
	}
}

// NewHandler returns a StreamHandler that attempts to proxy all requests that are not registered in the server.
func NewHandler(disruption Disruption, forwardConn *grpc.ClientConn, metrics *protocol.MetricMap) grpc.StreamHandler {
	handler := &handler{
		disruption:  disruption,
		forwardConn: forwardConn,
		metrics:     metrics,
	}

	// return the handler function
	return handler.streamHandler
}

type handler struct {
	disruption  Disruption
	forwardConn *grpc.ClientConn
	metrics     *protocol.MetricMap
}

// contains verifies if a list of strings contains the given string
func contains(list []string, target string) bool {
	for _, element := range list {
		if element == target {
			return true
		}
	}
	return false
}

// handles requests from the client. If selected for error injection, returns an error,
// otherwise, forwards to the server transparently
func (h *handler) streamHandler(_ interface{}, serverStream grpc.ServerStream) error {
	h.metrics.Inc(protocol.MetricRequests)

	fullMethodName, ok := grpc.MethodFromServerStream(serverStream)
	if !ok {
		return status.Errorf(codes.Internal, "ServerTransportStream not exists in context")
	}

	// full method name has the form /service/method, we want the service
	serviceName := strings.Split(fullMethodName, "/")[1]
	if contains(h.disruption.Excluded, serviceName) {
		h.metrics.Inc(protocol.MetricRequestsExcluded)
		return h.transparentForward(serverStream)
	}

	if rand.Float32() < h.disruption.ErrorRate {
		h.metrics.Inc(protocol.MetricRequestsDisrupted)
		return h.injectError(serverStream)
	}

	// add delay
	if h.disruption.AverageDelay > 0 {
		h.metrics.Inc(protocol.MetricRequestsDisrupted)

		delay := int64(h.disruption.AverageDelay)
		if h.disruption.DelayVariation > 0 {
			variation := int64(h.disruption.DelayVariation)
			delay = delay + variation - 2*rand.Int63n(variation)
		}
		time.Sleep(time.Duration(delay))
	}

	return h.transparentForward(serverStream)
}

func (h *handler) transparentForward(serverStream grpc.ServerStream) error {
	// TODO: Add a `forwarded` header to metadata, https://en.wikipedia.org/wiki/X-Forwarded-For.
	ctx := serverStream.Context()
	md, _ := metadata.FromIncomingContext(ctx)
	outgoingCtx := metadata.NewOutgoingContext(ctx, md.Copy())
	clientCtx, clientCancel := context.WithCancel(outgoingCtx)
	defer clientCancel()
	fullMethodName, ok := grpc.MethodFromServerStream(serverStream)
	if !ok {
		return status.Errorf(codes.Internal, "ServerTransportStream not exists in context")
	}

	clientStream, err := grpc.NewClientStream(
		clientCtx,
		clientStreamDescForProxy(),
		h.forwardConn,
		fullMethodName,
	)
	if err != nil {
		return err
	}

	// Explicitly *do not close* s2cErrChan and c2sErrChan, otherwise the select below will not terminate.
	// Channels do not have to be closed, it is just a control flow mechanism, see
	// https://groups.google.com/forum/#!msg/golang-nuts/pZwdYRGxCIk/qpbHxRRPJdUJ
	s2cErrChan := h.forwardServerToClient(serverStream, clientStream)
	c2sErrChan := h.forwardClientToServer(clientStream, serverStream)
	// We don't know which side is going to stop sending first, so we need a select between the two.
	for range 2 {
		select {
		case s2cErr := <-s2cErrChan:
			if errors.Is(s2cErr, io.EOF) {
				// this is the happy case where the sender has encountered io.EOF, and won't be sending anymore./
				// the clientStream>serverStream may continue pumping though.
				_ = clientStream.CloseSend()
			} else {
				// however, we may have gotten a receive error (stream disconnected, a read error etc) in which case we need
				// to cancel the clientStream to the backend, let all of its goroutines be freed up by the CancelFunc and
				// exit with an error to the stack
				clientCancel()
				return status.Errorf(codes.Internal, "failed forwarding response to client: %v", s2cErr)
			}
		case c2sErr := <-c2sErrChan:
			// This happens when the clientStream has nothing else to offer (io.EOF), returned a gRPC error. In those two
			// cases we may have received Trailers as part of the call. In case of other errors (stream closed) the trailers
			// will be nil.
			serverStream.SetTrailer(clientStream.Trailer())
			// c2sErr will contain RPC error from client code. If not io.EOF return the RPC error as server stream error.
			if !errors.Is(c2sErr, io.EOF) {
				return c2sErr
			}
			return nil
		}
	}
	return status.Errorf(codes.Internal, "gRPC proxy should never reach this stage.")
}

func (h *handler) forwardClientToServer(src grpc.ClientStream, dst grpc.ServerStream) chan error {
	ret := make(chan error, 1)
	go func() {
		f := &emptypb.Empty{}
		for i := 0; ; i++ {
			if err := src.RecvMsg(f); err != nil {
				ret <- err // this can be io.EOF which is happy case
				break
			}
			if i == 0 {
				// This is a bit of a hack, but client to server headers are only readable after first client msg is
				// received but must be written to server stream before the first msg is flushed.
				// This is the only place to do it nicely.
				md, err := src.Header()
				if err != nil {
					ret <- err
					break
				}
				if err := dst.SendHeader(md); err != nil {
					ret <- err
					break
				}
			}
			if err := dst.SendMsg(f); err != nil {
				ret <- err
				break
			}
		}
	}()
	return ret
}

func (h *handler) forwardServerToClient(src grpc.ServerStream, dst grpc.ClientStream) chan error {
	ret := make(chan error, 1)
	go func() {
		f := &emptypb.Empty{}
		for i := 0; ; i++ {
			if err := src.RecvMsg(f); err != nil {
				ret <- err // this can be io.EOF which is happy case
				break
			}
			if err := dst.SendMsg(f); err != nil {
				ret <- err
				break
			}
		}
	}()
	return ret
}

func (h *handler) injectError(serverStream grpc.ServerStream) error {
	err := h.drainServerStream(serverStream)
	if err != nil {
		return fmt.Errorf("error receiving request from client %w", err)
	}

	return status.Error(codes.Code(h.disruption.StatusCode), h.disruption.StatusMessage)
}

// read all messages from client
func (h *handler) drainServerStream(src grpc.ServerStream) error {
	f := &emptypb.Empty{}
	for {
		if err := src.RecvMsg(f); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}
