package kafka

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	extensionapi "go.k6.io/k6-extension-api"
	extensionapitest "go.k6.io/k6-extension-api/test"
)

func metricsVU(t *testing.T) *extensionapitest.VU {
	t.Helper()
	return extensionapitest.NewVU()
}

func collectSamples(samples []extensionapi.Sample) map[string]float64 {
	out := map[string]float64{}
	for _, sample := range samples {
		key := sample.Metric.Name()
		if topic, ok := sample.Tags.Values()["topic"]; ok {
			key += "{topic=" + topic + "}"
		}
		out[key] += sample.Value
	}
	return out
}

func TestRegisterMetrics(t *testing.T) {
	t.Parallel()
	vu := metricsVU(t)
	metrics := registerMetrics(vu)
	require.Equal(t, "kafka_writer_message_count", metrics.writerMessageCount.Name())
	require.Equal(t, "kafka_writer_message_bytes", metrics.writerMessageBytes.Name())
	require.Equal(t, "kafka_writer_batch_size", metrics.writerBatchSize.Name())
	require.Equal(t, "kafka_reader_lag", metrics.readerLag.Name())
}

func TestFlushNilVUAndNilCollector(t *testing.T) {
	t.Parallel()
	vu := metricsVU(t)
	vu.Phase = extensionapi.ExecutionPhaseInit
	metrics := registerMetrics(vu)
	collector := newMetricsCollector(metrics, roleWriter)
	collector.flushProduce(vu, map[string]int64{"t": 1}, map[string]int64{"t": 1}, 0)
	require.Empty(t, vu.Samples())

	var nilCollector *metricsCollector
	nilCollector.flushClose(vu)
}

func TestFlushProducePerTopic(t *testing.T) {
	t.Parallel()
	vu := metricsVU(t)
	collector := newMetricsCollector(registerMetrics(vu), roleWriter)
	collector.flushProduce(vu, map[string]int64{"a": 3, "b": 2}, map[string]int64{"a": 30, "b": 20}, 0)

	got := collectSamples(vu.Samples())
	require.Equal(t, 3.0, got["kafka_writer_message_count{topic=a}"])
	require.Equal(t, 2.0, got["kafka_writer_message_count{topic=b}"])
	require.Equal(t, 30.0, got["kafka_writer_message_bytes{topic=a}"])
	require.Equal(t, 20.0, got["kafka_writer_message_bytes{topic=b}"])
}

func TestFlushProducesExpectedErrorsAndHookDeltas(t *testing.T) {
	t.Parallel()
	vu := metricsVU(t)
	collector := newMetricsCollector(registerMetrics(vu), roleWriter)
	collector.flushProduce(vu, nil, nil, 2)
	collector.OnBrokerConnect(kgo.BrokerMetadata{}, 5*time.Millisecond, nil, nil)
	collector.OnBrokerConnect(kgo.BrokerMetadata{}, 5*time.Millisecond, nil, nil)
	collector.flushProduce(vu, nil, nil, 0)

	got := collectSamples(vu.Samples())
	require.Equal(t, 2.0, got["kafka_writer_error_count"])
	require.Equal(t, 2.0, got["kafka_writer_dial_count"])
	require.NotContains(t, got, "kafka_writer_message_count{topic=a}")
}

func TestFlushCloseDrainsFetchHooks(t *testing.T) {
	t.Parallel()
	vu := metricsVU(t)
	collector := newMetricsCollector(registerMetrics(vu), roleReader)
	collector.OnFetchBatchRead(kgo.BrokerMetadata{}, "t", 0, kgo.FetchBatchMetrics{NumRecords: 5, UncompressedBytes: 50})
	collector.flushClose(vu)

	got := collectSamples(vu.Samples())
	require.Equal(t, 1.0, got["kafka_reader_fetches_count{topic=t}"])
	require.Equal(t, 5.0, got["kafka_reader_fetch_size{topic=t}"])
	require.Equal(t, 50.0, got["kafka_reader_fetch_bytes{topic=t}"])
}
