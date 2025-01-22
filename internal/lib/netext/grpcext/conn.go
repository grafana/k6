// Package grpcext allows gRPC requests collecting stats info.
package grpcext

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/metrics"

	protov1 "github.com/golang/protobuf/proto" //nolint:staticcheck,nolintlint // this is the old v1 version
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	grpcstats "google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

// InvokeRequest represents a unary gRPC request.
type InvokeRequest struct {
	Method                 string
	MethodDescriptor       protoreflect.MethodDescriptor
	Timeout                time.Duration
	TagsAndMeta            *metrics.TagsAndMeta
	DiscardResponseMessage bool
	Message                []byte
	Metadata               metadata.MD
}

// InvokeResponse represents a gRPC response.
type InvokeResponse struct {
	Message  interface{}
	Error    interface{}
	Headers  map[string][]string
	Trailers map[string][]string
	Status   codes.Code
}

// StreamRequest represents a gRPC stream request.
type StreamRequest struct {
	Method                 string
	MethodDescriptor       protoreflect.MethodDescriptor
	Timeout                time.Duration
	DiscardResponseMessage bool
	TagsAndMeta            *metrics.TagsAndMeta
	Metadata               metadata.MD
}

type clientConnCloser interface {
	grpc.ClientConnInterface
	Close() error
}

// Conn is a gRPC client connection.
type Conn struct {
	raw clientConnCloser
}

// DefaultOptions generates an option set
// with common options for requests from a VU.
func DefaultOptions(getState func() *lib.State) []grpc.DialOption {
	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		return getState().Dialer.DialContext(ctx, "tcp", addr)
	}

	//nolint:staticcheck // see https://github.com/grafana/k6/issues/3699
	return []grpc.DialOption{
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
		grpc.WithReturnConnectionError(),
		grpc.WithStatsHandler(statsHandler{getState: getState}),
		grpc.WithContextDialer(dialer),
	}
}

// Dial establish a gRPC connection.
func Dial(ctx context.Context, addr string, options ...grpc.DialOption) (*Conn, error) {
	//nolint:staticcheck // see https://github.com/grafana/k6/issues/3699
	conn, err := grpc.DialContext(ctx, addr, options...)
	if err != nil {
		return nil, err
	}
	return &Conn{
		raw: conn,
	}, nil
}

// Reflect returns using the reflection the FileDescriptorSet describing the service.
func (c *Conn) Reflect(ctx context.Context) (*descriptorpb.FileDescriptorSet, error) {
	rc := reflectionClient{Conn: c.raw}
	return rc.Reflect(ctx)
}

