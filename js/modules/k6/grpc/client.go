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

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"
	reflectpb "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
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

	vu modules.VU
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

// Load will parse the given proto files and make the file descriptors available to request.
func (c *Client) Load(importPaths []string, filenames ...string) ([]MethodInfo, error) {
	if c.vu.State() != nil {
		return nil, errors.New("load must be called in the init context")
	}

	initEnv := c.vu.InitEnv()
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
	return c.convertToMethodInfo(fdset)
}

// Connect is a block dial to the gRPC server at the given address (host:port)
func (c *Client) Connect(addr string, params map[string]interface{}) (bool, error) {
	state := c.vu.State()
	if state == nil {
		return false, errConnectInInitContext
	}

	p, err := c.parseConnectParams(params)
	if err != nil {
		return false, err
	}

	opts := make([]grpc.DialOption, 0, 2)

	if !p.IsPlaintext {
		tlsCfg := state.TLSConfig.Clone()
		tlsCfg.NextProtos = []string{"h2"}

		// TODO(rogchap): Would be good to add support for custom RootCAs (self signed)

		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	if ua := state.Options.UserAgent; ua.Valid {
		opts = append(opts, grpc.WithUserAgent(ua.ValueOrZero()))
	}

	dialer := func(ctx context.Context, addr string) (net.Conn, error) {
		return state.Dialer.DialContext(ctx, "tcp", addr)
	}
	opts = append(opts, grpc.WithContextDialer(dialer))

	ctx, cancel := context.WithTimeout(c.vu.Context(), p.Timeout)
	defer cancel()

	err = c.dial(ctx, addr, p.UseReflectionProtocol, opts...)
	return err != nil, err
}

// Invoke creates and calls a unary RPC by fully qualified method name
func (c *Client) Invoke(
	method string,
	req goja.Value,
	params map[string]interface{},
) (*Response, error) {
	rt := c.vu.Runtime()
	state := c.vu.State()
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

	p, err := c.parseParams(params)
	if err != nil {
		return nil, err
	}

	ctx := metadata.NewOutgoingContext(c.vu.Context(), metadata.New(nil))
	for param, strval := range p.Metadata {
		ctx = metadata.AppendToOutgoingContext(ctx, param, strval)
	}

	tags := state.CloneTags()
	for k, v := range p.Tags {
		tags[k] = v
	}

	if state.Options.SystemTags.Has(metrics.TagURL) {
		tags["url"] = fmt.Sprintf("%s%s", c.conn.Target(), method)
	}
	parts := strings.Split(method[1:], "/")
	if state.Options.SystemTags.Has(metrics.TagService) {
		tags["service"] = parts[0]
	}
	if state.Options.SystemTags.Has(metrics.TagMethod) {
		tags["method"] = parts[1]
	}

	// Only set the name system tag if the user didn't explicitly set it beforehand
	if _, ok := tags["name"]; !ok && state.Options.SystemTags.Has(metrics.TagName) {
		tags["name"] = method
	}

	ctx = withTags(ctx, tags)

	reqdm := dynamicpb.NewMessage(md.Input())
	{
		b, err := req.ToObject(rt).MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("unable to serialise request object: %w", err)
		}
		if err := protojson.Unmarshal(b, reqdm); err != nil {
			return nil, fmt.Errorf("unable to serialise request object to protocol buffer: %w", err)
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, p.Timeout)
	defer cancel()

	resp := dynamicpb.NewMessage(md.Output())
	header, trailer := metadata.New(nil), metadata.New(nil)
	err = c.conn.Invoke(reqCtx, method, reqdm, resp, grpc.Header(&header), grpc.Trailer(&trailer))

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

func (c *Client) convertToMethodInfo(fdset *descriptorpb.FileDescriptorSet) ([]MethodInfo, error) {
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
	appendMethodInfo := func(
		fd protoreflect.FileDescriptor,
		sd protoreflect.ServiceDescriptor,
		md protoreflect.MethodDescriptor,
	) {
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
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		sds := fd.Services()
		for i := 0; i < sds.Len(); i++ {
			sd := sds.Get(i)
			mds := sd.Methods()
			for j := 0; j < mds.Len(); j++ {
				md := mds.Get(j)
				appendMethodInfo(fd, sd, md)
			}
		}
		return true
	})
	return rtn, nil
}

func (c *Client) dial(
	ctx context.Context,
	addr string,
	reflect bool,
	options ...grpc.DialOption,
) error {
	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
		grpc.WithStatsHandler(statsHandler{vu: c.vu}),
		grpc.WithReturnConnectionError(),
	}
	opts = append(opts, options...)

	var err error
	c.conn, err = grpc.DialContext(ctx, addr, opts...)
	if err != nil {
		return err
	}

	if !reflect {
		return nil
	}

	return c.reflect(ctx)
}

// reflect will use the grpc reflection api to make the file descriptors available to request.
// It is called in the connect function the first time the Client.Connect function is called.
func (c *Client) reflect(ctx context.Context) error {
	client := reflectpb.NewServerReflectionClient(c.conn)
	methodClient, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return fmt.Errorf("can't get server info: %w", err)
	}
	req := &reflectpb.ServerReflectionRequest{
		MessageRequest: &reflectpb.ServerReflectionRequest_ListServices{},
	}
	resp, err := sendReceive(methodClient, req)
	if err != nil {
		return fmt.Errorf("can't list services: %w", err)
	}
	listResp := resp.GetListServicesResponse()
	if listResp == nil {
		return fmt.Errorf("can't list services, nil response")
	}
	fdset, err := resolveServiceFileDescriptors(methodClient, listResp)
	if err != nil {
		return fmt.Errorf("can't resolve services' file descriptors: %w", err)
	}
	_, err = c.convertToMethodInfo(fdset)
	if err != nil {
		err = fmt.Errorf("can't convert method info: %w", err)
	}
	return err
}

