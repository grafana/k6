package opentelemetry

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/internal/lib/testutils"
	"go.k6.io/k6/metrics"
	"go.k6.io/k6/output"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type MetricsServer interface {
	Start() error
	Stop()
	Endpoint() string
	LastMetrics() []byte
}

type baseServer struct {
	mu          sync.Mutex
	lastMetrics []byte
}

func (s *baseServer) setLastMetrics(metrics []byte) {
	s.mu.Lock()
	s.lastMetrics = metrics
	s.mu.Unlock()
}

func (s *baseServer) LastMetrics() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastMetrics
}

type httpMetricsServer struct {
	baseServer
	server *httptest.Server
}

func newHTTPServer() *httpMetricsServer {
	s := &httpMetricsServer{}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/metrics" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		s.setLastMetrics(body)
		w.WriteHeader(http.StatusOK)
	}))
	return s
}

func (s *httpMetricsServer) Start() error     { return nil }
func (s *httpMetricsServer) Stop()            { s.server.Close() }
func (s *httpMetricsServer) Endpoint() string { return s.server.Listener.Addr().String() }

type grpcMetricsServer struct {
	baseServer
	server   *grpc.Server
	listener net.Listener
}

func newGRPCServer() (*grpcMetricsServer, error) {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	s := &grpcMetricsServer{
		server:   grpc.NewServer(),
		listener: listener,
	}

	collectormetrics.RegisterMetricsServiceServer(s.server, &grpcMetricsHandler{
		UnimplementedMetricsServiceServer: collectormetrics.UnimplementedMetricsServiceServer{},
		baseServer:                        &s.baseServer,
	})
	return s, nil
}

func (s *grpcMetricsServer) Start() error {
	errChan := make(chan error, 1)
	go func() {
		if err := s.server.Serve(s.listener); err != nil {
			errChan <- fmt.Errorf("server failed to serve: %w", err)
		}
		close(errChan)
	}()

	select {
	case err := <-errChan:
		return err
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

func (s *grpcMetricsServer) Stop() {
	s.server.Stop()
	if err := s.listener.Close(); err != nil {
		_ = err
	}
}

func (s *grpcMetricsServer) Endpoint() string { return s.listener.Addr().String() }

type grpcMetricsHandler struct {
	collectormetrics.UnimplementedMetricsServiceServer
	baseServer *baseServer
}

func (h *grpcMetricsHandler) Export(_ context.Context, req *collectormetrics.ExportMetricsServiceRequest) (*collectormetrics.ExportMetricsServiceResponse, error) {
	data, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	h.baseServer.setLastMetrics(data)
	return &collectormetrics.ExportMetricsServiceResponse{}, nil
}

func createServer(t *testing.T, protocol string) MetricsServer {
	switch protocol {
	case "http":
		return newHTTPServer()
	case "grpc":
		server, err := newGRPCServer()
		require.NoError(t, err)
		require.NoError(t, server.Start())
		return server
	default:
		t.Fatalf("unsupported protocol: %s", protocol)
		return nil
	}
}

func TestOutput(t *testing.T) {
	t.Parallel()

	testProtocols := []string{"http", "grpc"}
	testCases := []struct {
		name   string
		metric struct {
			typ   metrics.MetricType
			value float64
		}
		validate func(*testing.T, *collectormetrics.ExportMetricsServiceRequest)
	}{
		{
			name: "gauge_metric",
			metric: struct {
				typ   metrics.MetricType
				value float64
			}{metrics.Gauge, 42.0},
			validate: validateGaugeMetric,
		},
		{
			name: "counter_metric",
			metric: struct {
				typ   metrics.MetricType
				value float64
			}{metrics.Counter, 10.0},
			validate: validateCounterMetric,
		},
		{
			name: "trend_metric",
			metric: struct {
				typ   metrics.MetricType
				value float64
			}{metrics.Trend, 25.0},
			validate: validateTrendMetric,
		},
	}

	for _, proto := range testProtocols {
		proto := proto
		t.Run(fmt.Sprintf("%s collector", proto), func(t *testing.T) {
			t.Parallel()
			for _, tc := range testCases {
				tc := tc
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					server := createServer(t, proto)
					defer server.Stop()

					config := createTestConfig(proto, server.Endpoint())
					output := setupOutput(t, config)
					defer func() {
						if err := output.Stop(); err != nil {
							t.Errorf("failed to stop output: %v", err)
						}
					}()

					sample := createTestSample(t, tc.metric.typ, tc.metric.value)
					output.AddMetricSamples([]metrics.SampleContainer{metrics.Samples([]metrics.Sample{sample})})

					time.Sleep(300 * time.Millisecond)
					validateMetrics(t, server.LastMetrics(), tc.validate)
				})
			}
		})
	}
}

