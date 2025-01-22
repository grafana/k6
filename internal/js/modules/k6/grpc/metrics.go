package grpc

import "go.k6.io/k6/metrics"

// instanceMetrics contains the metrics for the grpc extension.
type instanceMetrics struct {
	Streams                 *metrics.Metric
	StreamsMessagesSent     *metrics.Metric
	StreamsMessagesReceived *metrics.Metric
}

// registerMetrics registers and returns the metrics in the provided registry
func registerMetrics(registry *metrics.Registry) (*instanceMetrics, error) {
	var err error
	m := &instanceMetrics{}

	if m.Streams, err = registry.NewMetric("grpc_streams", metrics.Counter); err != nil {
		return nil, err
	}

	if m.StreamsMessagesSent, err = registry.NewMetric("grpc_streams_msgs_sent", metrics.Counter); err != nil {
		return nil, err
	}

	if m.StreamsMessagesReceived, err = registry.NewMetric("grpc_streams_msgs_received", metrics.Counter); err != nil {
		return nil, err
	}

	return m, nil
}