type params struct {
	Metadata map[string]string
	Tags     map[string]string
	Timeout  time.Duration
}

func (c *Client) parseParams(raw map[string]interface{}) (params, error) {
	p := params{
		Timeout: 1 * time.Minute,
	}
	for k, v := range raw {
		switch k {
		case "headers":
			c.vu.State().Logger.Warn("The headers property is deprecated, replace it with the metadata property, please.")
			fallthrough
		case "metadata":
			p.Metadata = make(map[string]string)

			rawHeaders, ok := v.(map[string]interface{})
			if !ok {
				return p, errors.New("metadata must be an object with key-value pairs")
			}
			for hk, kv := range rawHeaders {
				// TODO(rogchap): Should we manage a string slice?
				strval, ok := kv.(string)
				if !ok {
					return p, fmt.Errorf("metadata %q value must be a string", hk)
				}
				p.Metadata[hk] = strval
			}
		case "tags":
			p.Tags = make(map[string]string)

			rawTags, ok := v.(map[string]interface{})
			if !ok {
				return p, errors.New("tags must be an object with key-value pairs")
			}
			for tk, tv := range rawTags {
				strVal, ok := tv.(string)
				if !ok {
					return p, fmt.Errorf("tag %q value must be a string", tk)
				}
				p.Tags[tk] = strVal
			}
		case "timeout":
			var err error
			p.Timeout, err = types.GetDurationValue(v)
			if err != nil {
				return p, fmt.Errorf("invalid timeout value: %w", err)
			}
		default:
			return p, fmt.Errorf("unknown param: %q", k)
		}
	}
	return p, nil
}

type connectParams struct {
	IsPlaintext           bool
	UseReflectionProtocol bool
	Timeout               time.Duration
}

func (c *Client) parseConnectParams(raw map[string]interface{}) (connectParams, error) {
	params := connectParams{
		IsPlaintext:           false,
		UseReflectionProtocol: false,
		Timeout:               time.Minute,
	}
	for k, v := range raw {
		switch k {
		case "plaintext":
			var ok bool
			params.IsPlaintext, ok = v.(bool)
			if !ok {
				return params, fmt.Errorf("invalid plaintext value: '%#v', it needs to be boolean", v)
			}
		case "timeout":
			var err error
			params.Timeout, err = types.GetDurationValue(v)
			if err != nil {
				return params, fmt.Errorf("invalid timeout value: %w", err)
			}
		case "reflect":
			var ok bool
			params.UseReflectionProtocol, ok = v.(bool)
			if !ok {
				return params, fmt.Errorf("invalid reflect value: '%#v', it needs to be boolean", v)
			}

		default:
			return params, fmt.Errorf("unknown connect param: %q", k)
		}
	}
	return params, nil
}

type statsHandler struct {
	vu modules.VU
}

// TagConn implements the grpcstats.Handler interface
func (statsHandler) TagConn(ctx context.Context, _ *grpcstats.ConnTagInfo) context.Context {
	// noop
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
		debugStat(stat, logger, httpDebugOption)
	}
}

// sendReceiver is a smaller interface for decoupling
// from `reflectpb.ServerReflection_ServerReflectionInfoClient`,
// that has the dependency from `grpc.ClientStream`,
// which is too much in the case the requirement is to just make a reflection's request.
// It makes the API more restricted and with a controlled surface,
// in this way the testing should be easier also.
type sendReceiver interface {
	Send(*reflectpb.ServerReflectionRequest) error
	Recv() (*reflectpb.ServerReflectionResponse, error)
}

// sendReceive sends a request to a reflection client and,
// receives a response.
func sendReceive(
	client sendReceiver,
	req *reflectpb.ServerReflectionRequest,
) (*reflectpb.ServerReflectionResponse, error) {
	if err := client.Send(req); err != nil {
		return nil, fmt.Errorf("can't send request: %w", err)
	}
	resp, err := client.Recv()
	if err != nil {
		return nil, fmt.Errorf("can't receive response: %w", err)
	}
	return resp, nil
}

type fileDescriptorLookupKey struct {
	Package string
	Name    string
}

func resolveServiceFileDescriptors(
	client sendReceiver,
	res *reflectpb.ListServiceResponse,
) (*descriptorpb.FileDescriptorSet, error) {
	services := res.GetService()
	seen := make(map[fileDescriptorLookupKey]bool, len(services))
	fdset := &descriptorpb.FileDescriptorSet{
		File: make([]*descriptorpb.FileDescriptorProto, 0, len(services)),
	}

	for _, service := range services {
		req := &reflectpb.ServerReflectionRequest{
			MessageRequest: &reflectpb.ServerReflectionRequest_FileContainingSymbol{
				FileContainingSymbol: service.GetName(),
			},
		}
		resp, err := sendReceive(client, req)
		if err != nil {
			return nil, fmt.Errorf("can't get method on service %q: %w", service, err)
		}
		fdResp := resp.GetFileDescriptorResponse()
		for _, raw := range fdResp.GetFileDescriptorProto() {
			var fdp descriptorpb.FileDescriptorProto
			if err = proto.Unmarshal(raw, &fdp); err != nil {
				return nil, fmt.Errorf("can't unmarshal proto on service %q: %w", service, err)
			}
			fdkey := fileDescriptorLookupKey{
				Package: *fdp.Package,
				Name:    *fdp.Name,
			}
			if seen[fdkey] {
				// When a proto file contains declarations for multiple services
				// then the same proto file is returned multiple times,
				// this prevents adding the returned proto file as a duplicate.
				continue
			}
			seen[fdkey] = true
			fdset.File = append(fdset.File, &fdp)
		}
	}
	return fdset, nil
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