func createTestConfig(protocol, endpoint string) map[string]string {
	config := map[string]string{
		"K6_OTEL_SERVICE_NAME":    "test_service",
		"K6_OTEL_FLUSH_INTERVAL":  "100ms",
		"K6_OTEL_EXPORT_INTERVAL": "100ms",
		"K6_OTEL_EXPORTER_TYPE":   protocol,
		"K6_OTEL_METRIC_PREFIX":   "test.",
	}

	if protocol == "http" {
		config["K6_OTEL_HTTP_EXPORTER_INSECURE"] = "true"
		config["K6_OTEL_HTTP_EXPORTER_ENDPOINT"] = endpoint
		config["K6_OTEL_HTTP_EXPORTER_URL_PATH"] = "/v1/metrics"
	} else {
		config["K6_OTEL_GRPC_EXPORTER_INSECURE"] = "true"
		config["K6_OTEL_GRPC_EXPORTER_ENDPOINT"] = endpoint
	}

	return config
}

func setupOutput(t *testing.T, config map[string]string) *Output {
	o, err := New(output.Params{
		Logger:      testutils.NewLogger(t),
		Environment: config,
	})
	require.NoError(t, err)
	require.NoError(t, o.Start())
	return o
}

func createTestSample(t *testing.T, metricType metrics.MetricType, value float64) metrics.Sample {
	registry := metrics.NewRegistry()
	metricName := metricType.String() + "_metric"
	metric, err := registry.NewMetric(metricName, metricType)
	require.NoError(t, err)

	return metrics.Sample{
		TimeSeries: metrics.TimeSeries{
			Metric: metric,
			Tags: registry.RootTagSet().WithTagsFromMap(map[string]string{
				"tag1": "value1",
			}),
		},
		Value: value,
	}
}

func validateMetrics(t *testing.T, data []byte, validate func(*testing.T, *collectormetrics.ExportMetricsServiceRequest)) {
	require.NotNil(t, data, "No metrics were received by collector")

	var metricsRequest collectormetrics.ExportMetricsServiceRequest
	err := proto.Unmarshal(data, &metricsRequest)
	require.NoError(t, err)

	validate(t, &metricsRequest)
}

func validateGaugeMetric(t *testing.T, mr *collectormetrics.ExportMetricsServiceRequest) {
	metric := findMetric(mr, "test.gauge_metric")
	require.NotNil(t, metric, "gauge metric not found")
	gauge := metric.GetGauge()
	require.NotNil(t, gauge)
	require.Len(t, gauge.DataPoints, 1)
	assert.Equal(t, 42.0, gauge.DataPoints[0].GetAsDouble())
	assertHasAttribute(t, gauge.DataPoints[0].Attributes, "tag1", "value1")
}

func validateCounterMetric(t *testing.T, mr *collectormetrics.ExportMetricsServiceRequest) {
	metric := findMetric(mr, "test.counter_metric")
	require.NotNil(t, metric, "counter metric not found")
	sum := metric.GetSum()
	require.NotNil(t, sum)
	require.Len(t, sum.DataPoints, 1)
	assert.Equal(t, 10.0, sum.DataPoints[0].GetAsDouble())
	assertHasAttribute(t, sum.DataPoints[0].Attributes, "tag1", "value1")
}

func validateTrendMetric(t *testing.T, mr *collectormetrics.ExportMetricsServiceRequest) {
	metric := findMetric(mr, "test.trend_metric")
	require.NotNil(t, metric, "trend metric not found")
	histogram := metric.GetHistogram()
	require.NotNil(t, histogram)
	require.Len(t, histogram.DataPoints, 1)
	assert.Equal(t, uint64(1), histogram.DataPoints[0].GetCount())
	assert.Equal(t, 25.0, histogram.DataPoints[0].GetSum())
	assertHasAttribute(t, histogram.DataPoints[0].Attributes, "tag1", "value1")
}

func findMetric(mr *collectormetrics.ExportMetricsServiceRequest, name string) *metricpb.Metric {
	for _, rm := range mr.GetResourceMetrics() {
		for _, sm := range rm.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == name {
					return m
				}
			}
		}
	}
	return nil
}

func assertHasAttribute(t *testing.T, attrs []*commonpb.KeyValue, key, value string) {
	for _, attr := range attrs {
		if attr.Key == key {
			assert.Equal(t, value, attr.GetValue().GetStringValue())
			return
		}
	}
	t.Errorf("Attribute %s not found", key)
}
