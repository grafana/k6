package kafka

import (
	"net"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	extensionapi "go.k6.io/k6-extension-api"
)

// Kafka protocol API keys used to attribute broker read/write timing.
const (
	apiKeyProduce int16 = 0
	apiKeyFetch   int16 = 1
)

// kafkaMetrics holds the registered k6 metric handles, shared across all
// Writer/Reader collectors in a VU (the k6 registry returns the same handle for
// a given name+type, so registering per collector is safe).
type kafkaMetrics struct {
	// Writer.
	writerMessageCount extensionapi.Metric
	writerMessageBytes extensionapi.Metric
	writerWriteCount   extensionapi.Metric
	writerErrorCount   extensionapi.Metric
	writerBatchSize    extensionapi.Metric
	writerBatchBytes   extensionapi.Metric
	writerDialCount    extensionapi.Metric
	writerDialSeconds  extensionapi.Metric
	writerWriteSeconds extensionapi.Metric
	writerWaitSeconds  extensionapi.Metric

	// Reader.
	readerMessageCount  extensionapi.Metric
	readerMessageBytes  extensionapi.Metric
	readerFetchesCount  extensionapi.Metric
	readerErrorCount    extensionapi.Metric
	readerTimeoutsCount extensionapi.Metric
	readerFetchSize     extensionapi.Metric
	readerFetchBytes    extensionapi.Metric
	readerLag           extensionapi.Metric
	readerOffset        extensionapi.Metric
	readerDialCount     extensionapi.Metric
	readerDialSeconds   extensionapi.Metric
	readerReadSeconds   extensionapi.Metric
	readerWaitSeconds   extensionapi.Metric
}

// registerMetrics registers (or reuses) all Kafka metric handles on the k6
// registry. NewMetric returns the existing metric when name+type match, so this
// is safe to call once per VU.
func registerMetrics(host extensionapi.Metrics) *kafkaMetrics {
	register := func(name string, kind extensionapi.MetricKind, unit extensionapi.MetricUnit) extensionapi.Metric {
		metric, err := host.RegisterMetric(extensionapi.MetricSpec{Name: name, Kind: kind, Unit: unit})
		if err != nil {
			return extensionapi.Metric{}
		}
		return metric
	}
	c := func(name string, unit extensionapi.MetricUnit) extensionapi.Metric {
		return register(name, extensionapi.MetricCounter, unit)
	}
	t := func(name string, unit extensionapi.MetricUnit) extensionapi.Metric {
		return register(name, extensionapi.MetricTrend, unit)
	}
	return &kafkaMetrics{
		writerMessageCount: c("kafka_writer_message_count", extensionapi.MetricUnitDefault),
		writerMessageBytes: c("kafka_writer_message_bytes", extensionapi.MetricUnitData),
		writerWriteCount:   c("kafka_writer_write_count", extensionapi.MetricUnitDefault),
		writerErrorCount:   c("kafka_writer_error_count", extensionapi.MetricUnitDefault),
		writerBatchSize:    t("kafka_writer_batch_size", extensionapi.MetricUnitDefault),
		writerBatchBytes:   t("kafka_writer_batch_bytes", extensionapi.MetricUnitData),
		writerDialCount:    c("kafka_writer_dial_count", extensionapi.MetricUnitDefault),
		writerDialSeconds:  t("kafka_writer_dial_seconds", extensionapi.MetricUnitTime),
		writerWriteSeconds: t("kafka_writer_write_seconds", extensionapi.MetricUnitTime),
		writerWaitSeconds:  t("kafka_writer_wait_seconds", extensionapi.MetricUnitTime),

		readerMessageCount:  c("kafka_reader_message_count", extensionapi.MetricUnitDefault),
		readerMessageBytes:  c("kafka_reader_message_bytes", extensionapi.MetricUnitData),
		readerFetchesCount:  c("kafka_reader_fetches_count", extensionapi.MetricUnitDefault),
		readerErrorCount:    c("kafka_reader_error_count", extensionapi.MetricUnitDefault),
		readerTimeoutsCount: c("kafka_reader_timeouts_count", extensionapi.MetricUnitDefault),
		readerFetchSize:     t("kafka_reader_fetch_size", extensionapi.MetricUnitDefault),
		readerFetchBytes:    t("kafka_reader_fetch_bytes", extensionapi.MetricUnitData),
		readerLag:           t("kafka_reader_lag", extensionapi.MetricUnitDefault),
		readerOffset:        t("kafka_reader_offset", extensionapi.MetricUnitDefault),
		readerDialCount:     c("kafka_reader_dial_count", extensionapi.MetricUnitDefault),
		readerDialSeconds:   t("kafka_reader_dial_seconds", extensionapi.MetricUnitTime),
		readerReadSeconds:   t("kafka_reader_read_seconds", extensionapi.MetricUnitTime),
		readerWaitSeconds:   t("kafka_reader_wait_seconds", extensionapi.MetricUnitTime),
	}
}

