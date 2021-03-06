// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package otlpgrpc // import "go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc"

	"go.opentelemetry.io/otel/exporters/otlp"
	colmetricpb "go.opentelemetry.io/otel/exporters/otlp/internal/opentelemetry-proto-gen/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/otel/exporters/otlp/internal/opentelemetry-proto-gen/collector/trace/v1"
	metricpb "go.opentelemetry.io/otel/exporters/otlp/internal/opentelemetry-proto-gen/metrics/v1"
	tracepb "go.opentelemetry.io/otel/exporters/otlp/internal/opentelemetry-proto-gen/trace/v1"
	"go.opentelemetry.io/otel/exporters/otlp/internal/transform"
	metricsdk "go.opentelemetry.io/otel/sdk/export/metric"
	tracesdk "go.opentelemetry.io/otel/sdk/export/trace"
)

type driver struct {
	connection *connection

	lock          sync.Mutex
	metricsClient colmetricpb.MetricsServiceClient
	tracesClient  coltracepb.TraceServiceClient
}

var (
	errNoClient     = errors.New("no client")
	errDisconnected = errors.New("exporter disconnected")
)

// NewDriver creates a new gRPC protocol driver.
func NewDriver(opts ...Option) otlp.ProtocolDriver {
	cfg := config{
		collectorEndpoint: fmt.Sprintf("%s:%d", otlp.DefaultCollectorHost, otlp.DefaultCollectorPort),
		serviceConfig:     DefaultServiceConfig,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	d := &driver{}
	d.connection = newConnection(cfg, d.handleNewConnection)
	return d
}

func (d *driver) handleNewConnection(cc *grpc.ClientConn) {
	d.lock.Lock()
	defer d.lock.Unlock()
	if cc != nil {
		d.metricsClient = colmetricpb.NewMetricsServiceClient(cc)
		d.tracesClient = coltracepb.NewTraceServiceClient(cc)
	} else {
		d.metricsClient = nil
		d.tracesClient = nil
	}
}

// Start implements otlp.ProtocolDriver. It establishes a connection
// to the collector.
func (d *driver) Start(ctx context.Context) error {
	d.connection.startConnection(ctx)
	return nil
}

// Stop implements otlp.ProtocolDriver. It shuts down the connection
// to the collector.
func (d *driver) Stop(ctx context.Context) error {
	return d.connection.shutdown(ctx)
}

// ExportMetrics implements otlp.ProtocolDriver. It transforms metrics
// to protobuf binary format and sends the result to the collector.
func (d *driver) ExportMetrics(ctx context.Context, cps metricsdk.CheckpointSet, selector metricsdk.ExportKindSelector) error {
	if !d.connection.connected() {
		return errDisconnected
	}
	ctx, cancel := d.connection.contextWithStop(ctx)
	defer cancel()

	rms, err := transform.CheckpointSet(ctx, selector, cps, 1)
	if err != nil {
		return err
	}
	if len(rms) == 0 {
		return nil
	}

	return d.uploadMetrics(ctx, rms)
}

func (d *driver) uploadMetrics(ctx context.Context, protoMetrics []*metricpb.ResourceMetrics) error {
	ctx = d.connection.contextWithMetadata(ctx)
	err := func() error {
		d.lock.Lock()
		defer d.lock.Unlock()
		if d.metricsClient == nil {
			return errNoClient
		}
		_, err := d.metricsClient.Export(ctx, &colmetricpb.ExportMetricsServiceRequest{
			ResourceMetrics: protoMetrics,
		})
		return err
	}()
	if err != nil {
		d.connection.setStateDisconnected(err)
	}
	return err
}

// ExportTraces implements otlp.ProtocolDriver. It transforms spans to
// protobuf binary format and sends the result to the collector.
func (d *driver) ExportTraces(ctx context.Context, ss []*tracesdk.SpanSnapshot) error {
	if !d.connection.connected() {
		return errDisconnected
	}
	ctx, cancel := d.connection.contextWithStop(ctx)
	defer cancel()

	protoSpans := transform.SpanData(ss)
	if len(protoSpans) == 0 {
		return nil
	}

	return d.uploadTraces(ctx, protoSpans)
}

func (d *driver) uploadTraces(ctx context.Context, protoSpans []*tracepb.ResourceSpans) error {
	ctx = d.connection.contextWithMetadata(ctx)
	err := func() error {
		d.lock.Lock()
		defer d.lock.Unlock()
		if d.tracesClient == nil {
			return errNoClient
		}
		_, err := d.tracesClient.Export(ctx, &coltracepb.ExportTraceServiceRequest{
			ResourceSpans: protoSpans,
		})
		return err
	}()
	if err != nil {
		d.connection.setStateDisconnected(err)
	}
	return err
}