// Invoke executes a unary gRPC request.
func (c *Conn) Invoke(
	ctx context.Context,
	req InvokeRequest,
	opts ...grpc.CallOption,
) (*InvokeResponse, error) {
	if req.Method == "" {
		return nil, fmt.Errorf("url is required")
	}

	if req.MethodDescriptor == nil {
		return nil, fmt.Errorf("request method descriptor is required")
	}
	if len(req.Message) == 0 {
		return nil, fmt.Errorf("request message is required")
	}

	if req.Timeout != time.Duration(0) {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, req.Timeout)
		defer cancel()
	}

	ctx = metadata.NewOutgoingContext(ctx, req.Metadata)

	reqdm := dynamicpb.NewMessage(req.MethodDescriptor.Input())
	if err := protojson.Unmarshal(req.Message, reqdm); err != nil {
		return nil, fmt.Errorf("unable to serialise request object to protocol buffer: %w", err)
	}

	ctx = withRPCState(ctx, &rpcState{tagsAndMeta: req.TagsAndMeta})

	var resp *dynamicpb.Message
	if req.DiscardResponseMessage {
		resp = dynamicpb.NewMessage((&emptypb.Empty{}).ProtoReflect().Descriptor())
	} else {
		resp = dynamicpb.NewMessage(req.MethodDescriptor.Output())
	}

	header, trailer := metadata.New(nil), metadata.New(nil)

	copts := make([]grpc.CallOption, 0, len(opts)+2)
	copts = append(copts, opts...)
	copts = append(copts, grpc.Header(&header), grpc.Trailer(&trailer))

	err := c.raw.Invoke(ctx, req.Method, reqdm, resp, copts...)

	response := InvokeResponse{
		Headers:  header,
		Trailers: trailer,
	}

	marshaler := protojson.MarshalOptions{EmitUnpopulated: true}

	if err != nil {
		sterr := status.Convert(err)
		response.Status = sterr.Code()

		// (rogchap) when you access a JSON property in Sobek, you are actually accessing the underling
		// Go type (struct, map, slice etc); because these are dynamic messages the Unmarshaled JSON does
		// not map back to a "real" field or value (as a normal Go type would). If we don't marshal and then
		// unmarshal back to a map, you will get "undefined" when accessing JSON properties, even when
		// JSON.Stringify() shows the object to be correctly present.

		raw, _ := marshaler.Marshal(sterr.Proto())
		errMsg := make(map[string]interface{})
		_ = json.Unmarshal(raw, &errMsg)
		response.Error = errMsg
	}

	if resp != nil && !req.DiscardResponseMessage {
		msg, err := convert(marshaler, resp)
		if err != nil {
			return nil, fmt.Errorf("unable to convert response object to JSON: %w", err)
		}

		response.Message = msg
	}
	return &response, nil
}

// NewStream creates a new gRPC stream.
func (c *Conn) NewStream(
	ctx context.Context,
	req StreamRequest,
	opts ...grpc.CallOption,
) (*Stream, error) {
	ctx = metadata.NewOutgoingContext(ctx, req.Metadata)

	ctx = withRPCState(ctx, &rpcState{tagsAndMeta: req.TagsAndMeta})

	stream, err := c.raw.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    string(req.MethodDescriptor.Name()),
		ServerStreams: req.MethodDescriptor.IsStreamingServer(),
		ClientStreams: req.MethodDescriptor.IsStreamingClient(),
	}, req.Method, opts...)
	if err != nil {
		return nil, err
	}

	return &Stream{
		raw:                    stream,
		method:                 req.Method,
		methodDescriptor:       req.MethodDescriptor,
		discardResponseMessage: req.DiscardResponseMessage,
	}, nil
}

// Close closes the underhood connection.
func (c *Conn) Close() error {
	return c.raw.Close()
}

type statsHandler struct {
	getState func() *lib.State
}

// TagConn implements the grpcstats.Handler interface
func (statsHandler) TagConn(ctx context.Context, _ *grpcstats.ConnTagInfo) context.Context { // noop
	return ctx
}

// HandleConn implements the grpcstats.Handler interface
func (statsHandler) HandleConn(context.Context, grpcstats.ConnStats) {
	// noop
}

// TagRPC implements the grpcstats.Handler interface
func (statsHandler) TagRPC(ctx context.Context, _ *grpcstats.RPCTagInfo) context.Context {
	// noop
	return ctx
}

