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
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
	//nolint: staticcheck
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	grpcstats "google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/stats"
)

//nolint: lll
var (
	errInvokeRPCInInitContext = common.NewInitContextError("invoking RPC methods in the init context is not supported")
	errConnectInInitContext   = common.NewInitContextError("connecting to a gRPC server in the init context is not supported")
)

// Client reprecents a gRPC client that can be used to make RPC requests
type Client struct {
	mds  map[string]*desc.MethodDescriptor
	tags map[string]string
	conn *grpc.ClientConn
}

// NewClient creates a new gPRC client to make invoke RPC methods.
func (*GRPC) NewClient(ctxPtr *context.Context) interface{} {
	rt := common.GetRuntime(*ctxPtr)

	return common.Bind(rt, &Client{}, ctxPtr)
}

// MethodInfo holds information on any parsed method descriptors that can be used by the goja VM
type MethodInfo struct {
	grpc.MethodInfo
	Package    string
	Service    string
	FullMethod string
}

// Response is a gRPC response that can be used by the goja VM
type Response struct {
	Status   codes.Code
	Message  interface{}
	Headers  map[string][]string
	Trailers map[string][]string
	Error    interface{}
}

// Load will parse the given proto files and make the file descriptors available to request.
func (c *Client) Load(ctxPtr *context.Context, importPaths []string, filenames ...string) ([]MethodInfo, error) {
	if lib.GetState(*ctxPtr) != nil {
		return nil, errors.New("load must be called in the init context")
	}

	parser := protoparse.Parser{
		ImportPaths:      importPaths,
		InferImportPaths: len(importPaths) == 0,
	}

	fds, err := parser.ParseFiles(filenames...)
	if err != nil {
		return nil, err
	}

	var rtn []MethodInfo
	c.mds = make(map[string]*desc.MethodDescriptor)
	for _, fd := range fds {
		for _, sd := range fd.GetServices() {
			for _, md := range sd.GetMethods() {
				name := fmt.Sprintf("/%s/%s", sd.GetFullyQualifiedName(), md.GetName())
				c.mds[name] = md
				rtn = append(rtn, MethodInfo{
					MethodInfo: grpc.MethodInfo{
						Name:           md.GetName(),
						IsClientStream: md.IsClientStreaming(),
						IsServerStream: md.IsServerStreaming(),
					},
					Package:    fd.GetPackage(),
					Service:    sd.GetName(),
					FullMethod: name,
				})
			}
		}
	}

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
			switch t := v.(type) {
			case string:
				if timeout, err = time.ParseDuration(t); err != nil {
					return false, fmt.Errorf("unable to parse %q: %v", k, err)
				}
			case int64:
				timeout = time.Duration(float64(t)) * time.Millisecond
			case float64:
				timeout = time.Duration(t) * time.Millisecond
			default:
				return false, fmt.Errorf("unable to use type %T as a timeout value", v)
			}
		default:
			return false, fmt.Errorf("unknown connect param: %q", k)
		}
	}

	errc := make(chan error)
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

