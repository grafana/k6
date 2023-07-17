package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
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

// LoadProtoset will parse the given protoset file (serialized FileDescriptorSet) and make the file
// descriptors available to request.
func (c *Client) LoadProtoset(protosetPath string) ([]MethodInfo, error) {
	if c.vu.State() != nil {
		return nil, errors.New("load must be called in the init context")
	}

	initEnv := c.vu.InitEnv()
	if initEnv == nil {
		return nil, errors.New("missing init environment")
	}

	absFilePath := initEnv.GetAbsFilePath(protosetPath)
	fdsetFile, err := initEnv.FileSystems["file"].Open(absFilePath)
	if err != nil {
		return nil, fmt.Errorf("couldn't open protoset: %w", err)
	}

	defer func() { _ = fdsetFile.Close() }()
	fdsetBytes, err := io.ReadAll(fdsetFile)
	if err != nil {
		return nil, fmt.Errorf("couldn't read protoset: %w", err)
	}

	fdset := &descriptorpb.FileDescriptorSet{}
	if err = proto.Unmarshal(fdsetBytes, fdset); err != nil {
		return nil, fmt.Errorf("couldn't unmarshal protoset file %s: %w", protosetPath, err)
	}

	return c.convertToMethodInfo(fdset)
}

// Note: this function was lifted from `lib/options.go`
func decryptPrivateKey(key, password []byte) ([]byte, error) {
	block, _ := pem.Decode(key)
	if block == nil {
		return nil, errors.New("failed to decode PEM key")
	}

	blockType := block.Type
	if blockType == "ENCRYPTED PRIVATE KEY" {
		return nil, errors.New("encrypted pkcs8 formatted key is not supported")
	}
	/*
	   Even though `DecryptPEMBlock` has been deprecated since 1.16.x it is still
	   being used here because it is deprecated due to it not supporting *good* cryptography
	   ultimately though we want to support something so we will be using it for now.
	*/
	decryptedKey, err := x509.DecryptPEMBlock(block, password) //nolint:staticcheck
	if err != nil {
		return nil, err
	}
	key = pem.EncodeToMemory(&pem.Block{
		Type:  blockType,
		Bytes: decryptedKey,
	})
	return key, nil
}

