package kafka

import (
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
)

type kafkaMetrics struct {
	ReaderDials      *metrics.Metric
	ReaderFetches    *metrics.Metric
	ReaderMessages   *metrics.Metric
	ReaderBytes      *metrics.Metric
	ReaderRebalances *metrics.Metric
	ReaderTimeouts   *metrics.Metric
	ReaderErrors     *metrics.Metric

	ReaderDialTime   *metrics.Metric
	ReaderReadTime   *metrics.Metric
	ReaderWaitTime   *metrics.Metric
	ReaderFetchSize  *metrics.Metric
	ReaderFetchBytes *metrics.Metric

	ReaderOffset        *metrics.Metric
	ReaderLag           *metrics.Metric
	ReaderMinBytes      *metrics.Metric
	ReaderMaxBytes      *metrics.Metric
	ReaderMaxWait       *metrics.Metric
	ReaderQueueLength   *metrics.Metric
	ReaderQueueCapacity *metrics.Metric

	WriterWrites   *metrics.Metric
	WriterMessages *metrics.Metric
	WriterBytes    *metrics.Metric
	WriterErrors   *metrics.Metric

	WriterBatchTime      *metrics.Metric
	WriterBatchQueueTime *metrics.Metric
	WriterWriteTime      *metrics.Metric
	WriterWaitTime       *metrics.Metric
	WriterRetries        *metrics.Metric
	WriterBatchSize      *metrics.Metric
	WriterBatchBytes     *metrics.Metric

	WriterMaxAttempts  *metrics.Metric
	WriterMaxBatchSize *metrics.Metric
	WriterBatchTimeout *metrics.Metric
	WriterReadTimeout  *metrics.Metric
	WriterWriteTimeout *metrics.Metric
	WriterRequiredAcks *metrics.Metric
	WriterAsync        *metrics.Metric
}

type kafkaMetricDefinition struct {
	name         string
	metricType   metrics.MetricType
	valueType    metrics.ValueType
	hasValueType bool
	assign       func(*kafkaMetrics, *metrics.Metric)
}

func metricDef(
	name string,
	metricType metrics.MetricType,
	assign func(*kafkaMetrics, *metrics.Metric),
) kafkaMetricDefinition {
	return kafkaMetricDefinition{
		name:       name,
		metricType: metricType,
		assign:     assign,
	}
}

func typedMetricDef(
	name string,
	metricType metrics.MetricType,
	valueType metrics.ValueType,
	assign func(*kafkaMetrics, *metrics.Metric),
) kafkaMetricDefinition {
	return kafkaMetricDefinition{
		name:         name,
		metricType:   metricType,
		valueType:    valueType,
		hasValueType: true,
		assign:       assign,
	}
}

