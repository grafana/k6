package grpc_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"go.k6.io/k6/lib/testutils/grpcservice"
	"go.k6.io/k6/lib/testutils/httpmultibin/grpc_wrappers_testing"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestStream_InvalidHeader(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	initString := codeBlock{
		code: `
		var client = new grpc.Client();
		client.load([], "../../../../lib/testutils/httpmultibin/grpc_testing/test.proto");`,
	}

	val, err := ts.Run(initString.code)
	assertResponse(t, initString, err, val, ts)

	ts.ToVUContext()

	_, err = ts.Run(`
	client.connect("GRPCBIN_ADDR");
	new grpc.Stream(client, "foo/bar")`)

	assert.Error(t, err)
	assert.ErrorContains(t, err, `method "/foo/bar" not found in file descriptors`)
}

func TestStream_RequestHeaders(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	var registeredMetadata metadata.MD
	stub := &featureExplorerStub{}
	stub.listFeatures = func(_ *grpcservice.Rectangle, stream grpcservice.FeatureExplorer_ListFeaturesServer) error {
		// collect metadata from the stream context
		md, ok := metadata.FromIncomingContext(stream.Context())
		if ok {
			registeredMetadata = md
		}

		return nil
	}

	grpcservice.RegisterFeatureExplorerServer(ts.httpBin.ServerGRPC, stub)

	initString := codeBlock{
		code: `
		var client = new grpc.Client();
		client.load([], "../../../../lib/testutils/grpcservice/route_guide.proto");`,
	}
	vuString := codeBlock{
		code: `
		client.connect("GRPCBIN_ADDR");
		let stream = new grpc.Stream(client, "main.FeatureExplorer/ListFeatures", { metadata: { "X-Load-Tester": "k6" } })
		stream.write({
			lo: {
			  latitude: 400000000,
			  longitude: -750000000,
			},
			hi: {
			  latitude: 420000000,
			  longitude: -730000000,
			},
		});
		`,
	}

	val, err := ts.Run(initString.code)
	assertResponse(t, initString, err, val, ts)

	ts.ToVUContext()

	val, err = ts.RunOnEventLoop(vuString.code)

	assertResponse(t, vuString, err, val, ts)

	// Check that the metadata was registered
	assert.Len(t, registeredMetadata["x-load-tester"], 1)
	assert.Equal(t, registeredMetadata["x-load-tester"][0], "k6")
}

func TestStream_ErrorHandling(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	stub := &featureExplorerStub{}

	savedFeatures := []*grpcservice.Feature{
		{
			Name: "foo",
			Location: &grpcservice.Point{
				Latitude:  1,
				Longitude: 2,
			},
		},
		{
			Name: "bar",
			Location: &grpcservice.Point{
				Latitude:  3,
				Longitude: 4,
			},
		},
	}

	stub.listFeatures = func(_ *grpcservice.Rectangle, stream grpcservice.FeatureExplorer_ListFeaturesServer) error {
		for _, feature := range savedFeatures {
			if err := stream.Send(feature); err != nil {
				return err
			}
		}

		return status.Error(codes.Internal, "lorem ipsum")
	}

	grpcservice.RegisterFeatureExplorerServer(ts.httpBin.ServerGRPC, stub)

	initString := codeBlock{
		code: `
		var client = new grpc.Client();
		client.load([], "../../../../lib/testutils/grpcservice/route_guide.proto");`,
	}
	vuString := codeBlock{
		code: `
		client.connect("GRPCBIN_ADDR");
		let stream = new grpc.Stream(client, "main.FeatureExplorer/ListFeatures")
		stream.write({
			lo: {
			  latitude: 1,
			  longitude: 2,
			},
			hi: {
			  latitude: 1,
			  longitude: 2,
			},
		});
		stream.on('data', function (data) {
			call('Feature:' + data.name);
		})
		stream.on('error', function (e) {
			call('Code: ' + e.code + ' Message: ' + e.message);
		});
		`,
	}

	val, err := ts.Run(initString.code)
	assertResponse(t, initString, err, val, ts)

	ts.ToVUContext()

	val, err = ts.RunOnEventLoop(vuString.code)

	assertResponse(t, vuString, err, val, ts)

	assert.Equal(t,
		[]string{
			"Feature:foo",
			"Feature:bar",
			"Code: 13 Message: lorem ipsum",
		},
		ts.callRecorder.Recorded(),
	)
}