func buildTLSConfig(parentConfig *tls.Config, certificate, key []byte, caCertificates [][]byte) (*tls.Config, error) {
	var cp *x509.CertPool
	if len(caCertificates) > 0 {
		cp, _ = x509.SystemCertPool()
		for i, caCert := range caCertificates {
			if ok := cp.AppendCertsFromPEM(caCert); !ok {
				return nil, fmt.Errorf("failed to append ca certificate [%d] from PEM", i)
			}
		}
	}

	// Ignoring 'TLS MinVersion is too low' because this tls.Config will inherit MinValue and MaxValue
	// from the vu state tls.Config

	//nolint:golint,gosec
	tlsCfg := &tls.Config{
		CipherSuites:       parentConfig.CipherSuites,
		InsecureSkipVerify: parentConfig.InsecureSkipVerify,
		MinVersion:         parentConfig.MinVersion,
		MaxVersion:         parentConfig.MaxVersion,
		Renegotiation:      parentConfig.Renegotiation,
		RootCAs:            cp,
	}
	if len(certificate) > 0 && len(key) > 0 {
		cert, err := tls.X509KeyPair(certificate, key)
		if err != nil {
			return nil, fmt.Errorf("failed to append certificate from PEM: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}
	return tlsCfg, nil
}

func buildTLSConfigFromMap(parentConfig *tls.Config, tlsConfigMap map[string]interface{}) (*tls.Config, error) {
	var cert, key, pass []byte
	var ca [][]byte
	var err error
	if certstr, ok := tlsConfigMap["cert"].(string); ok {
		cert = []byte(certstr)
	}
	if keystr, ok := tlsConfigMap["key"].(string); ok {
		key = []byte(keystr)
	}
	if passwordStr, ok := tlsConfigMap["password"].(string); ok {
		pass = []byte(passwordStr)
		if len(pass) > 0 {
			if key, err = decryptPrivateKey(key, pass); err != nil {
				return nil, err
			}
		}
	}
	if cas, ok := tlsConfigMap["cacerts"]; ok {
		var caCertsArray []interface{}
		if caCertsArray, ok = cas.([]interface{}); ok {
			ca = make([][]byte, len(caCertsArray))
			for i, entry := range caCertsArray {
				var entryStr string
				if entryStr, ok = entry.(string); ok {
					ca[i] = []byte(entryStr)
				}
			}
		} else if caCertStr, caCertStrOk := cas.(string); caCertStrOk {
			ca = [][]byte{[]byte(caCertStr)}
		}
	}
	return buildTLSConfig(parentConfig, cert, key, ca)
}

// IsSameConnection compares this Client to another Client's raw connection for equality
// See: connectParams.ConnectionSharing parameter
func (c *Client) IsSameConnection(v goja.Value) (bool, error) {
	rt := c.vu.Runtime()

	if common.IsNullish(v) {
		return false, nil
	}

	client, ok := v.ToObject(rt).Export().(*Client)
	if !ok {
		return false, errors.New("parameter must a 'Client'")
	}

	if client.conn == nil || c.conn == nil {
		return false, errors.New("no gRPC connection, you must call connect first")
	}

	return c.conn.Equals(client.conn), nil
}

// Connect is a block dial to the gRPC server at the given address (host:port)
func (c *Client) Connect(addr string, params map[string]interface{}) (bool, error) {
	state := c.vu.State()
	if state == nil {
		return false, common.NewInitContextError("connecting to a gRPC server in the init context is not supported")
	}

	p, err := c.parseConnectParams(params)
	if err != nil {
		return false, fmt.Errorf("invalid grpc.connect() parameters: %w", err)
	}

	opts := grpcext.DefaultOptions(c.vu.State)

	var tcred credentials.TransportCredentials
	if !p.IsPlaintext {
		tlsCfg := state.TLSConfig.Clone()
		if len(p.TLS) > 0 {
			if tlsCfg, err = buildTLSConfigFromMap(tlsCfg, p.TLS); err != nil {
				return false, err
			}
		}
		tlsCfg.NextProtos = []string{"h2"}

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

	if p.MaxReceiveSize > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(int(p.MaxReceiveSize))))
	}

	if p.MaxSendSize > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(int(p.MaxSendSize))))
	}

	c.addr = addr

	if p.ConnectionSharing <= 1 {
		c.conn, err = grpcext.Dial(ctx, addr, opts...)
	} else {
		c.conn, err = grpcext.DialShared(ctx, addr, uint64(p.ConnectionSharing), opts...)
	}
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
	params goja.Value,
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

	p, err := c.parseInvokeParams(params)
	if err != nil {
		return nil, fmt.Errorf("invalid grpc.invoke() parameters: %w", err)
	}

	if req == nil {
		return nil, errors.New("request cannot be nil")
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

	if state.Options.SystemTags.Has(metrics.TagURL) {
		p.TagsAndMeta.SetSystemTagOrMeta(metrics.TagURL, fmt.Sprintf("%s%s", c.addr, method))
	}
	parts := strings.Split(method[1:], "/")
	p.TagsAndMeta.SetSystemTagOrMetaIfEnabled(state.Options.SystemTags, metrics.TagService, parts[0])
	p.TagsAndMeta.SetSystemTagOrMetaIfEnabled(state.Options.SystemTags, metrics.TagMethod, parts[1])

	// Only set the name system tag if the user didn't explicitly set it beforehand
	if _, ok := p.TagsAndMeta.Tags.Get("name"); !ok {
		p.TagsAndMeta.SetSystemTagOrMetaIfEnabled(state.Options.SystemTags, metrics.TagName, method)
	}

	reqmsg := grpcext.Request{
		MethodDescriptor: methodDesc,
		Message:          b,
		TagsAndMeta:      &p.TagsAndMeta,
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
		messages := fd.Messages()

		stack := make([]protoreflect.MessageDescriptor, 0, messages.Len())
		for i := 0; i < messages.Len(); i++ {
			stack = append(stack, messages.Get(i))
		}

		for len(stack) > 0 {
			message := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			_, errFind := protoregistry.GlobalTypes.FindMessageByName(message.FullName())
			if errors.Is(errFind, protoregistry.NotFound) {
				err = protoregistry.GlobalTypes.RegisterMessage(dynamicpb.NewMessageType(message))
				if err != nil {
					return false
				}
			}

			nested := message.Messages()
			for i := 0; i < nested.Len(); i++ {
				stack = append(stack, nested.Get(i))
			}
		}

		return true
	})
	if err != nil {
		return nil, err
	}
	return rtn, nil
}

type invokeParams struct {
	Metadata    map[string]string
	TagsAndMeta metrics.TagsAndMeta
	Timeout     time.Duration
}

