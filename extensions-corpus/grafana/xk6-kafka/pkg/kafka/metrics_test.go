package kafka

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
	"go.k6.io/k6/v2/js/modulestest"
	"go.k6.io/k6/v2/lib"
	"go.k6.io/k6/v2/metrics"
)

// metricsVU returns a runtime moved into VU context with a samples channel and a
// registry whose root tag set backs the VU tags.
func metricsVU(t *testing.T) (*modulestest.Runtime, *metrics.Registry, chan metrics.SampleContainer) {
	t.Helper()
	rt := modulestest.NewRuntime(t)
	registry := metrics.NewRegistry()
	samples := make(chan metrics.SampleContainer, 1000)
	rt.MoveToVUContext(&lib.State{
		Samples: samples,
		Tags:    lib.NewVUStateTags(registry.RootTagSet()),
	})
	return rt, registry, samples
}

// collectSamples drains the channel, summing values keyed by metric name plus an
// optional topic tag (e.g. "kafka_writer_message_count{topic=a}").
func collectSamples(samples chan metrics.SampleContainer) map[string]float64 {
	out := map[string]float64{}
	for {
		select {
		case sc := <-samples:
			for _, s := range sc.GetSamples() {
				key := s.Metric.Name
				if tp, ok := s.Tags.Get("topic"); ok {
					key += "{topic=" + tp + "}"
				}
				out[key] += s.Value
			}
		default:
			return out
		}
	}
}

func TestRegisterMetricsTypes(t *testing.T) {
	t.Parallel()
	r := metrics.NewRegistry()
	km := registerMetrics(r)
	require.Equal(t, metrics.Counter, km.writerMessageCount.Type)
	require.Equal(t, metrics.Data, km.writerMessageBytes.Contains)
	require.Equal(t, metrics.Trend, km.writerBatchSize.Type)
	require.Equal(t, metrics.Trend, km.readerLag.Type)
	require.Equal(t, metrics.Counter, km.readerFetchesCount.Type)
	// Re-registering reuses the same handle (name+type match).
	require.Same(t, km.writerMessageCount, registerMetrics(r).writerMessageCount)
}

func TestUnsupportedMetricsNotRegistered(t *testing.T) {
	t.Parallel()
	r := metrics.NewRegistry()
	registerMetrics(r)
	for _, name := range []string{
		"kafka_writer_retries_count",
		"kafka_writer_batch_seconds",
		"kafka_reader_rebalance_count",
		"kafka_reader_queue_length",
		"kafka_reader_queue_capacity",
	} {
		require.Nil(t, r.Get(name), "metric %s must not be registered (no franz-go source)", name)
	}
}

func TestFlushNilVUAndNilCollector(t *testing.T) {
	t.Parallel()
	rt := modulestest.NewRuntime(t) // init context: State() is nil
	km := registerMetrics(metrics.NewRegistry())
	mc := newMetricsCollector(km, roleWriter)
	// No VU state → no-op, no panic.
	mc.flushProduce(rt.VU, map[string]int64{"t": 1}, map[string]int64{"t": 1}, 0)
	// Nil collector → no-op, no panic.
	var nilmc *metricsCollector
	nilmc.flushClose(rt.VU)
}

func TestFlushProducePerTopic(t *testing.T) {
	t.Parallel()
	rt, registry, samples := metricsVU(t)
	mc := newMetricsCollector(registerMetrics(registry), roleWriter)

	mc.flushProduce(rt.VU,
		map[string]int64{"a": 3, "b": 2},
		map[string]int64{"a": 30, "b": 20}, 0)

	got := collectSamples(samples)
	require.Equal(t, 3.0, got["kafka_writer_message_count{topic=a}"])
	require.Equal(t, 2.0, got["kafka_writer_message_count{topic=b}"])
	require.Equal(t, 30.0, got["kafka_writer_message_bytes{topic=a}"])
	require.Equal(t, 20.0, got["kafka_writer_message_bytes{topic=b}"])
}