// metricRole distinguishes a producer collector from a consumer collector, so a
// shared broker read/write hook records timing only for the relevant API key.
type metricRole int

const (
	roleWriter metricRole = iota
	roleReader
)

// perTopicTrend is a batch/fetch trend value bucketed by topic (drained at flush).
type perTopicTrend struct {
	topic string
	value float64
}

// metricsCollector accumulates franz-go hook data (off the VU goroutine) into
// atomics/buffers under a mutex, and is drained into k6 samples by the owning
// Writer/Reader at produce/consume/close time (on the VU goroutine).
//
// Hook-sourced counters are monotonic totals; flush emits the delta since the
// previous flush (k6 sums counter samples). Trend values are buffered and each
// emitted as one sample.
type metricsCollector struct {
	metrics *kafkaMetrics
	role    metricRole

	mu sync.Mutex
	// Untagged counters (monotonic total + last flushed) and trend buffers.
	dialCount, dialFlushed int64
	dialSeconds            []float64
	rwSeconds              []float64 // write (writer) or read (reader) durations
	waitSeconds            []float64
	// Per-topic batch/fetch: monotonic count total + last flushed, and trend buffers.
	batchCount   map[string]int64
	batchFlushed map[string]int64
	batchSize    []perTopicTrend
	batchBytes   []perTopicTrend
}

func newMetricsCollector(m *kafkaMetrics, role metricRole) *metricsCollector {
	return &metricsCollector{
		metrics:      m,
		role:         role,
		batchCount:   map[string]int64{},
		batchFlushed: map[string]int64{},
	}
}

// --- franz-go hooks (called off the VU goroutine) ---

func (mc *metricsCollector) OnBrokerConnect(_ kgo.BrokerMetadata, initDur time.Duration, _ net.Conn, err error) {
	if err != nil {
		return
	}
	mc.mu.Lock()
	mc.dialCount++
	mc.dialSeconds = append(mc.dialSeconds, float64(initDur.Milliseconds()))
	mc.mu.Unlock()
}

func (mc *metricsCollector) OnBrokerWrite(
	_ kgo.BrokerMetadata, key int16, _ int, writeWait, timeToWrite time.Duration, err error,
) {
	if mc.role != roleWriter || key != apiKeyProduce || err != nil {
		return
	}
	mc.mu.Lock()
	mc.rwSeconds = append(mc.rwSeconds, float64(timeToWrite.Milliseconds()))
	mc.waitSeconds = append(mc.waitSeconds, float64(writeWait.Milliseconds()))
	mc.mu.Unlock()
}

func (mc *metricsCollector) OnBrokerRead(
	_ kgo.BrokerMetadata, key int16, _ int, readWait, timeToRead time.Duration, err error,
) {
	if mc.role != roleReader || key != apiKeyFetch || err != nil {
		return
	}
	mc.mu.Lock()
	mc.rwSeconds = append(mc.rwSeconds, float64(timeToRead.Milliseconds()))
	mc.waitSeconds = append(mc.waitSeconds, float64(readWait.Milliseconds()))
	mc.mu.Unlock()
}

func (mc *metricsCollector) OnProduceBatchWritten(
	_ kgo.BrokerMetadata, topic string, _ int32, m kgo.ProduceBatchMetrics,
) {
	mc.mu.Lock()
	mc.batchCount[topic]++
	mc.batchSize = append(mc.batchSize, perTopicTrend{topic, float64(m.NumRecords)})
	mc.batchBytes = append(mc.batchBytes, perTopicTrend{topic, float64(m.UncompressedBytes)})
	mc.mu.Unlock()
}

func (mc *metricsCollector) OnFetchBatchRead(_ kgo.BrokerMetadata, topic string, _ int32, m kgo.FetchBatchMetrics) {
	mc.mu.Lock()
	mc.batchCount[topic]++
	mc.batchSize = append(mc.batchSize, perTopicTrend{topic, float64(m.NumRecords)})
	mc.batchBytes = append(mc.batchBytes, perTopicTrend{topic, float64(m.UncompressedBytes)})
	mc.mu.Unlock()
}

// hooks returns the franz-go hook set for this collector's role.
func (mc *metricsCollector) hooks() []kgo.Hook {
	return []kgo.Hook{mc}
}

// --- flushing (called on the VU goroutine) ---

func addSample(out *[]extensionapi.Sample, metric extensionapi.Metric, tags extensionapi.Tags, value float64, now time.Time) {
	*out = append(*out, extensionapi.Sample{
		Metric: metric,
		Time:   now,
		Value:  value,
		Tags:   tags,
	})
}

// sampleAdder appends call-site samples during a flush.
type sampleAdder = func(now time.Time, base extensionapi.Tags, out *[]extensionapi.Sample)

