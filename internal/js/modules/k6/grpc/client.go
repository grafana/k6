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

	"go.k6.io/k6/internal/lib/netext/grpcext"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"

	"github.com/grafana/sobek"
	"github.com/jhump/protoreflect/desc"            //nolint:staticcheck // FIXME: #4035
	"github.com/jhump/protoreflect/desc/protoparse" //nolint:staticcheck // FIXME: #4035
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

	for i, s := range importPaths {
		// Clean file scheme as it is the only supported scheme and the following APIs do not support them
		importPaths[i] = strings.TrimPrefix(s, "file://")
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

// Connect is a block dial to the gRPC server at the given address (host:port)
func (c *Client) Connect(addr string, params sobek.Value) (bool, error) {
	state := c.vu.State()
	if state == nil {
		return false, common.NewInitContextError("connecting to a gRPC server in the init context is not supported")
	}

	p, err := newConnectParams(c.vu, params)
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
	c.conn, err = grpcext.Dial(ctx, addr, opts...)
	if err != nil {
		return false, err
	}

	if !p.UseReflectionProtocol {
		return true, nil
	}

	ctx = metadata.NewOutgoingContext(ctx, p.ReflectionMetadata)

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
	req sobek.Value,
	params sobek.Value,
) (*grpcext.InvokeResponse, error) {
	grpcReq, err := c.buildInvokeRequest(method, req, params)
	if err != nil {
		return nil, err
	}

	return c.conn.Invoke(c.vu.Context(), grpcReq)
}

// AsyncInvoke creates and calls a unary RPC by fully qualified method name asynchronously
func (c *Client) AsyncInvoke(
	method string,
	req sobek.Value,
	params sobek.Value,
) (*sobek.Promise, error) {
	grpcReq, err := c.buildInvokeRequest(method, req, params)

	promise, resolve, reject := c.vu.Runtime().NewPromise()
	if err != nil {
		err = reject(err)
		return promise, err
	}

	callback := c.vu.RegisterCallback()
	go func() {
		res, err := c.conn.Invoke(c.vu.Context(), grpcReq)

		callback(func() error {
			if err != nil {
				return reject(err)
			}
			return resolve(res)
		})
	}()

	return promise, nil
}

// buildInvokeRequest creates a new InvokeRequest from the given method name, request object and parameters
func (c *Client) buildInvokeRequest(
	method string,
	req sobek.Value,
	params sobek.Value,
) (grpcext.InvokeRequest, error) {
	grpcReq := grpcext.InvokeRequest{}

	state := c.vu.State()
	if state == nil {
		return grpcReq, common.NewInitContextError("invoking RPC methods in the init context is not supported")
	}
	if c.conn == nil {
		return grpcReq, errors.New("no gRPC connection, you must call connect first")
	}
	if method == "" {
		return grpcReq, errors.New("method to invoke cannot be empty")
	}
	if method[0] != '/' {
		method = "/" + method
	}
	methodDesc := c.mds[method]
	if methodDesc == nil {
		return grpcReq, fmt.Errorf("method %q not found in file descriptors", method)
	}

	p, err := newCallParams(c.vu, params)
	if err != nil {
		return grpcReq, fmt.Errorf("invalid GRPC's client.invoke() parameters: %w", err)
	}

	// k6 GRPC Invoke's default timeout is 2 minutes
	if p.Timeout == time.Duration(0) {
		p.Timeout = 2 * time.Minute
	}

	if req == nil {
		return grpcReq, errors.New("request cannot be nil")
	}
	b, err := req.ToObject(c.vu.Runtime()).MarshalJSON()
	if err != nil {
		return grpcReq, fmt.Errorf("unable to serialise request object: %w", err)
	}

	p.SetSystemTags(state, c.addr, method)

	return grpcext.InvokeRequest{
		Method:                 method,
		MethodDescriptor:       methodDesc,
		Timeout:                p.Timeout,
		DiscardResponseMessage: p.DiscardResponseMessage,
		Message:                b,
		TagsAndMeta:            &p.TagsAndMeta,
		Metadata:               p.Metadata,
	}, nil
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

// MethodInfo holds information on any parsed method descriptors that can be used by the Sobek VM
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

// sanitizeMethodName
func sanitizeMethodName(name string) string {
	if name == "" {
		return name
	}

	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}

	return name
}

// getMethodDescriptor sanitize it, and gets GRPC method descriptor or an error if not found
func (c *Client) getMethodDescriptor(method string) (protoreflect.MethodDescriptor, error) {
	method = sanitizeMethodName(method)

	if method == "" {
		return nil, errors.New("method to invoke cannot be empty")
	}

	methodDesc := c.mds[method]

	if methodDesc == nil {
		return nil, fmt.Errorf("method %q not found in file descriptors", method)
	}

	return methodDesc, nil
}