func TestFlushProduceFailureNoMessageCount(t *testing.T) {
	t.Parallel()
	rt, registry, samples := metricsVU(t)
	mc := newMetricsCollector(registerMetrics(registry), roleWriter)

	// A failed produce reports errors, not message counts.
	mc.flushProduce(rt.VU, map[string]int64{}, map[string]int64{}, 2)

	got := collectSamples(samples)
	require.Equal(t, 2.0, got["kafka_writer_error_count"])
	require.Empty(t, got["kafka_writer_message_count{topic=a}"])
	for k := range got {
		require.NotContains(t, k, "kafka_writer_message_count", "no message count on failure")
	}
}

func TestFlushConsumeFetchErrorNoMessageCount(t *testing.T) {
	t.Parallel()
	rt, registry, samples := metricsVU(t)
	mc := newMetricsCollector(registerMetrics(registry), roleReader)

	// A fetch error returns no messages: only the error counter is emitted.
	mc.flushConsume(rt.VU, nil, nil, nil, nil, true, false)

	got := collectSamples(samples)
	require.Equal(t, 1.0, got["kafka_reader_error_count"])
	for k := range got {
		require.NotContains(t, k, "kafka_reader_message_count", "no message count on fetch error")
	}
}

func TestHookCounterEmitsDeltas(t *testing.T) {
	t.Parallel()
	rt, registry, samples := metricsVU(t)
	mc := newMetricsCollector(registerMetrics(registry), roleWriter)

	// Two dials, then flush → delta 2.
	mc.OnBrokerConnect(kgo.BrokerMetadata{}, 5*time.Millisecond, nil, nil)
	mc.OnBrokerConnect(kgo.BrokerMetadata{}, 5*time.Millisecond, nil, nil)
	mc.flushProduce(rt.VU, nil, nil, 0)
	require.Equal(t, 2.0, collectSamples(samples)["kafka_writer_dial_count"])

	// One more dial, flush again → delta 1 (not the running total 3).
	mc.OnBrokerConnect(kgo.BrokerMetadata{}, 5*time.Millisecond, nil, nil)
	mc.flushProduce(rt.VU, nil, nil, 0)
	require.Equal(t, 1.0, collectSamples(samples)["kafka_writer_dial_count"])
}

func TestTimingMetricsAreUntagged(t *testing.T) {
	t.Parallel()
	rt, registry, samples := metricsVU(t)
	mc := newMetricsCollector(registerMetrics(registry), roleReader)

	mc.OnBrokerConnect(kgo.BrokerMetadata{}, 3*time.Millisecond, nil, nil)
	mc.OnBrokerRead(kgo.BrokerMetadata{}, apiKeyFetch, 100, time.Millisecond, 2*time.Millisecond, nil)
	mc.flushClose(rt.VU)

	got := collectSamples(samples)
	// Broker-request-level timings are emitted, untagged (no {topic=...} key).
	require.Contains(t, got, "kafka_reader_read_seconds")
	require.Contains(t, got, "kafka_reader_wait_seconds")
	require.Contains(t, got, "kafka_reader_dial_seconds")
	for k := range got {
		if len(k) >= 18 && k[:18] == "kafka_reader_read_" {
			require.NotContains(t, k, "{topic=", "timing metrics must be untagged: %s", k)
		}
	}
}

func TestFlushCloseDrainsFetchHooks(t *testing.T) {
	t.Parallel()
	rt, registry, samples := metricsVU(t)
	mc := newMetricsCollector(registerMetrics(registry), roleReader)

	mc.OnFetchBatchRead(kgo.BrokerMetadata{}, "t", 0, kgo.FetchBatchMetrics{NumRecords: 5, UncompressedBytes: 50})
	mc.flushClose(rt.VU)

	got := collectSamples(samples)
	require.Equal(t, 1.0, got["kafka_reader_fetches_count{topic=t}"])
	require.Equal(t, 5.0, got["kafka_reader_fetch_size{topic=t}"])
	require.Equal(t, 50.0, got["kafka_reader_fetch_bytes{topic=t}"])
}