// InvokeRPC creates and calls a unary RPC by fully qualified method name
//nolint: funlen,gocognit
func (c *Client) InvokeRPC(ctxPtr *context.Context,
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

	c.tags = state.CloneTags()
	timeout := 60 * time.Second

	ctx = metadata.NewOutgoingContext(ctx, metadata.New(nil))
	for k, v := range params {
		switch k {
		case "headers":
			rawHeaders, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			for hk, kv := range rawHeaders {
				// TODO(rogchap): Should we manage a string slice?
				strVal, ok := kv.(string)
				if !ok {
					continue
				}
				ctx = metadata.AppendToOutgoingContext(ctx, hk, strVal)
			}
		case "tags":
			rawTags, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			for tk, tv := range rawTags {
				strVal, ok := tv.(string)
				if !ok {
					continue
				}
				c.tags[tk] = strVal
			}
		case "timeout":
			var err error
			switch t := v.(type) {
			case string:
				if timeout, err = time.ParseDuration(t); err != nil {
					return nil, fmt.Errorf("unable to parse %q: %v", k, err)
				}
			case int64:
				timeout = time.Duration(float64(t)) * time.Millisecond
			case float64:
				timeout = time.Duration(t) * time.Millisecond
			default:
				return nil, fmt.Errorf("unable to use type %T as a timeout value", v)
			}
		default:
			return nil, fmt.Errorf("unknown param: %q", k)
		}
	}
	if state.Options.SystemTags.Has(stats.TagURL) {
		c.tags["url"] = fmt.Sprintf("%s%s", c.conn.Target(), method)
	}

	parts := strings.Split(method[1:], "/")
	if state.Options.SystemTags.Has(stats.TagService) {
		c.tags["service"] = parts[0]
	}
	if state.Options.SystemTags.Has(stats.TagMethod) {
		c.tags["method"] = parts[1]
	}

	if state.Options.SystemTags.Has(stats.TagRPCType) {
		// (rogchap) This method only supports unary RPCs
		// if this is refactored to support streaming then this should
		// be updated to be based on the method descriptor (IsClientStreaming/IsServerStreaming)
		c.tags["rpc_type"] = "unary"
	}

	// Only set the name system tag if the user didn't explicitly set it beforehand
	if _, ok := c.tags["name"]; !ok && state.Options.SystemTags.Has(stats.TagName) {
		c.tags["name"] = method
	}

	reqdm := dynamic.NewMessage(md.GetInputType())
	b, _ := req.ToObject(rt).MarshalJSON()
	_ = reqdm.UnmarshalJSON(b)

	if rpsLimit := state.RPSLimit; rpsLimit != nil {
		if err := rpsLimit.Wait(ctx); err != nil {
			return nil, err
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	s := grpcdynamic.NewStub(c.conn)
	header, trailer := metadata.New(nil), metadata.New(nil)
	resp, err := s.InvokeRpc(reqCtx, md, reqdm, grpc.Header(&header), grpc.Trailer(&trailer))

	var response Response
	response.Headers = header
	response.Trailers = trailer

	if err != nil {
		st := status.Convert(err)
		response.Status = st.Code()
		errdm, _ := dynamic.AsDynamicMessage(st.Proto())

		// (rogchap) when you access a JSON property in goja, you are actually accessing the underling
		// Go type (struct, map, slice etc); because these are dynamic messages the Unmarshaled JSON does
		// not map back to a "real" field or value (as a normal Go type would). If we don't marshal and then
		// unmarshal back to a map, you will get "undefined" when accessing JSON properties, even when
		// JSON.Stringify() shows the object to be correctly present.
		raw, _ := errdm.MarshalJSONPB(&jsonpb.Marshaler{EmitDefaults: true})
		errMsg := make(map[string]interface{})
		_ = json.Unmarshal(raw, &errMsg)
		response.Error = errMsg
	}

	if resp != nil {
		msgdm := dynamic.NewMessage(md.GetOutputType())
		msgdm.Merge(resp)
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
		raw, _ := msgdm.MarshalJSONPB(&jsonpb.Marshaler{EmitDefaults: true})
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

	switch s := stat.(type) {
	case *grpcstats.OutHeader:
		if state.Options.SystemTags.Has(stats.TagIP) && s.RemoteAddr != nil {
			if ip, _, err := net.SplitHostPort(s.RemoteAddr.String()); err == nil {
				c.tags["ip"] = ip
			}
		}
	case *grpcstats.End:

		if state.Options.SystemTags.Has(stats.TagStatus) {
			c.tags["status"] = strconv.Itoa(int(status.Code(s.Error)))
		}

		tags := stats.IntoSampleTags(&c.tags)
		stats.PushIfNotDone(ctx, state.Samples, stats.ConnectedSamples{
			Samples: []stats.Sample{
				{
					Metric: metrics.GRPCReqDuration,
					Tags:   tags,
					Value:  stats.D(s.EndTime.Sub(s.BeginTime)),
					Time:   s.BeginTime,
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
		return ""
	}
	dm, err := dynamic.AsDynamicMessage(msg)
	if err != nil {
		return ""
	}
	b, err := dm.MarshalTextIndent()
	if err != nil {
		return ""
	}

	return string(b)
}
