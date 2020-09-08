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
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
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

// GRPC represents the gRPC protocol module for k6
type GRPC struct {
	StatusOK                 codes.Code `js:"StatusOK"`
	StatusCanceled           codes.Code `js:"StatusCanceled"`
	StatusUnknown            codes.Code `js:"StatusUnknown"`
	StatusInvalidArgument    codes.Code `js:"StatusInvalidArgument"`
	StatusDeadlineExceeded   codes.Code `js:"StatusDeadlineExceeded"`
	StatusNotFound           codes.Code `js:"StatusNotFound"`
	StatusAlreadyExists      codes.Code `js:"StatusAlreadyExists"`
	StatusPermissionDenied   codes.Code `js:"StatusPermissionDenied"`
	StatusResourceExhausted  codes.Code `js:"StatusResourceExhausted"`
	StatusFailedPrecondition codes.Code `js:"StatusFailedPrecondition"`
	StatusAborted            codes.Code `js:"StatusAborted"`
	StatusOutOfRange         codes.Code `js:"StatusOutOfRange"`
	StatusUnimplemented      codes.Code `js:"StatusUnimplemented"`
	StatusInternal           codes.Code `js:"StatusInternal"`
	StatusUnavailable        codes.Code `js:"StatusUnavailable"`
	StatusDataLoss           codes.Code `js:"StatusDataLoss"`
	StatusUnauthenticated    codes.Code `js:"StatusUnauthenticated"`
}

// New creates a new gRPC module
func New() *GRPC {
	return &GRPC{
		StatusOK:                 codes.OK,
		StatusCanceled:           codes.Canceled,
		StatusUnknown:            codes.Unknown,
		StatusInvalidArgument:    codes.InvalidArgument,
		StatusDeadlineExceeded:   codes.DeadlineExceeded,
		StatusNotFound:           codes.NotFound,
		StatusAlreadyExists:      codes.AlreadyExists,
		StatusPermissionDenied:   codes.PermissionDenied,
		StatusResourceExhausted:  codes.ResourceExhausted,
		StatusFailedPrecondition: codes.FailedPrecondition,
		StatusAborted:            codes.Aborted,
		StatusOutOfRange:         codes.OutOfRange,
		StatusUnimplemented:      codes.Unimplemented,
		StatusInternal:           codes.Internal,
		StatusUnavailable:        codes.Unavailable,
		StatusDataLoss:           codes.DataLoss,
		StatusUnauthenticated:    codes.Unauthenticated,
	}
}

var (
	errInvokeRPCInInitContext = common.NewInitContextError("Invoking RPC methods in the init context is not supported")
	errConnectInInitContext   = common.NewInitContextError("Connecting to a gRPC server in the init context is not supported")
)

// Client reprecents a gRPC client that can be used to make RPC requests
type Client struct {
	mds map[string]*desc.MethodDescriptor

	sampleTags    *stats.SampleTags
	samplesOutput chan<- stats.SampleContainer

	conn *grpc.ClientConn
}

// NewClient creates a new gPRC client to make invoke RPC methods.
func (*GRPC) NewClient(ctxPtr *context.Context /* TODO(rogchap): any options?*/) interface{} {
	rt := common.GetRuntime(*ctxPtr)
	return common.Bind(rt, &Client{}, ctxPtr)
}

// Load will parse the given proto files and make the file descriptors avaliable to request. This can only be called once;
// subsequent calls to Load will be a noop.
func (c *Client) Load(ctxPtr *context.Context, importPaths []string, filenames ...string) error {
	if lib.GetState(*ctxPtr) != nil {
		return errors.New("load must be called in the init context")
	}

	parser := protoparse.Parser{
		ImportPaths:      importPaths,
		InferImportPaths: len(importPaths) == 0,
	}

	fds, err := parser.ParseFiles(filenames...)
	if err != nil {
		return err
	}
	c.mds = make(map[string]*desc.MethodDescriptor)
	for _, fd := range fds {
		for _, sd := range fd.GetServices() {
			for _, md := range sd.GetMethods() {
				var s strings.Builder
				s.WriteString(sd.GetFullyQualifiedName())
				s.WriteRune('/')
				s.WriteString(md.GetName())
				c.mds[s.String()] = md
			}
		}
	}

	// TODO(rogchap): Would be good to list the available services/methods found as a list of fully qualified names
	return nil
}

type transportCreds struct {
	credentials.TransportCredentials
	errc chan<- error
}

func (t transportCreds) ClientHandshake(ctx context.Context, addr string, in net.Conn) (net.Conn, credentials.AuthInfo, error) {
	out, auth, err := t.TransportCredentials.ClientHandshake(ctx, addr, in)
	if err != nil {
		t.errc <- err
	}
	return out, auth, err
}