// this test case is checking that everything that server sends
// after the client finished (client.end called) is delivered to the client
// and the end event is called
func TestStream_ReceiveAllServerResponsesAfterEnd(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	stub := &featureExplorerStub{}

	savedFeatures := []*grpcservice.Feature{
		{
			Name: "foo",
			Location: &grpcservice.Point{
				Latitude:  1,
				Longitude: 2,
			},
		},
		{
			Name: "bar",
			Location: &grpcservice.Point{
				Latitude:  3,
				Longitude: 4,
			},
		},
	}

	stub.listFeatures = func(_ *grpcservice.Rectangle, stream grpcservice.FeatureExplorer_ListFeaturesServer) error {
		for _, feature := range savedFeatures {
			// adding a delay to make server response "slower"
			time.Sleep(200 * time.Millisecond)

			if err := stream.Send(feature); err != nil {
				return err
			}
		}

		return nil
	}

	grpcservice.RegisterFeatureExplorerServer(ts.httpBin.ServerGRPC, stub)

	initString := codeBlock{
		code: `
		var client = new grpc.Client();
		client.load([], "../../../../lib/testutils/grpcservice/route_guide.proto");`,
	}
	vuString := codeBlock{
		code: `
		client.connect("GRPCBIN_ADDR");
		let stream = new grpc.Stream(client, "main.FeatureExplorer/ListFeatures")
		stream.on('data', function (data) {
			call('Feature:' + data.name);
		});
		stream.on('end', function () {
			call('End called');
		});

		stream.write({
			lo: {
			  latitude: 1,
			  longitude: 2,
			},
			hi: {
			  latitude: 1,
			  longitude: 2,
			},
		});
		stream.end();
		`,
	}

	val, err := ts.Run(initString.code)
	assertResponse(t, initString, err, val, ts)

	ts.ToVUContext()

	val, err = ts.RunOnEventLoop(vuString.code)

	assertResponse(t, vuString, err, val, ts)

	assert.Equal(t, ts.callRecorder.Recorded(), []string{
		"Feature:foo",
		"Feature:bar",
		"End called",
	},
	)
}

// featureExplorerStub is a stub for FeatureExplorerServer
// it has ability to override methods
type featureExplorerStub struct {
	grpcservice.UnimplementedFeatureExplorerServer

	getFeature   func(ctx context.Context, point *grpcservice.Point) (*grpcservice.Feature, error)
	listFeatures func(rect *grpcservice.Rectangle, stream grpcservice.FeatureExplorer_ListFeaturesServer) error
}

func (s *featureExplorerStub) GetFeature(ctx context.Context, point *grpcservice.Point) (*grpcservice.Feature, error) {
	if s.getFeature != nil {
		return s.getFeature(ctx, point)
	}

	return nil, status.Errorf(codes.Unimplemented, "method GetFeature not implemented")
}

func (s *featureExplorerStub) ListFeatures(rect *grpcservice.Rectangle, stream grpcservice.FeatureExplorer_ListFeaturesServer) error {
	if s.listFeatures != nil {
		return s.listFeatures(rect, stream)
	}

	return status.Errorf(codes.Unimplemented, "method ListFeatures not implemented")
}

func TestStream_Wrappers(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	stub := grpc_wrappers_testing.Register(ts.httpBin.ServerGRPC)
	stub.TestStreamImplementation = func(stream grpc_wrappers_testing.Service_TestStreamServer) error {
		result := ""

		for {
			msg, err := stream.Recv()
			if errors.Is(err, io.EOF) {
				return stream.SendAndClose(&wrappers.StringValue{
					Value: strings.TrimRight(result, " "),
				})
			}

			if err != nil {
				return err
			}

			result += msg.Value + " "
		}
	}

	replace := func(code string) (sobek.Value, error) {
		return ts.VU.Runtime().RunString(ts.httpBin.Replacer.Replace(code))
	}

	initString := codeBlock{
		code: `
		var client = new grpc.Client();
		client.load([], "../../../../lib/testutils/httpmultibin/grpc_wrappers_testing/test.proto");`,
	}
	vuString := codeBlock{
		code: `
		client.connect("GRPCBIN_ADDR");
		let stream = new grpc.Stream(client, "grpc.wrappers.testing.Service/TestStream");
		stream.on('data', function (data) {
			call('Result: ' + data);
		})

		stream.write('Hey');
		stream.write('John');
		stream.end();

		stream.on('error', function (e) {
			call('Code: ' + e.code + ' Message: ' + e.message);
		});
		`,
	}

	val, err := replace(initString.code)
	assertResponse(t, initString, err, val, ts)

	ts.ToVUContext()

	val, err = replace(vuString.code)

	ts.EventLoop.WaitOnRegistered()

	assertResponse(t, vuString, err, val, ts)

	assert.Equal(t, ts.callRecorder.Recorded(), []string{
		"Result: Hey John",
	},
	)
}

func TestStream_UndefinedHandler(t *testing.T) {
	t.Parallel()

	ts := newTestState(t)

	stub := grpc_wrappers_testing.Register(ts.httpBin.ServerGRPC)
	stub.TestStreamImplementation = func(stream grpc_wrappers_testing.Service_TestStreamServer) error {
		return stream.SendAndClose(&wrappers.StringValue{
			Value: "test",
		})
	}

	replace := func(code string) (sobek.Value, error) {
		return ts.VU.Runtime().RunString(ts.httpBin.Replacer.Replace(code))
	}

	initString := codeBlock{
		code: `
		var client = new grpc.Client();
		client.load([], "../../../../lib/testutils/httpmultibin/grpc_wrappers_testing/test.proto");`,
	}
	vuString := codeBlock{
		code: `
		client.connect("GRPCBIN_ADDR");
		let stream = new grpc.Stream(client, "grpc.wrappers.testing.Service/TestStream");
		stream.on('data', undefined);

		stream.end();
		`,
	}

	val, err := replace(initString.code)
	assertResponse(t, initString, err, val, ts)

	ts.ToVUContext()

	_, err = replace(vuString.code)
	ts.EventLoop.WaitOnRegistered()

	require.ErrorContains(t, err, "handler for \"data\" event isn't a callable function")
}
