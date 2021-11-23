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

package twirp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/twitchtv/twirp/ctxsetters"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	grpcstats "google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/twitchtv/twirp"

	//nolint: staticcheck
	protoV1 "github.com/golang/protobuf/proto"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/types"
	"go.k6.io/k6/stats"

)

//nolint: lll
var (
	errInvokeRPCInInitContext = common.NewInitContextError("invoking twirp methods in the init context is not supported")
	errConnectInInitContext   = common.NewInitContextError("initializing a twirp client in the init context is not supported")
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type twirpClient struct {
	client         HTTPClient
	interceptor    twirp.Interceptor
	isJson         bool
	serviceBaseUrl string
	pathPrefix     string
}

// Client represents a gRPC client that can be used to make RPC requests
type Client struct {
	mds  map[string]protoreflect.MethodDescriptor
	conn *twirpClient
}

// XClient represents the Client constructor (e.g. `new grpc.Client()`) and
// creates a new gPRC client object that can load protobuf definitions, connect
// to servers and invoke RPC methods.
func (*Twirp) XClient(ctxPtr *context.Context) interface{} {
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
	Status   twirp.ErrorCode
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
	return c.convertToMethodInfo(fdset)
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

// sanitizeBaseURL parses the the baseURL, and adds the "http" scheme if needed.
// If the URL is unparsable, the baseURL is returned unchaged.
func sanitizeBaseURL(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL // invalid URL will fail later when making requests
	}
	if u.Scheme == "" {
		u.Scheme = "http"
	}
	return u.String()
}

// nolint:funlen,cyclop
func (c *Client) Init(ctxPtr *context.Context, baseURL string, params map[string]interface{}) (bool, error) {
	state := lib.GetState(*ctxPtr)
	if state == nil {
		return false, errConnectInInitContext
	}
	pathPrefix, isJson := "/twirp", false

	for k, v := range params {
		switch k {
		case "pathPrefix":
			var ok bool
			pathPrefix, ok = v.(string)
			if !ok {
				return false, fmt.Errorf("invalid pathPrefix value: '%#v', it needs to be string", v)
			}
		case "isJson":
			var ok bool
			isJson, ok = v.(bool)
			if !ok {
				return false, fmt.Errorf("invalid isJson value: '%#v', it needs to be boolean", v)
			}

		default:
			return false, fmt.Errorf("unknown connect param: %q", k)
		}
	}

	serviceBaseUrl := sanitizeBaseURL(baseURL)

	client := withoutRedirects(&http.Client{})

	c.conn = &twirpClient{
		client,
		twirp.ChainInterceptors(),
		isJson,
		serviceBaseUrl,
		pathPrefix,
	}

	return true, nil
}

// baseServicePath composes the path prefix for the service (without <Method>).
// e.g.: baseServicePath("/twirp", "my.pkg", "MyService")
//       returns => "/twirp/my.pkg.MyService/"
// e.g.: baseServicePath("", "", "MyService")
//       returns => "/MyService/"
func baseServicePath(prefix, pkg, service string) string {
	fullServiceName := service
	if pkg != "" {
		fullServiceName = pkg + "." + service
	}
	return path.Join("/", prefix, fullServiceName) + "/"
}

// Invoke creates and calls a unary RPC by fully qualified method name
//nolint: funlen,gocognit,gocyclo,cyclop
func (c *Client) Invoke(
	ctxPtr *context.Context,
	pkg string,
	service string,
	method string,
	req goja.Value,
	params map[string]interface{},
) (*Response, error) {
	ctx := *ctxPtr
	rt := common.GetRuntime(ctx)
	state := lib.GetState(ctx)
	if state == nil {
		return nil, errInvokeRPCInInitContext
	}
	if c.conn == nil {
		return nil, errors.New("no twirp client, you must call init first")
	}
	if method == "" {
		return nil, errors.New("method to invoke cannot be empty")
	}

	rpcName := baseServicePath("", pkg, service) + method
	serviceUrl := c.conn.serviceBaseUrl + baseServicePath(c.conn.pathPrefix, pkg, service) + method

	md := c.mds[rpcName]
	if md == nil {
		return nil, fmt.Errorf("rpc method %q not found in file descriptors", rpcName)
	}

	tags := state.CloneTags()
	timeout := 60 * time.Second



	requestCtx := ctxsetters.WithPackageName(ctx, pkg)
	requestCtx = ctxsetters.WithMethodName(requestCtx, method)
	requestCtx = ctxsetters.WithServiceName(requestCtx, service)

	for k, v := range params {
		switch k {
		case "headers":
			rawHeaders, ok := v.(map[string]interface{})
			httpHeaders := &http.Header {}
			if !ok {
				return nil, errors.New("headers must be an object with key-value pairs")
			}
			for hk, kv := range rawHeaders {
				strVal, ok := kv.(string)
				if !ok {
					return nil, fmt.Errorf("header %q value must be a string", hk)
				}
				httpHeaders.Set(hk, strVal)
			}
			ctx, err := twirp.WithHTTPRequestHeaders(ctx, *httpHeaders)
			if err != nil {
				return nil, err
			}
			requestCtx = ctx
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
		tags["url"] = serviceUrl
	}
	if state.Options.SystemTags.Has(stats.TagService) {
		tags["service"] = baseServicePath("", pkg, service)
	}
	if state.Options.SystemTags.Has(stats.TagMethod) {
		tags["method"] = method
	}

	// Only set the name system tag if the user didn't explicitly set it beforehand
	if _, ok := tags["name"]; !ok && state.Options.SystemTags.Has(stats.TagName) {
		tags["name"] = method
	}

	requestCtx = withTags(requestCtx, tags)

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


	resp := dynamicpb.NewMessage(md.Output())

	reqCtx, cancel := context.WithTimeout(requestCtx, timeout)
	defer cancel()

	_, err := doProtobufRequest(reqCtx, c.conn.client, serviceUrl, reqdm, resp)

	marshaler := protojson.MarshalOptions{EmitUnpopulated: true}

	var response Response

	if err != nil {
		twerr, ok := err.(twirp.Error)
		if !ok {
			twerr = twirp.InternalErrorWith(err)
		}
		response.Status = twerr.Code()
		raw, _ := json.Marshal(twerr)
		errMsg := make(map[string]interface{})
		_ = json.Unmarshal(raw, &errMsg)
		response.Error = errMsg
	}

	if resp != nil {
		raw, _ := marshaler.Marshal(resp)
		msg := make(map[string]interface{})
		_ = json.Unmarshal(raw, &msg)
		response.Message = msg
	}
	return &response, nil
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
					Metric: state.BuiltinMetrics.GRPCReqDuration,
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

//--------------------------------------------------------------------------------------------------------------
//--------------------------------------------------------------------------------------------------------------
//--------------------------------------------------------------------------------------------------------------
//--------------------------------------------------------------------------------------------------------------

// withoutRedirects makes sure that the POST request can not be redirected.
// The standard library will, by default, redirect requests (including POSTs) if it gets a 302 or
// 303 response, and also 301s in go1.8. It redirects by making a second request, changing the
// method to GET and removing the body. This produces very confusing error messages, so instead we
// set a redirect policy that always errors. This stops Go from executing the redirect.
//
// We have to be a little careful in case the user-provided http.Client has its own CheckRedirect
// policy - if so, we'll run through that policy first.
//
// Because this requires modifying the http.Client, we make a new copy of the client and return it.
func withoutRedirects(in *http.Client) *http.Client {
	clientCopy := *in
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if in.CheckRedirect != nil {
			// Run the input's redirect if it exists, in case it has side effects, but ignore any error it
			// returns, since we want to use ErrUseLastResponse.
			err := in.CheckRedirect(req, via)
			_ = err // Silly, but this makes sure generated code passes errcheck -blank, which some people use.
		}
		return http.ErrUseLastResponse
	}
	return &clientCopy
}

// JSON serialization for errors
type twerrJSON struct {
	Code string            `json:"code"`
	Msg  string            `json:"msg"`
	Meta map[string]string `json:"meta,omitempty"`
}

type wrappedError struct {
	prefix string
	cause  error
}

func (e *wrappedError) Error() string { return e.prefix + ": " + e.cause.Error() }
func (e *wrappedError) Unwrap() error { return e.cause } // for go1.13 + errors.Is/As
func (e *wrappedError) Cause() error  { return e.cause } // for github.com/pkg/errors

// wrapInternal wraps an error with a prefix as an Internal error.
// The original error cause is accessible by github.com/pkg/errors.Cause.
func wrapInternal(err error, prefix string) twirp.Error {
	return twirp.InternalErrorWith(&wrappedError{prefix: prefix, cause: err})
}

// getCustomHTTPReqHeaders retrieves a copy of any headers that are set in
// a context through the twirp.WithHTTPRequestHeaders function.
// If there are no headers set, or if they have the wrong type, nil is returned.
func getCustomHTTPReqHeaders(ctx context.Context) http.Header {
	header, ok := twirp.HTTPRequestHeaders(ctx)
	if !ok || header == nil {
		return nil
	}
	copied := make(http.Header)
	for k, vv := range header {
		if vv == nil {
			copied[k] = nil
			continue
		}
		copied[k] = make([]string, len(vv))
		copy(copied[k], vv)
	}
	return copied
}

// newRequest makes an http.Request from a client, adding common headers.
func newRequest(ctx context.Context, url string, reqBody io.Reader, contentType string) (*http.Request, error) {
	req, err := http.NewRequest("POST", url, reqBody)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	if customHeader := getCustomHTTPReqHeaders(ctx); customHeader != nil {
		req.Header = customHeader
	}
	req.Header.Set("Accept", contentType)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Twirp-Version", "v8.1.0")
	return req, nil
}

// twirpErrorFromIntermediary maps HTTP errors from non-twirp sources to twirp errors.
// The mapping is similar to gRPC: https://github.com/grpc/grpc/blob/master/doc/http-grpc-status-mapping.md.
// Returned twirp Errors have some additional metadata for inspection.
func twirpErrorFromIntermediary(status int, msg string, bodyOrLocation string) twirp.Error {
	var code twirp.ErrorCode
	if isHTTPRedirect(status) { // 3xx
		code = twirp.Internal
	} else {
		switch status {
		case 400: // Bad Request
			code = twirp.Internal
		case 401: // Unauthorized
			code = twirp.Unauthenticated
		case 403: // Forbidden
			code = twirp.PermissionDenied
		case 404: // Not Found
			code = twirp.BadRoute
		case 429: // Too Many Requests
			code = twirp.ResourceExhausted
		case 502, 503, 504: // Bad Gateway, Service Unavailable, Gateway Timeout
			code = twirp.Unavailable
		default: // All other codes
			code = twirp.Unknown
		}
	}

	twerr := twirp.NewError(code, msg)
	twerr = twerr.WithMeta("http_error_from_intermediary", "true") // to easily know if this error was from intermediary
	twerr = twerr.WithMeta("status_code", strconv.Itoa(status))
	if isHTTPRedirect(status) {
		twerr = twerr.WithMeta("location", bodyOrLocation)
	} else {
		twerr = twerr.WithMeta("body", bodyOrLocation)
	}
	return twerr
}

func isHTTPRedirect(status int) bool {
	return status >= 300 && status <= 399
}

// errorFromResponse builds a twirp.Error from a non-200 HTTP response.
// If the response has a valid serialized Twirp error, then it's returned.
// If not, the response status code is used to generate a similar twirp
// error. See twirpErrorFromIntermediary for more info on intermediary errors.
func errorFromResponse(resp *http.Response) twirp.Error {
	statusCode := resp.StatusCode
	statusText := http.StatusText(statusCode)

	if isHTTPRedirect(statusCode) {
		// Unexpected redirect: it must be an error from an intermediary.
		// Twirp clients don't follow redirects automatically, Twirp only handles
		// POST requests, redirects should only happen on GET and HEAD requests.
		location := resp.Header.Get("Location")
		msg := fmt.Sprintf("unexpected HTTP status code %d %q received, Location=%q", statusCode, statusText, location)
		return twirpErrorFromIntermediary(statusCode, msg, location)
	}

	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return wrapInternal(err, "failed to read server error response body")
	}

	var tj twerrJSON
	dec := json.NewDecoder(bytes.NewReader(respBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&tj); err != nil || tj.Code == "" {
		// Invalid JSON response; it must be an error from an intermediary.
		msg := fmt.Sprintf("Error from intermediary with HTTP status code %d %q", statusCode, statusText)
		return twirpErrorFromIntermediary(statusCode, msg, string(respBodyBytes))
	}

	errorCode := twirp.ErrorCode(tj.Code)
	if !twirp.IsValidErrorCode(errorCode) {
		msg := "invalid type returned from server error response: " + tj.Code
		return twirp.InternalError(msg).WithMeta("body", string(respBodyBytes))
	}

	twerr := twirp.NewError(errorCode, tj.Msg)
	for k, v := range tj.Meta {
		twerr = twerr.WithMeta(k, v)
	}
	return twerr
}

// doProtobufRequest makes a Protobuf request to the remote Twirp service.
func doProtobufRequest(ctx context.Context, client HTTPClient,
	url string, in *dynamicpb.Message, out *dynamicpb.Message,
) (_ context.Context, err error) {
	reqBodyBytes, err := proto.Marshal(in)
	if err != nil {
		return ctx, wrapInternal(err, "failed to marshal proto request")
	}
	reqBody := bytes.NewBuffer(reqBodyBytes)
	if err = ctx.Err(); err != nil {
		return ctx, wrapInternal(err, "aborted because context was done")
	}

	req, err := newRequest(ctx, url, reqBody, "application/protobuf")
	if err != nil {
		return ctx, wrapInternal(err, "could not build request")
	}

	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return ctx, wrapInternal(err, "failed to do request")
	}

	defer func() {
		cerr := resp.Body.Close()
		if err == nil && cerr != nil {
			err = wrapInternal(cerr, "failed to close response body")
		}
	}()

	if err = ctx.Err(); err != nil {
		return ctx, wrapInternal(err, "aborted because context was done")
	}

	if resp.StatusCode != 200 {
		return ctx, errorFromResponse(resp)
	}

	respBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ctx, wrapInternal(err, "failed to read response body")
	}
	if err = ctx.Err(); err != nil {
		return ctx, wrapInternal(err, "aborted because context was done")
	}

	if err = proto.Unmarshal(respBodyBytes, out); err != nil {
		return ctx, wrapInternal(err, "failed to unmarshal proto response")
	}
	return ctx, nil
}

