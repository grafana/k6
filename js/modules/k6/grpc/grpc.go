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
	"fmt"
	"net"
	"strings"

	"github.com/dop251/goja"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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

// Client reprecents a gRPC client that can be used to make RPC requests
type Client struct {
	fds []*desc.FileDescriptor

	sampleTags    *stats.SampleTags
	samplesOutput chan<- stats.SampleContainer

	conn *grpc.ClientConn
}

func (*GRPC) NewClient(ctx *context.Context /* TODO(rogchap): any options?*/) interface{} {
	rt := common.GetRuntime(*ctx)
	return common.Bind(rt, &Client{}, ctx)
}

// Load will parse the given proto files and make the file descriptors avaliable to request. This can only be called once;
// subsequent calls to Load will be a noop.
func (c *Client) Load(importPaths []string, filenames ...string) error {
	var err error

	parser := protoparse.Parser{
		ImportPaths:      importPaths,
		InferImportPaths: len(importPaths) == 0,
	}

	c.fds, err = parser.ParseFiles(filenames...)

	// TODO(rogchap): Would be good to list the available services/methods found as a list of fully qualified names
	return err
}

// Connect is a block dial to the gRPC server at the given address (host:port)
func (c *Client) Connect(ctxPtr *context.Context, addr string) error {
	// TODO(rogchap): Do check to make sure we are not in init
	state := lib.GetState(*ctxPtr)

	// pass as a parm option
	isPlaintext := true

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

		// TODO(rogchap); Need to add support for TLS and other credetials

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

	// TODO(rogchap): Create a true block dial so we can report on any errors
	// var err error
	// if c.conn, err = grpc.Dial(addr, grpc.WithInsecure(), grpc.WithStatsHandler(c)); err != nil {
	// 	return err
	// }
	// return nil
}

// InvokeRPC creates and calls a unary RPC by fully qualified method name
func (c *Client) InvokeRPC(ctx *context.Context, method string, req goja.Value) (*Response, error) {
	rt := common.GetRuntime(*ctx)
	state := lib.GetState(*ctx)
	// TODO(rogchap): check if state is nil

	if state == nil {
		return nil, fmt.Errorf("state is nil!")
	}

	tags := state.CloneTags()

	c.sampleTags = stats.IntoSampleTags(&tags)
	c.samplesOutput = state.Samples

	// TODO(rogchap): deal with base cases
	method = strings.TrimPrefix(method, "/")
	parts := strings.Split(method, "/")

	var md *desc.MethodDescriptor
	// TODO(rogchap) maybe we could create a map at load time so that this becomes O(1) rather than O(n) for each iteration?
	for _, fd := range c.fds {
		s := fd.FindService(parts[0])
		if s == nil {
			continue
		}
		md = s.FindMethodByName(parts[1])
		if md != nil {
			break
		}
	}

	if md == nil {
		return nil, fmt.Errorf("Method %q not found in file descriptors", method)
	}

	reqdm := dynamic.NewMessage(md.GetInputType())
	s := grpcdynamic.NewStub(c.conn)

	b, _ := req.ToObject(rt).MarshalJSON()
	reqdm.UnmarshalJSON(b)

	resp, err := s.InvokeRpc(*ctx, md, reqdm)

	var response Response
	if err != nil {
		response.Status = status.Code(err)
		//TODO(roghcap): deal with error message
	}

	respdm := dynamic.NewMessage(md.GetOutputType())
	if resp != nil {
		respdm.Merge(resp)
	}

	//TODO(rogchap): convert message to goja.Value and add to the response

	return &response, nil
}

// Close will close the client gRPC connection
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
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
