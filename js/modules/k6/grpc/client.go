/*
 *
 * k6 - a next-generation load testing tool
 * Copyright (C) 2020 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	grpcstats "google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	//nolint: staticcheck
	protoV1 "github.com/golang/protobuf/proto"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/lib/types"
	"github.com/loadimpact/k6/stats"
)

//nolint: lll
var (
	errInvokeRPCInInitContext = common.NewInitContextError("invoking RPC methods in the init context is not supported")
	errConnectInInitContext   = common.NewInitContextError("connecting to a gRPC server in the init context is not supported")
)

// Client represents a gRPC client that can be used to make RPC requests
type Client struct {
	mds  map[string]protoreflect.MethodDescriptor
	conn *grpc.ClientConn
}

// XClient represents the Client constructor (e.g. `new grpc.Client()`) and
// creates a new gPRC client object that can load protobuf definitions, connect
// to servers and invoke RPC methods.
func (*GRPC) XClient(ctxPtr *context.Context) interface{} {
	rt := common.GetRuntime(*ctxPtr)

	return common.Bind(rt, &Client{}, ctxPtr)
}

// MethodInfo holds information on any parsed method descriptors that can be used by the goja VM
type MethodInfo struct {
	grpc.MethodInfo `json:"-" js:"-"`
	Package         string
	Service         string
	FullMethod      string
}

// Response is a gRPC response that can be used by the goja VM
type Response struct {
	Status   codes.Code
	Message  interface{}
	Headers  map[string][]string
	Trailers map[string][]string
	Error    interface{}
}

func walkFileDescriptors(seen map[string]struct{}, fd *desc.FileDescriptor) []*descriptorpb.FileDescriptorProto {
	fds := []*descriptorpb.FileDescriptorProto{}

	if _, ok := seen[fd.GetName()]; ok {
		return fds
	}
	seen[fd.GetName()] = struct{}{}
	fds = append(fds, fd.AsFileDescriptorProto())

	for _, dep := range fd.GetDependencies() {
		deps := walkFileDescriptors(seen, dep)
		fds = append(fds, deps...)
	}

	return fds
}

// Load will parse the given proto files and make the file descriptors available to request.
func (c *Client) Load(ctxPtr *context.Context, importPaths []string, filenames ...string) ([]MethodInfo, error) {
	if lib.GetState(*ctxPtr) != nil {
		return nil, errors.New("load must be called in the init context")
	}

	initEnv := common.GetInitEnv(*ctxPtr)
	if initEnv == nil {
		return nil, errors.New("missing init environment")
	}

	// If no import paths are specified, use the current working directory
	if len(importPaths) == 0 {
		importPaths = append(importPaths, initEnv.CWD.Path)
	}

	parser := protoparse.Parser{
		ImportPaths:      importPaths,
		InferImportPaths: false,
		Accessor: protoparse.FileAccessor(func(filename string) (io.ReadCloser, error) {
			absFilePath := initEnv.GetAbsFilePath(filename)
			return initEnv.FileSystems["file"].Open(absFilePath)
		}),
	}

	fds, err := parser.ParseFiles(filenames...)
	if err != nil {
		return nil, err
	}

	fdset := &descriptorpb.FileDescriptorSet{}

	seen := make(map[string]struct{})
	for _, fd := range fds {
		fdset.File = append(fdset.File, walkFileDescriptors(seen, fd)...)
	}

	files, err := protodesc.NewFiles(fdset)
	if err != nil {
		return nil, err
	}

	var rtn []MethodInfo
	if c.mds == nil {
		// This allows us to call load() multiple times, without overwriting the
		// previously loaded definitions.
		c.mds = make(map[string]protoreflect.MethodDescriptor)
	}

	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		sds := fd.Services()
		for i := 0; i < sds.Len(); i++ {
			sd := sds.Get(i)
			mds := sd.Methods()
			for j := 0; j < mds.Len(); j++ {
				md := mds.Get(j)
				name := fmt.Sprintf("/%s/%s", sd.FullName(), md.Name())
				c.mds[name] = md
				rtn = append(rtn, MethodInfo{
					MethodInfo: grpc.MethodInfo{
						Name:           string(md.Name()),
						IsClientStream: md.IsStreamingClient(),
						IsServerStream: md.IsStreamingServer(),
					},
					Package:    string(fd.Package()),
					Service:    string(sd.Name()),
					FullMethod: name,
				})
			}
		}
		return true
	})

	return rtn, nil
}

type transportCreds struct {
	credentials.TransportCredentials
	errc chan<- error
}

func (t transportCreds) ClientHandshake(ctx context.Context,
	addr string, in net.Conn) (net.Conn, credentials.AuthInfo, error) {
	out, auth, err := t.TransportCredentials.ClientHandshake(ctx, addr, in)
	if err != nil {
		t.errc <- err
	}

	return out, auth, err
}

// Connect is a block dial to the gRPC server at the given address (host:port)
// nolint: funlen
func (c *Client) Connect(ctxPtr *context.Context, addr string, params map[string]interface{}) (bool, error) {
	state := lib.GetState(*ctxPtr)
	if state == nil {
		return false, errConnectInInitContext
	}

	isPlaintext, timeout := false, 60*time.Second

	for k, v := range params {
		switch k {
		case "plaintext":
			isPlaintext, _ = v.(bool)
		case "timeout":
			var err error
			timeout, err = types.GetDurationValue(v)
			if err != nil {
				return false, fmt.Errorf("invalid timeout value: %w", err)
			}
		default:
			return false, fmt.Errorf("unknown connect param: %q", k)
		}
	}

	// (rogchap) Even with FailOnNonTempDialError, if there is a TLS error this will timeout
	// rather than report the error, so we can't rely on WithBlock. By running in a goroutine
	// we can then wait on the error channel instead, which could happen before the Dial
	// returns. We only need to close the channel to un-block in a non-error scenario;
	// otherwise it can be GCd without closing as we return on an error on the channel.
	errc := make(chan error, 1)
	go func() {
		opts := []grpc.DialOption{
			grpc.WithBlock(),
			grpc.FailOnNonTempDialError(true),
			grpc.WithStatsHandler(c),
		}

		if ua := state.Options.UserAgent; ua.Valid {
			opts = append(opts, grpc.WithUserAgent(ua.ValueOrZero()))
		}

		if !isPlaintext {
			tlsCfg := state.TLSConfig.Clone()
			tlsCfg.NextProtos = []string{"h2"}

			// TODO(rogchap): Would be good to add support for custom RootCAs (self signed)

			// (rogchap) we create a wrapper for transport credentials so that we can report
			// on any TLS errors.
			creds := transportCreds{
				credentials.NewTLS(tlsCfg),
				errc,
			}
			opts = append(opts, grpc.WithTransportCredentials(creds))
		}

		if isPlaintext {
			opts = append(opts, grpc.WithInsecure())
		}

		dialer := func(ctx context.Context, addr string) (net.Conn, error) {
			return state.Dialer.DialContext(ctx, "tcp", addr)
		}
		opts = append(opts, grpc.WithContextDialer(dialer))

		ctx, cancel := context.WithTimeout(*ctxPtr, timeout)
		defer cancel()

		var err error
		c.conn, err = grpc.DialContext(ctx, addr, opts...)
		if err != nil {
			errc <- err

			return
		}
		close(errc)
	}()

	if err := <-errc; err != nil {
		return false, err
	}

	return true, nil
}

// Invoke creates and calls a unary RPC by fully qualified method name
//nolint: funlen,gocognit,gocyclo
func (c *Client) Invoke(ctxPtr *context.Context,
	method string, req goja.Value, params map[string]interface{}) (*Response, error) {
	ctx := *ctxPtr
	rt := common.GetRuntime(ctx)
	state := lib.GetState(ctx)
	if state == nil {
		return nil, errInvokeRPCInInitContext
	}

	if c.conn == nil {
		return nil, errors.New("no gRPC connection, you must call connect first")
	}

	if method == "" {
		return nil, errors.New("method to invoke cannot be empty")
	}

	if method[0] != '/' {
		method = "/" + method
	}
	md := c.mds[method]

	if md == nil {
		return nil, fmt.Errorf("method %q not found in file descriptors", method)
	}

	tags := state.CloneTags()
	timeout := 60 * time.Second

	ctx = metadata.NewOutgoingContext(ctx, metadata.New(nil))
	for k, v := range params {
		switch k {
		case "headers":
			rawHeaders, ok := v.(map[string]interface{})
			if !ok {
				return nil, errors.New("headers must be an object with key-value pairs")
			}
			for hk, kv := range rawHeaders {
				// TODO(rogchap): Should we manage a string slice?
				strVal, ok := kv.(string)
				if !ok {
					return nil, fmt.Errorf("header %q value must be a string", hk)
				}
				ctx = metadata.AppendToOutgoingContext(ctx, hk, strVal)
			}
		case "tags":
			rawTags, ok := v.(map[string]interface{})
			if !ok {
				return nil, errors.New("tags must be an object with key-value pairs")
			}
			for tk, tv := range rawTags {
				strVal, ok := tv.(string)
				if !ok {
					return nil, fmt.Errorf("tag %q value must be a string", tk)
				}
				tags[tk] = strVal
			}
		case "timeout":
			var err error
			timeout, err = types.GetDurationValue(v)
			if err != nil {
				return nil, fmt.Errorf("invalid timeout value: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown param: %q", k)
		}
	}
	if state.Options.SystemTags.Has(stats.TagURL) {
		tags["url"] = fmt.Sprintf("%s%s", c.conn.Target(), method)
	}

	parts := strings.Split(method[1:], "/")
	if state.Options.SystemTags.Has(stats.TagService) {
		tags["service"] = parts[0]
	}
	if state.Options.SystemTags.Has(stats.TagMethod) {
		tags["method"] = parts[1]
	}

	// Only set the name system tag if the user didn't explicitly set it beforehand
	if _, ok := tags["name"]; !ok && state.Options.SystemTags.Has(stats.TagName) {
		tags["name"] = method
	}

	ctx = withTags(ctx, tags)

	reqdm := dynamicpb.NewMessage(md.Input())
	{
		b, err := req.ToObject(rt).MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("unable to serialise request object: %v", err)
		}
		if err := protojson.Unmarshal(b, reqdm); err != nil {
			return nil, fmt.Errorf("unable to serialise request object to protocol buffer: %v", err)
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	resp := dynamicpb.NewMessage(md.Output())
	header, trailer := metadata.New(nil), metadata.New(nil)
	err := c.conn.Invoke(reqCtx, method, reqdm, resp, grpc.Header(&header), grpc.Trailer(&trailer))

	var response Response
	response.Headers = header
	response.Trailers = trailer

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

// Close will close the client gRPC connection
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil

	return err
}

// TagConn implements the stats.Handler interface
func (*Client) TagConn(ctx context.Context, _ *grpcstats.ConnTagInfo) context.Context {
	// noop
	return ctx
}

// HandleConn implements the stats.Handler interface
func (*Client) HandleConn(context.Context, grpcstats.ConnStats) {
	// noop
}

// TagRPC implements the stats.Handler interface
func (*Client) TagRPC(ctx context.Context, _ *grpcstats.RPCTagInfo) context.Context {
	// noop
	return ctx
}

// HandleRPC implements the stats.Handler interface
func (c *Client) HandleRPC(ctx context.Context, stat grpcstats.RPCStats) {
	state := lib.GetState(ctx)
	tags := getTags(ctx)

	switch s := stat.(type) {
	case *grpcstats.OutHeader:
		if state.Options.SystemTags.Has(stats.TagIP) && s.RemoteAddr != nil {
			if ip, _, err := net.SplitHostPort(s.RemoteAddr.String()); err == nil {
				tags["ip"] = ip
			}
		}
	case *grpcstats.End:
		if state.Options.SystemTags.Has(stats.TagStatus) {
			tags["status"] = strconv.Itoa(int(status.Code(s.Error)))
		}

		mTags := map[string]string(tags)
		sampleTags := stats.IntoSampleTags(&mTags)
		stats.PushIfNotDone(ctx, state.Samples, stats.ConnectedSamples{
			Samples: []stats.Sample{
				{
					Metric: metrics.GRPCReqDuration,
					Tags:   sampleTags,
					Value:  stats.D(s.EndTime.Sub(s.BeginTime)),
					Time:   s.EndTime,
				},
			},
		})
	}

	// (rogchap) Re-using --http-debug flag as gRPC is technically still HTTP
	if state.Options.HTTPDebug.String != "" {
		logger := state.Logger.WithField("source", "http-debug")
		httpDebugOption := state.Options.HTTPDebug.String
		debugStat(stat, logger, httpDebugOption)
	}
}

func debugStat(stat grpcstats.RPCStats, logger logrus.FieldLogger, httpDebugOption string) {
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
		msgV1, ok := payload.(protoV1.Message)
		if !ok {
			return ""
		}
		msg = protoV1.MessageV2(msgV1)
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