// HandleRPC implements the grpcstats.Handler interface
func (h statsHandler) HandleRPC(ctx context.Context, stat grpcstats.RPCStats) {
	state := h.getState()
	stateRPC := getRPCState(ctx) //nolint:ifshort

	// If the request is done by the reflection handler then the tags will be
	// nil. In this case, we can reuse the VU.State's Tags.
	if stateRPC == nil {
		// TODO: investigate this more, there has to be a way to fix it :/
		ctm := state.Tags.GetCurrentValues()
		stateRPC = &rpcState{tagsAndMeta: &ctm}
	}

	switch s := stat.(type) {
	case *grpcstats.OutHeader:
		// TODO: figure out something better, e.g. via TagConn() or TagRPC()?
		if state.Options.SystemTags.Has(metrics.TagIP) && s.RemoteAddr != nil {
			if ip, _, err := net.SplitHostPort(s.RemoteAddr.String()); err == nil {
				stateRPC.tagsAndMeta.SetSystemTagOrMeta(metrics.TagIP, ip)
			}
		}
	case *grpcstats.End:
		if state.Options.SystemTags.Has(metrics.TagStatus) {
			stateRPC.tagsAndMeta.SetSystemTagOrMeta(metrics.TagStatus, strconv.Itoa(int(status.Code(s.Error))))
		}

		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			TimeSeries: metrics.TimeSeries{
				Metric: state.BuiltinMetrics.GRPCReqDuration,
				Tags:   stateRPC.tagsAndMeta.Tags,
			},
			Time:     s.EndTime,
			Metadata: stateRPC.tagsAndMeta.Metadata,
			Value:    metrics.D(s.EndTime.Sub(s.BeginTime)),
		})
	}

	// (rogchap) Re-using --http-debug flag as gRPC is technically still HTTP
	if state.Options.HTTPDebug.String != "" {
		logger := state.Logger.WithField("source", "http-debug")
		httpDebugOption := state.Options.HTTPDebug.String
		DebugStat(logger, stat, httpDebugOption)
	}
}

// DebugStat prints debugging information based on RPCStats.
func DebugStat(logger logrus.FieldLogger, stat grpcstats.RPCStats, httpDebugOption string) {
	switch s := stat.(type) {
	case *grpcstats.OutHeader:
		logger.Infof("Out Header:\nFull Method: %s\nRemote Address: %s\n%s\n",
			s.FullMethod, s.RemoteAddr, formatMetadata(s.Header))
	case *grpcstats.OutTrailer:
		if len(s.Trailer) > 0 {
			logger.Infof("Out Trailer:\n%s\n", formatMetadata(s.Trailer))
		}
	case *grpcstats.OutPayload:
		if httpDebugOption == "full" {
			logger.Infof("Out Payload:\nWire Length: %d\nSent Time: %s\n%s\n\n",
				s.WireLength, s.SentTime, formatPayload(s.Payload))
		}
	case *grpcstats.InHeader:
		if len(s.Header) > 0 {
			logger.Infof("In Header:\nWire Length: %d\n%s\n", s.WireLength, formatMetadata(s.Header))
		}
	case *grpcstats.InTrailer:
		if len(s.Trailer) > 0 {
			logger.Infof("In Trailer:\nWire Length: %d\n%s\n", s.WireLength, formatMetadata(s.Trailer))
		}
	case *grpcstats.InPayload:
		if httpDebugOption == "full" {
			logger.Infof("In Payload:\nWire Length: %d\nReceived Time: %s\n%s\n\n",
				s.WireLength, s.RecvTime, formatPayload(s.Payload))
		}
	}
}

func formatMetadata(md metadata.MD) string {
	var sb strings.Builder
	for k, v := range md {
		sb.WriteString(k)
		sb.WriteString(": ")
		sb.WriteString(strings.Join(v, ", "))
		sb.WriteRune('\n')
	}

	return sb.String()
}

func formatPayload(payload interface{}) string {
	msg, ok := payload.(proto.Message)
	if !ok {
		// check to see if we are dealing with a APIv1 message
		msgV1, ok := payload.(protov1.Message)
		if !ok {
			return ""
		}
		msg = protov1.MessageV2(msgV1)
	}

	marshaler := prototext.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
	}
	b, err := marshaler.Marshal(msg)
	if err != nil {
		return ""
	}
	return string(b)
}

type contextKey string

var ctxKeyRPCState = contextKey("rpcState") //nolint:gochecknoglobals

type rpcState struct {
	tagsAndMeta *metrics.TagsAndMeta
}

func withRPCState(ctx context.Context, rpcState *rpcState) context.Context {
	return context.WithValue(ctx, ctxKeyRPCState, rpcState)
}

func getRPCState(ctx context.Context) *rpcState {
	v := ctx.Value(ctxKeyRPCState)
	if v == nil {
		return nil
	}
	return v.(*rpcState) //nolint: forcetypeassert
}
