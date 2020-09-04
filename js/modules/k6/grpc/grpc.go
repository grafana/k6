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
	"strings"
	"sync"

	"github.com/dop251/goja"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"google.golang.org/grpc"
	grpcstats "google.golang.org/grpc/stats"

	"github.com/loadimpact/k6/js/common"
	"github.com/loadimpact/k6/lib"
	"github.com/loadimpact/k6/lib/metrics"
	"github.com/loadimpact/k6/stats"
)

// GRPC represents the gRPC protocol module for k6
type GRPC struct{}

// New creates a new gRPC module
func New() *GRPC {
	return &GRPC{}
}

// Client reprecents a gRPC client that can be used to make RPC requests
type Client struct {
	ctx context.Context

	sampleTags    *stats.SampleTags
	samplesOutput chan<- stats.SampleContainer

	conn *grpc.ClientConn
}

func (*GRPC) NewClient(ctx context.Context /* TODO(rogchap): any options?*/) *Client {
	return &Client{ctx: ctx}
}

// TODO(rogchap): avoid this global; doing this because we can't create an object that has state outside of the vu code block
var fds []*desc.FileDescriptor
var once sync.Once

// Load will parse the given proto files and make the file descriptors avaliable to request. This can only be called once;
// subsequent calls to Load will be a noop.
func (*GRPC) Load(importPaths []string, filenames ...string) error {
	var err error

	once.Do(func() {
		parser := protoparse.Parser{
			ImportPaths:      importPaths,
			InferImportPaths: len(importPaths) == 0,
		}

		fds, err = parser.ParseFiles(filenames...)
	})

	// TODO(rogchap): Would be good to list the available services/methods found as a list of fully qualified names
	return err
}

func (c *Client) Connect(addr string) error {
	// TODO(rogchap): Create a true block dial so we can report on any errors
	var err error
	if c.conn, err = grpc.Dial(addr, grpc.WithInsecure(), grpc.WithStatsHandler(c)); err != nil {
		return err
	}
	return nil
}

func (c *Client) InvokeRPC(method string, req goja.Value) (*dynamic.Message, error) {
	rt := common.GetRuntime(c.ctx)
	state := lib.GetState(c.ctx)
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
	for _, fd := range fds {
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

	// TODO(rogchap): Should I use the same context for the request or create a new one?
	resp, err := s.InvokeRpc(c.ctx, md, reqdm)
	if err != nil {
		//TODO(roghcap): deal with error message and status code
		return nil, err
	}

	respdm := dynamic.NewMessage(md.GetOutputType())
	respdm.Merge(resp)

	return respdm, err
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
	dataSent, dataReceived := 0, 0

	switch s := stat.(type) {
	case *grpcstats.OutPayload:
		dataSent = s.WireLength
	case *grpcstats.InHeader:
		dataReceived = s.WireLength
	case *grpcstats.InPayload:
		dataReceived = s.WireLength
	case *grpcstats.InTrailer:
		dataReceived = s.WireLength
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

	if dataSent > 0 {
		stats.PushIfNotDone(ctx, c.samplesOutput, stats.Sample{
			Metric: metrics.DataSent,
			Tags:   c.sampleTags,
			Value:  float64(dataSent),
		})
	}

	if dataReceived > 0 {
		stats.PushIfNotDone(ctx, c.samplesOutput, stats.Sample{
			Metric: metrics.DataReceived,
			Tags:   c.sampleTags,
			Value:  float64(dataReceived),
		})
	}
}

func (*Client) TagConn(ctx context.Context, _ *grpcstats.ConnTagInfo) context.Context {
	return ctx
}

func (*Client) HandleConn(context.Context, grpcstats.ConnStats) {}
