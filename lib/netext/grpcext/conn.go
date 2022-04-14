// Package grpcext allows gRPC requests collecting stats info.
package grpcext

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"go.k6.io/k6/js/modules"
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
	"google.golang.org/protobuf/types/dynamicpb"
)

// Request represents a gRPC request.
type Request struct {
	MethodDescriptor protoreflect.MethodDescriptor
	Tags             map[string]string
	Message          []byte
}

// Response represents a gRPC response.
type Response struct {
	Message  interface{}
	Error    interface{}
	Headers  map[string][]string
	Trailers map[string][]string
	Status   codes.Code
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
func DefaultOptions(vu modules.VU) []grpc.DialOption {
	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		return vu.State().Dialer.DialContext(ctx, "tcp", addr)
	}

	return []grpc.DialOption{
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
		grpc.WithReturnConnectionError(),
		grpc.WithStatsHandler(statsHandler{vu: vu}),
		grpc.WithContextDialer(dialer),
	}
}

// Dial establish a gRPC connection.
func Dial(ctx context.Context, addr string, options ...grpc.DialOption) (*Conn, error) {
	conn, err := grpc.DialContext(ctx, addr, options...)
	if err != nil {
		return nil, err
	}
	return &Conn{
		raw: conn,
	}, nil
}

// ReflectionClient returns a reflection client based on the current connection.
func (c *Conn) ReflectionClient() (*ReflectionClient, error) {
	return &ReflectionClient{Conn: c.raw}, nil
}

// Invoke executes a unary gRPC request.
func (c *Conn) Invoke(
	ctx context.Context,
	url string,
	md metadata.MD,
	req Request,
	opts ...grpc.CallOption) (*Response, error) {
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}
	if req.MethodDescriptor == nil {
		return nil, fmt.Errorf("request method descriptor is required")
	}
	if len(req.Message) == 0 {
		return nil, fmt.Errorf("request message is required")
	}

	ctx = metadata.NewOutgoingContext(ctx, md)

	reqdm := dynamicpb.NewMessage(req.MethodDescriptor.Input())
	if err := protojson.Unmarshal(req.Message, reqdm); err != nil {
		return nil, fmt.Errorf("unable to serialise request object to protocol buffer: %w", err)
	}

	ctx = withTags(ctx, req.Tags)

	resp := dynamicpb.NewMessage(req.MethodDescriptor.Output())
	header, trailer := metadata.New(nil), metadata.New(nil)

	copts := make([]grpc.CallOption, 0, len(opts)+2)
	copts = append(copts, opts...)
	copts = append(copts, grpc.Header(&header), grpc.Trailer(&trailer))

	err := c.raw.Invoke(ctx, url, reqdm, resp, copts...)

	response := Response{
		Headers:  header,
		Trailers: trailer,
	}

	marshaler := protojson.MarshalOptions{EmitUnpopulated: true}

	if err != nil {
		sterr := status.Convert(err)
		response.Status = sterr.Code()

		// (rogchap) when you access a JSON property in goja, you are actually accessing the underling
		// Go type (struct, map, slice etc); because these are dynamic messages the Unmarshaled JSON does
		// not map back to a "real" field or value (as a normal Go type would). If we don't marshal and then
		// unmarshal back to a map, you will get "undefined" when accessing JSON properties, even when
		// JSON.Stringify() shows the object to be correctly present.

		raw, _ := marshaler.Marshal(sterr.Proto())
		errMsg := make(map[string]interface{})
		_ = json.Unmarshal(raw, &errMsg)
		response.Error = errMsg
	}

	if resp != nil {
		// (rogchap) there is a lot of marshaling/unmarshaling here, but if we just pass the dynamic message
		// the default Marshaller would be used, which would strip any zero/default values from the JSON.
		// eg. given this message:
		// message Point {
		//    double x = 1;
		// 	  double y = 2;
		// 	  double z = 3;
		// }
		// and a value like this:
		// msg := Point{X: 6, Y: 4, Z: 0}
		// would result in JSON output:
		// {"x":6,"y":4}
		// rather than the desired:
		// {"x":6,"y":4,"z":0}
		raw, _ := marshaler.Marshal(resp)
		msg := make(map[string]interface{})
		_ = json.Unmarshal(raw, &msg)
		response.Message = msg
	}
	return &response, nil
}

// Close closes the underhood connection.
func (c *Conn) Close() error {
	return c.raw.Close()
}

type statsHandler struct {
	vu modules.VU
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
	state := h.vu.State()
	tags := getTags(ctx)
	switch s := stat.(type) {
	case *grpcstats.OutHeader:
		if state.Options.SystemTags.Has(metrics.TagIP) && s.RemoteAddr != nil {
			if ip, _, err := net.SplitHostPort(s.RemoteAddr.String()); err == nil {
				tags["ip"] = ip
			}
		}
	case *grpcstats.End:
		if state.Options.SystemTags.Has(metrics.TagStatus) {
			tags["status"] = strconv.Itoa(int(status.Code(s.Error)))
		}

		mTags := map[string]string(tags)
		sampleTags := metrics.IntoSampleTags(&mTags)
		metrics.PushIfNotDone(ctx, state.Samples, metrics.ConnectedSamples{
			Samples: []metrics.Sample{
				{
					Metric: state.BuiltinMetrics.GRPCReqDuration,
					Tags:   sampleTags,
					Value:  metrics.D(s.EndTime.Sub(s.BeginTime)),
					Time:   s.EndTime,
				},
			},
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

type ctxKeyTags struct{}

type reqtags map[string]string

func withTags(ctx context.Context, tags reqtags) context.Context {
	if tags == nil {
		tags = make(map[string]string)
	}
	return context.WithValue(ctx, ctxKeyTags{}, tags)
}

func getTags(ctx context.Context) reqtags {
	v := ctx.Value(ctxKeyTags{})
	if v == nil {
		return make(map[string]string)
	}
	return v.(reqtags)
}
