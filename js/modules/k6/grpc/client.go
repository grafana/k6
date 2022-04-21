package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib/netext/grpcext"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/metrics"

	"github.com/dop251/goja"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Client represents a gRPC client that can be used to make RPC requests
type Client struct {
	mds  map[string]protoreflect.MethodDescriptor
	conn *grpcext.Conn
	vu   modules.VU
	addr string
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
		return false, common.NewInitContextError("connecting to a gRPC server in the init context is not supported")
	}

	p, err := c.parseConnectParams(params)
	if err != nil {
		return false, err
	}

	opts := grpcext.DefaultOptions(c.vu)

	var tcred credentials.TransportCredentials
	if !p.IsPlaintext {
		tlsCfg := state.TLSConfig.Clone()
		tlsCfg.NextProtos = []string{"h2"}

		// TODO(rogchap): Would be good to add support for custom RootCAs (self signed)
		tcred = credentials.NewTLS(tlsCfg)
	} else {
		tcred = insecure.NewCredentials()
	}
	opts = append(opts, grpc.WithTransportCredentials(tcred))

	if ua := state.Options.UserAgent; ua.Valid {
		opts = append(opts, grpc.WithUserAgent(ua.ValueOrZero()))
	}

	ctx, cancel := context.WithTimeout(c.vu.Context(), p.Timeout)
	defer cancel()

	c.addr = addr
	c.conn, err = grpcext.Dial(ctx, addr, opts...)
	if err != nil {
		return false, err
	}

	if !p.UseReflectionProtocol {
		return true, nil
	}
	fdset, err := c.conn.Reflect(ctx)
	if err != nil {
		return false, err
	}
	_, err = c.convertToMethodInfo(fdset)
	if err != nil {
		return false, fmt.Errorf("can't convert method info: %w", err)
	}

	return true, err
}

// Invoke creates and calls a unary RPC by fully qualified method name
func (c *Client) Invoke(
	method string,
	req goja.Value,
	params map[string]interface{},
) (*grpcext.Response, error) {
	state := c.vu.State()
	if state == nil {
		return nil, common.NewInitContextError("invoking RPC methods in the init context is not supported")
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
	methodDesc := c.mds[method]
	if methodDesc == nil {
		return nil, fmt.Errorf("method %q not found in file descriptors", method)
	}

	p, err := c.parseParams(params)
	if err != nil {
		return nil, err
	}

	b, err := req.ToObject(c.vu.Runtime()).MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("unable to serialise request object: %w", err)
	}

	md := metadata.New(nil)
	for param, strval := range p.Metadata {
		md.Append(param, strval)
	}

	ctx, cancel := context.WithTimeout(c.vu.Context(), p.Timeout)
	defer cancel()

	tags := state.CloneTags()
	for k, v := range p.Tags {
		tags[k] = v
	}

	if state.Options.SystemTags.Has(metrics.TagURL) {
		tags["url"] = fmt.Sprintf("%s%s", c.addr, method)
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

	reqmsg := grpcext.Request{
		MethodDescriptor: methodDesc,
		Message:          b,
		Tags:             tags,
	}

	return c.conn.Invoke(ctx, method, md, reqmsg)
}

// Close will close the client gRPC connection
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil

	return err
}

// MethodInfo holds information on any parsed method descriptors that can be used by the goja VM
type MethodInfo struct {
	Package         string
	Service         string
	FullMethod      string
	grpc.MethodInfo `json:"-" js:"-"`
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