var kafkaMetricDefinitions = []kafkaMetricDefinition{
	metricDef("kafka_reader_dial_count", metrics.Counter, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.ReaderDials = metric
	}),
	metricDef("kafka_reader_fetches_count", metrics.Counter, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.ReaderFetches = metric
	}),
	metricDef("kafka_reader_message_count", metrics.Counter, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.ReaderMessages = metric
	}),
	typedMetricDef("kafka_reader_message_bytes", metrics.Counter, metrics.Data, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.ReaderBytes = metric
	}),
	metricDef("kafka_reader_rebalance_count", metrics.Counter, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.ReaderRebalances = metric
	}),
	metricDef("kafka_reader_timeouts_count", metrics.Counter, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.ReaderTimeouts = metric
	}),
	metricDef("kafka_reader_error_count", metrics.Counter, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.ReaderErrors = metric
	}),
	typedMetricDef("kafka_reader_dial_seconds", metrics.Trend, metrics.Time, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.ReaderDialTime = metric
	}),
	typedMetricDef("kafka_reader_read_seconds", metrics.Trend, metrics.Time, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.ReaderReadTime = metric
	}),
	typedMetricDef("kafka_reader_wait_seconds", metrics.Trend, metrics.Time, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.ReaderWaitTime = metric
	}),
	metricDef("kafka_reader_fetch_size", metrics.Counter, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.ReaderFetchSize = metric
	}),
	typedMetricDef("kafka_reader_fetch_bytes", metrics.Counter, metrics.Data, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.ReaderFetchBytes = metric
	}),
	metricDef("kafka_reader_offset", metrics.Gauge, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.ReaderOffset = metric
	}),
	metricDef("kafka_reader_lag", metrics.Gauge, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.ReaderLag = metric
	}),
	metricDef("kafka_reader_fetch_bytes_min", metrics.Gauge, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.ReaderMinBytes = metric
	}),
	metricDef("kafka_reader_fetch_bytes_max", metrics.Gauge, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.ReaderMaxBytes = metric
	}),
	typedMetricDef("kafka_reader_fetch_wait_max", metrics.Gauge, metrics.Time, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.ReaderMaxWait = metric
	}),
	metricDef("kafka_reader_queue_length", metrics.Gauge, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.ReaderQueueLength = metric
	}),
	metricDef("kafka_reader_queue_capacity", metrics.Gauge, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.ReaderQueueCapacity = metric
	}),
	metricDef("kafka_writer_write_count", metrics.Counter, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.WriterWrites = metric
	}),
	metricDef("kafka_writer_message_count", metrics.Counter, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.WriterMessages = metric
	}),
	typedMetricDef("kafka_writer_message_bytes", metrics.Counter, metrics.Data, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.WriterBytes = metric
	}),
	metricDef("kafka_writer_error_count", metrics.Counter, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.WriterErrors = metric
	}),
	typedMetricDef("kafka_writer_batch_seconds", metrics.Trend, metrics.Time, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.WriterBatchTime = metric
	}),
	typedMetricDef("kafka_writer_batch_queue_seconds", metrics.Trend, metrics.Time, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.WriterBatchQueueTime = metric
	}),
	typedMetricDef("kafka_writer_write_seconds", metrics.Trend, metrics.Time, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.WriterWriteTime = metric
	}),
	typedMetricDef("kafka_writer_wait_seconds", metrics.Trend, metrics.Time, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.WriterWaitTime = metric
	}),
	metricDef("kafka_writer_retries_count", metrics.Counter, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.WriterRetries = metric
	}),
	metricDef("kafka_writer_batch_size", metrics.Counter, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.WriterBatchSize = metric
	}),
	typedMetricDef("kafka_writer_batch_bytes", metrics.Counter, metrics.Data, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.WriterBatchBytes = metric
	}),
	metricDef("kafka_writer_attempts_max", metrics.Gauge, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.WriterMaxAttempts = metric
	}),
	metricDef("kafka_writer_batch_max", metrics.Gauge, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.WriterMaxBatchSize = metric
	}),
	typedMetricDef("kafka_writer_batch_timeout", metrics.Gauge, metrics.Time, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.WriterBatchTimeout = metric
	}),
	typedMetricDef("kafka_writer_read_timeout", metrics.Gauge, metrics.Time, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.WriterReadTimeout = metric
	}),
	typedMetricDef("kafka_writer_write_timeout", metrics.Gauge, metrics.Time, func(
		km *kafkaMetrics,
		metric *metrics.Metric,
	) {
		km.WriterWriteTimeout = metric
	}),
	metricDef("kafka_writer_acks_required", metrics.Gauge, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.WriterRequiredAcks = metric
	}),
	metricDef("kafka_writer_async", metrics.Rate, func(km *kafkaMetrics, metric *metrics.Metric) {
		km.WriterAsync = metric
	}),
}

func registeredKafkaMetricNames() []string {
	names := make([]string, 0, len(kafkaMetricDefinitions))
	for _, definition := range kafkaMetricDefinitions {
		names = append(names, definition.name)
	}

	return names
}

func registerKafkaMetric(
	registry *metrics.Registry,
	definition kafkaMetricDefinition,
) (*metrics.Metric, error) {
	if definition.hasValueType {
		return registry.NewMetric(definition.name, definition.metricType, definition.valueType)
	}

	return registry.NewMetric(definition.name, definition.metricType)
}

// registerMetrics registers the metrics for the kafka module in the metrics registry.
func registerMetrics(vu modules.VU) (kafkaMetrics, error) {
	registry := vu.InitEnv().Registry
	registeredMetrics := kafkaMetrics{}

	for _, definition := range kafkaMetricDefinitions {
		metric, err := registerKafkaMetric(registry, definition)
		if err != nil {
			return registeredMetrics, err
		}

		definition.assign(&registeredMetrics, metric)
	}

	return registeredMetrics, nil
}