// Connect is a block dial to the gRPC server at the given address (host:port)
func (c *Client) Connect(ctxPtr *context.Context, addr string, params map[string]interface{}) error {
	state := lib.GetState(*ctxPtr)
	if state == nil {
		return errConnectInInitContext
	}

	isPlaintext := false

	for k, v := range params {
		switch k {
		case "plaintext":
			isPlaintext, _ = v.(bool)
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
			tlsCfg := state.TLSConfig

			var err error
			tlsCfg.RootCAs, err = x509.SystemCertPool()
			if err != nil {
				// (rogchap): If there is no System Pool, we could just create our own and still
				// continue; we only need a Cert Pool if we are adding our own RootCAs so returning
				// error for now.
				errc <- err
				return
			}
			//TODO(rogchap): Would be good to add support for custom RootCAs (self signed)

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

		var err error
		c.conn, err = grpc.Dial(addr, opts...)
		if err != nil {
			errc <- err
			return
		}
		close(errc)
	}()

	select {
	case err := <-errc:
		return err
	}
}

// InvokeRPC creates and calls a unary RPC by fully qualified method name
func (c *Client) InvokeRPC(ctxPtr *context.Context, method string, req goja.Value, params map[string]interface{}) (*Response, error) {
	ctx := *ctxPtr
	rt := common.GetRuntime(ctx)
	state := lib.GetState(ctx)
	if state == nil {
		return nil, errInvokeRPCInInitContext
	}

	if c.conn == nil {
		return nil, errors.New("No gRPC connection, you must call connect first")
	}

	method = strings.TrimPrefix(method, "/")
	md := c.mds[method]

	if md == nil {
		return nil, fmt.Errorf("Method %q not found in file descriptors", method)
	}

	timeout := 60 * time.Second
	tags := state.CloneTags()

	// TODO(rogchap): handle custom timeout default 60s
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
				tags[tk] = strVal
			}
		case "timeout":
			if t, ok := v.(float64); ok && t > 0.0 {
				timeout = time.Duration(t) * time.Millisecond
			}
		}
	}

	// TODO(rogchap): add standard gRPC tags
	// suggested tags:
	// * service (would the URL be enough as replacement for service/method?)
	// * method (this maybe confusing if we use the TagMethod as that is the HTTP Method (GET, POST etc)
	// * rpc_type: unary, client_streaming, server_streaming, bidirectional_streaming
	// * request_message: fully qualified name
	// * response_message: fully qualified name

	c.sampleTags = stats.IntoSampleTags(&tags)
	c.samplesOutput = state.Samples

	reqdm := dynamic.NewMessage(md.GetInputType())
	b, _ := req.ToObject(rt).MarshalJSON()
	reqdm.UnmarshalJSON(b)

	reqCtx, cancelFunc := context.WithTimeout(ctx, timeout)
	defer cancelFunc()
	s := grpcdynamic.NewStub(c.conn)
	resp, err := s.InvokeRpc(reqCtx, md, reqdm)

	var msgdm *dynamic.Message
	var response Response
	if err != nil {
		st := status.Convert(err)
		response.Status = st.Code()
		msgdm, _ = dynamic.AsDynamicMessage(st.Proto())
	}

	if resp != nil {
		msgdm = dynamic.NewMessage(md.GetOutputType())
		msgdm.Merge(resp)
	}

	// (rogchap) there is a lot of marshaling/unmarshaling here, but because this is a dynamic message
	// we need to marshal to get the JSON representation first. Using a map seems the best way to create
	// a goja.Value from the raw JSON bytes.
	raw, _ := msgdm.MarshalJSON()
	msg := make(map[string]interface{})
	json.Unmarshal(raw, &msg)
	response.Message = rt.ToValue(msg)

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

/*** stats.Handler interface methods ***/

func (*Client) TagRPC(ctx context.Context, _ *grpcstats.RPCTagInfo) context.Context {
	return ctx
}

func (c *Client) HandleRPC(ctx context.Context, stat grpcstats.RPCStats) {
	switch s := stat.(type) {
	case *grpcstats.End:
		stats.PushIfNotDone(ctx, c.samplesOutput, stats.ConnectedSamples{
			Samples: []stats.Sample{
				{
					Metric: metrics.GRPCReqDuration,
					Tags:   c.sampleTags,
					Value:  stats.D(s.EndTime.Sub(s.BeginTime)),
				},
				{
					Metric: metrics.GRPCReqs,
					Tags:   c.sampleTags,
					Value:  1,
				},
			},
		})

	}
}

func (*Client) TagConn(ctx context.Context, _ *grpcstats.ConnTagInfo) context.Context {
	return ctx
}

func (*Client) HandleConn(context.Context, grpcstats.ConnStats) {}
