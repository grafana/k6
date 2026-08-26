package kafka

import (
	"net"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.k6.io/k6/v2/js/modules"
	"go.k6.io/k6/v2/metrics"
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
	writerMessageCount *metrics.Metric
	writerMessageBytes *metrics.Metric
	writerWriteCount   *metrics.Metric
	writerErrorCount   *metrics.Metric
	writerBatchSize    *metrics.Metric
	writerBatchBytes   *metrics.Metric
	writerDialCount    *metrics.Metric
	writerDialSeconds  *metrics.Metric
	writerWriteSeconds *metrics.Metric
	writerWaitSeconds  *metrics.Metric

	// Reader.
	readerMessageCount  *metrics.Metric
	readerMessageBytes  *metrics.Metric
	readerFetchesCount  *metrics.Metric
	readerErrorCount    *metrics.Metric
	readerTimeoutsCount *metrics.Metric
	readerFetchSize     *metrics.Metric
	readerFetchBytes    *metrics.Metric
	readerLag           *metrics.Metric
	readerOffset        *metrics.Metric
	readerDialCount     *metrics.Metric
	readerDialSeconds   *metrics.Metric
	readerReadSeconds   *metrics.Metric
	readerWaitSeconds   *metrics.Metric
}

// registerMetrics registers (or reuses) all Kafka metric handles on the k6
// registry. NewMetric returns the existing metric when name+type match, so this
// is safe to call once per VU.
func registerMetrics(r *metrics.Registry) *kafkaMetrics {
	c := func(name string, vt ...metrics.ValueType) *metrics.Metric {
		return r.MustNewMetric(name, metrics.Counter, vt...)
	}
	t := func(name string, vt ...metrics.ValueType) *metrics.Metric {
		return r.MustNewMetric(name, metrics.Trend, vt...)
	}
	return &kafkaMetrics{
		writerMessageCount: c("kafka_writer_message_count"),
		writerMessageBytes: c("kafka_writer_message_bytes", metrics.Data),
		writerWriteCount:   c("kafka_writer_write_count"),
		writerErrorCount:   c("kafka_writer_error_count"),
		writerBatchSize:    t("kafka_writer_batch_size"),
		writerBatchBytes:   t("kafka_writer_batch_bytes", metrics.Data),
		writerDialCount:    c("kafka_writer_dial_count"),
		writerDialSeconds:  t("kafka_writer_dial_seconds", metrics.Time),
		writerWriteSeconds: t("kafka_writer_write_seconds", metrics.Time),
		writerWaitSeconds:  t("kafka_writer_wait_seconds", metrics.Time),

		readerMessageCount:  c("kafka_reader_message_count"),
		readerMessageBytes:  c("kafka_reader_message_bytes", metrics.Data),
		readerFetchesCount:  c("kafka_reader_fetches_count"),
		readerErrorCount:    c("kafka_reader_error_count"),
		readerTimeoutsCount: c("kafka_reader_timeouts_count"),
		readerFetchSize:     t("kafka_reader_fetch_size"),
		readerFetchBytes:    t("kafka_reader_fetch_bytes", metrics.Data),
		readerLag:           t("kafka_reader_lag"),
		readerOffset:        t("kafka_reader_offset"),
		readerDialCount:     c("kafka_reader_dial_count"),
		readerDialSeconds:   t("kafka_reader_dial_seconds", metrics.Time),
		readerReadSeconds:   t("kafka_reader_read_seconds", metrics.Time),
		readerWaitSeconds:   t("kafka_reader_wait_seconds", metrics.Time),
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

func addSample(out *[]metrics.Sample, m *metrics.Metric, tags *metrics.TagSet, value float64, now time.Time) {
	*out = append(*out, metrics.Sample{
		TimeSeries: metrics.TimeSeries{Metric: m, Tags: tags},
		Time:       now,
		Value:      value,
	})
}

// sampleAdder appends call-site samples during a flush.
type sampleAdder = func(now time.Time, base *metrics.TagSet, out *[]metrics.Sample)

// flush builds the hook-accumulated samples (plus any call-site samples the
// callback adds) and pushes them to the VU sample buffer. It is a no-op when no
// VU state is available (e.g. init context) or the collector is nil.
func (mc *metricsCollector) flush(vu modules.VU, callSite sampleAdder) {
	if mc == nil || vu == nil {
		return
	}
	state := vu.State()
	if state == nil {
		return
	}
	now := time.Now()
	base := state.Tags.GetCurrentValues().Tags
	var out []metrics.Sample
	if callSite != nil {
		callSite(now, base, &out)
	}
	mc.drainHooks(base, now, &out)
	if len(out) == 0 {
		return
	}
	metrics.PushIfNotDone(vu.Context(), state.Samples, metrics.Samples(out))
}

// drainHooks snapshots hook counter deltas and drains trend buffers into
// samples, selecting the writer or reader metric handles by role.
func (mc *metricsCollector) drainHooks(base *metrics.TagSet, now time.Time, out *[]metrics.Sample) {
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
			addSample(out, batchCountM, base.With("topic", topic), float64(d), now)
			mc.batchFlushed[topic] = total
		}
	}
	for _, pt := range mc.batchSize {
		addSample(out, batchSizeM, base.With("topic", pt.topic), pt.value, now)
	}
	for _, pt := range mc.batchBytes {
		addSample(out, batchBytesM, base.With("topic", pt.topic), pt.value, now)
	}
	mc.batchSize, mc.batchBytes = mc.batchSize[:0], mc.batchBytes[:0]
}

// flushProduce emits Writer metrics for one produce call.
func (mc *metricsCollector) flushProduce(vu modules.VU, msgCount, msgBytes map[string]int64, errCount int64) {
	mc.flush(vu, func(now time.Time, base *metrics.TagSet, out *[]metrics.Sample) {
		km := mc.metrics
		for topic, n := range msgCount {
			tt := base.With("topic", topic)
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
	vu modules.VU, msgCount, msgBytes map[string]int64, lags, offsets []perTopicTrend, fetchErr, timedOut bool,
) {
	mc.flush(vu, func(now time.Time, base *metrics.TagSet, out *[]metrics.Sample) {
		km := mc.metrics
		for topic, n := range msgCount {
			tt := base.With("topic", topic)
			addSample(out, km.readerMessageCount, tt, float64(n), now)
			addSample(out, km.readerMessageBytes, tt, float64(msgBytes[topic]), now)
		}
		for _, pt := range lags {
			addSample(out, km.readerLag, base.With("topic", pt.topic), pt.value, now)
		}
		for _, pt := range offsets {
			addSample(out, km.readerOffset, base.With("topic", pt.topic), pt.value, now)
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
func (mc *metricsCollector) flushClose(vu modules.VU) {
	mc.flush(vu, nil)
}