func (c *Client) parseInvokeParams(paramsVal goja.Value) (*invokeParams, error) {
	result := &invokeParams{
		Timeout:     1 * time.Minute,
		TagsAndMeta: c.vu.State().Tags.GetCurrentValues(),
	}
	if paramsVal == nil || goja.IsUndefined(paramsVal) || goja.IsNull(paramsVal) {
		return result, nil
	}
	rt := c.vu.Runtime()
	params := paramsVal.ToObject(rt)
	for _, k := range params.Keys() {
		switch k {
		case "headers":
			c.vu.State().Logger.Warn("The headers property is deprecated, replace it with the metadata property, please.")
			fallthrough
		case "metadata":
			result.Metadata = make(map[string]string)
			v := params.Get(k).Export()
			rawHeaders, ok := v.(map[string]interface{})
			if !ok {
				return result, errors.New("metadata must be an object with key-value pairs")
			}
			for hk, kv := range rawHeaders {
				// TODO(rogchap): Should we manage a string slice?
				strval, ok := kv.(string)
				if !ok {
					return result, fmt.Errorf("metadata %q value must be a string", hk)
				}
				result.Metadata[hk] = strval
			}
		case "tags":
			if err := common.ApplyCustomUserTags(rt, &result.TagsAndMeta, params.Get(k)); err != nil {
				return result, fmt.Errorf("metric tags: %w", err)
			}
		case "timeout":
			var err error
			v := params.Get(k).Export()
			result.Timeout, err = types.GetDurationValue(v)
			if err != nil {
				return result, fmt.Errorf("invalid timeout value: %w", err)
			}
		default:
			return result, fmt.Errorf("unknown param: %q", k)
		}
	}
	return result, nil
}

type connectParams struct {
	IsPlaintext           bool
	UseReflectionProtocol bool
	Timeout               time.Duration
	MaxReceiveSize        int64
	MaxSendSize           int64
	ConnectionSharing     int64
	TLS                   map[string]interface{}
}

func (c *Client) parseConnectParams(raw map[string]interface{}) (connectParams, error) {
	params := connectParams{
		IsPlaintext:           false,
		UseReflectionProtocol: false,
		Timeout:               time.Minute,
		MaxReceiveSize:        0,
		MaxSendSize:           0,
		ConnectionSharing:     0,
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
		case "maxReceiveSize":
			var ok bool
			params.MaxReceiveSize, ok = v.(int64)
			if !ok {
				return params, fmt.Errorf("invalid maxReceiveSize value: '%#v', it needs to be an integer", v)
			}
			if params.MaxReceiveSize < 0 {
				return params, fmt.Errorf("invalid maxReceiveSize value: '%#v, it needs to be a positive integer", v)
			}
		case "maxSendSize":
			var ok bool
			params.MaxSendSize, ok = v.(int64)
			if !ok {
				return params, fmt.Errorf("invalid maxSendSize value: '%#v', it needs to be an integer", v)
			}
			if params.MaxSendSize < 0 {
				return params, fmt.Errorf("invalid maxSendSize value: '%#v, it needs to be a positive integer", v)
			}
		case "connectionSharing":
			var (
				ok                    bool
				connectionSharingBool bool
			)

			connectionSharingBool, ok = v.(bool)
			if ok {
				params.ConnectionSharing = 1
				if connectionSharingBool {
					params.ConnectionSharing = 100
				}
			} else {
				params.ConnectionSharing, ok = v.(int64)
				if !ok {
					return params, fmt.Errorf("invalid connectionSharing value: '%#v', it needs to be boolean or a"+
						" positive integer > 1", v)
				}
				if params.ConnectionSharing <= 1 {
					return params, fmt.Errorf("invalid connectionSharing value: '%#v', it needs to be boolean or a"+
						" positive integer > 1", v)
				}
			}
		case "tls":
			var ok bool
			params.TLS, ok = v.(map[string]interface{})

			if !ok {
				return params, fmt.Errorf("invalid tls value: '%#v', expected (optional) keys: cert, key, password, and cacerts", v)
			}
			// optional map keys below
			if cert, certok := params.TLS["cert"]; certok {
				if _, ok = cert.(string); !ok {
					return params, fmt.Errorf("invalid tls cert value: '%#v', it needs to be a PEM formatted string", v)
				}
			}
			if key, keyok := params.TLS["key"]; keyok {
				if _, ok = key.(string); !ok {
					return params, fmt.Errorf("invalid tls key value: '%#v', it needs to be a PEM formatted string", v)
				}
			}
			if pass, passok := params.TLS["password"]; passok {
				if _, ok = pass.(string); !ok {
					return params, fmt.Errorf("invalid tls password value: '%#v', it needs to be a string", v)
				}
			}
			if cacerts, cacertsok := params.TLS["cacerts"]; cacertsok {
				var cacertsArray []interface{}
				if cacertsArray, ok = cacerts.([]interface{}); ok {
					for _, cacertsArrayEntry := range cacertsArray {
						if _, ok = cacertsArrayEntry.(string); !ok {
							return params, fmt.Errorf("invalid tls cacerts value: '%#v',"+
								" it needs to be a string or an array of PEM formatted strings", v)
						}
					}
				} else if _, ok = cacerts.(string); !ok {
					return params, fmt.Errorf("invalid tls cacerts value: '%#v',"+
						" it needs to be a string or an array of PEM formatted strings", v)
				}
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