// flush builds the hook-accumulated samples (plus any call-site samples the
// callback adds) and pushes them to the VU sample buffer. It is a no-op when no
// VU state is available (e.g. init context) or the collector is nil.
func (mc *metricsCollector) flush(vu extensionapi.VU, callSite sampleAdder) {
	if mc == nil || vu == nil {
		return
	}
	metricsCapability, ok := vu.(extensionapi.Metrics)
	if !ok || !inVUContext(vu) {
		return
	}
	now := time.Now()
	base := metricsCapability.CurrentTags()
	var out []extensionapi.Sample
	if callSite != nil {
		callSite(now, base, &out)
	}
	mc.drainHooks(base, now, &out)
	if len(out) == 0 {
		return
	}
	_ = metricsCapability.Emit(vu.Context(), out)
}

// drainHooks snapshots hook counter deltas and drains trend buffers into
// samples, selecting the writer or reader metric handles by role.
func (mc *metricsCollector) drainHooks(base extensionapi.Tags, now time.Time, out *[]extensionapi.Sample) {
	km := mc.metrics
	dialCountM, dialSecM := km.writerDialCount, km.writerDialSeconds
	rwSecM, waitSecM := km.writerWriteSeconds, km.writerWaitSeconds
	batchCountM, batchSizeM, batchBytesM := km.writerWriteCount, km.writerBatchSize, km.writerBatchBytes
	if mc.role == roleReader {
		dialCountM, dialSecM = km.readerDialCount, km.readerDialSeconds
		rwSecM, waitSecM = km.readerReadSeconds, km.readerWaitSeconds
		batchCountM, batchSizeM, batchBytesM = km.readerFetchesCount, km.readerFetchSize, km.readerFetchBytes
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	if d := mc.dialCount - mc.dialFlushed; d > 0 {
		addSample(out, dialCountM, base, float64(d), now)
		mc.dialFlushed = mc.dialCount
	}
	for _, v := range mc.dialSeconds {
		addSample(out, dialSecM, base, v, now)
	}
	for _, v := range mc.rwSeconds {
		addSample(out, rwSecM, base, v, now)
	}
	for _, v := range mc.waitSeconds {
		addSample(out, waitSecM, base, v, now)
	}
	mc.dialSeconds, mc.rwSeconds, mc.waitSeconds = mc.dialSeconds[:0], mc.rwSeconds[:0], mc.waitSeconds[:0]

	for topic, total := range mc.batchCount {
		if d := total - mc.batchFlushed[topic]; d > 0 {
			addSample(out, batchCountM, base.With(map[string]string{"topic": topic}), float64(d), now)
			mc.batchFlushed[topic] = total
		}
	}
	for _, pt := range mc.batchSize {
		addSample(out, batchSizeM, base.With(map[string]string{"topic": pt.topic}), pt.value, now)
	}
	for _, pt := range mc.batchBytes {
		addSample(out, batchBytesM, base.With(map[string]string{"topic": pt.topic}), pt.value, now)
	}
	mc.batchSize, mc.batchBytes = mc.batchSize[:0], mc.batchBytes[:0]
}

// flushProduce emits Writer metrics for one produce call.
func (mc *metricsCollector) flushProduce(vu extensionapi.VU, msgCount, msgBytes map[string]int64, errCount int64) {
	mc.flush(vu, func(now time.Time, base extensionapi.Tags, out *[]extensionapi.Sample) {
		km := mc.metrics
		for topic, n := range msgCount {
			tt := base.With(map[string]string{"topic": topic})
			addSample(out, km.writerMessageCount, tt, float64(n), now)
			addSample(out, km.writerMessageBytes, tt, float64(msgBytes[topic]), now)
		}
		if errCount > 0 {
			addSample(out, km.writerErrorCount, base, float64(errCount), now)
		}
	})
}

// flushConsume emits Reader metrics for one consume call.
func (mc *metricsCollector) flushConsume(
	vu extensionapi.VU, msgCount, msgBytes map[string]int64, lags, offsets []perTopicTrend, fetchErr, timedOut bool,
) {
	mc.flush(vu, func(now time.Time, base extensionapi.Tags, out *[]extensionapi.Sample) {
		km := mc.metrics
		for topic, n := range msgCount {
			tt := base.With(map[string]string{"topic": topic})
			addSample(out, km.readerMessageCount, tt, float64(n), now)
			addSample(out, km.readerMessageBytes, tt, float64(msgBytes[topic]), now)
		}
		for _, pt := range lags {
			addSample(out, km.readerLag, base.With(map[string]string{"topic": pt.topic}), pt.value, now)
		}
		for _, pt := range offsets {
			addSample(out, km.readerOffset, base.With(map[string]string{"topic": pt.topic}), pt.value, now)
		}
		if fetchErr {
			addSample(out, km.readerErrorCount, base, 1, now)
		}
		if timedOut {
			addSample(out, km.readerTimeoutsCount, base, 1, now)
		}
	})
}

// flushClose drains any remaining hook-accumulated metrics at close time.
func (mc *metricsCollector) flushClose(vu extensionapi.VU) {
	mc.flush(vu, nil)
}