// doJSONRequest makes a JSON request to the remote Twirp service.
func doJSONRequest(ctx context.Context, client HTTPClient, hooks *twirp.ClientHooks, url string, in, out proto.Message) (_ context.Context, err error) {
	marshaler := &protojson.MarshalOptions{UseProtoNames: true}
	reqBytes, err := marshaler.Marshal(in)
	if err != nil {
		return ctx, wrapInternal(err, "failed to marshal json request")
	}
	if err = ctx.Err(); err != nil {
		return ctx, wrapInternal(err, "aborted because context was done")
	}

	req, err := newRequest(ctx, url, bytes.NewReader(reqBytes), "application/json")
	if err != nil {
		return ctx, wrapInternal(err, "could not build request")
	}

	req = req.WithContext(ctx)
	resp, err := client.Do(req)
	if err != nil {
		return ctx, wrapInternal(err, "failed to do request")
	}

	defer func() {
		cerr := resp.Body.Close()
		if err == nil && cerr != nil {
			err = wrapInternal(cerr, "failed to close response body")
		}
	}()

	if err = ctx.Err(); err != nil {
		return ctx, wrapInternal(err, "aborted because context was done")
	}

	if resp.StatusCode != 200 {
		return ctx, errorFromResponse(resp)
	}

	d := json.NewDecoder(resp.Body)
	rawRespBody := json.RawMessage{}
	if err := d.Decode(&rawRespBody); err != nil {
		return ctx, wrapInternal(err, "failed to unmarshal json response")
	}
	unmarshaler := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err = unmarshaler.Unmarshal(rawRespBody, out); err != nil {
		return ctx, wrapInternal(err, "failed to unmarshal json response")
	}
	if err = ctx.Err(); err != nil {
		return ctx, wrapInternal(err, "aborted because context was done")
	}
	return ctx, nil
}
