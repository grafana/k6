package kafka

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go.k6.io/k6/v2/js/modules"
)

// WriterConfig is the producer configuration (see index.d.ts WriterConfig).
// Fields where 0 is a meaningful value use pointers to distinguish "unset".
type WriterConfig struct {
	Brokers         []string    `js:"brokers"`
	Topic           string      `js:"topic"`
	AutoCreateTopic bool        `js:"autoCreateTopic"`
	Balancer        any         `js:"balancer"` // string (named) or function (custom, not honored)
	MaxAttempts     *int        `js:"maxAttempts"`
	BatchSize       int         `js:"batchSize"` // accepted-ignored
	BatchBytes      int         `js:"batchBytes"`
	BatchTimeout    int64       `js:"batchTimeout"` // nanoseconds
	ReadTimeout     int64       `js:"readTimeout"`  // accepted-ignored
	RequiredAcks    *int        `js:"requiredAcks"`
	WriteTimeout    int64       `js:"writeTimeout"` // nanoseconds
	Compression     string      `js:"compression"`
	SASL            *SASLConfig `js:"sasl"`
	TLS             *TLSConfig  `js:"tls"`
	ConnectLogger   bool        `js:"connectLogger"` // accepted-ignored
}

// ProduceMessage is a message to produce (see index.d.ts Message). `key` and
// `value` accept a string or a Uint8Array; `headers` is a plain object.
type ProduceMessage struct {
	Topic   string         `js:"topic"`
	Key     any            `js:"key"`
	Value   any            `js:"value"`
	Headers map[string]any `js:"headers"`
	Time    time.Time      `js:"time"`
}

// ProduceConfig is the argument to produce.
type ProduceConfig struct {
	Messages []ProduceMessage `js:"messages"`
}

// Writer produces messages to Kafka.
type Writer struct {
	vu           modules.VU
	client       *kgo.Client
	collector    *metricsCollector
	defaultTopic string
}

// openWriter builds a producer client from the config.
func openWriter(vu modules.VU, cfg WriterConfig, collector *metricsCollector) (*Writer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("at least one broker is required")
	}
	opts, err := writerOptions(cfg, collector)
	if err != nil {
		return nil, err
	}
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating producer: %w", err)
	}
	return &Writer{vu: vu, client: client, collector: collector, defaultTopic: cfg.Topic}, nil
}

// writerOptions assembles the franz-go options for a producer.
func writerOptions(cfg WriterConfig, collector *metricsCollector) ([]kgo.Opt, error) {
	opts, err := clientOptions(cfg.Brokers, cfg.SASL, cfg.TLS)
	if err != nil {
		return nil, err
	}
	if collector != nil {
		opts = append(opts, kgo.WithHooks(collector.hooks()...))
	}

	if cfg.Topic != "" {
		opts = append(opts, kgo.DefaultProduceTopic(cfg.Topic))
	}
	if cfg.AutoCreateTopic {
		opts = append(opts, kgo.AllowAutoTopicCreation())
	}
	if codec, ok := compressionCodec(cfg.Compression); ok {
		opts = append(opts, kgo.ProducerBatchCompression(codec))
	}
	if p := partitioner(cfg.Balancer); p != nil {
		opts = append(opts, kgo.RecordPartitioner(p))
	}

	// requiredAcks defaults to all in-sync replicas (contract default -1).
	// Non-all acks are incompatible with the idempotent producer, so disable it.
	acks := kgo.AllISRAcks()
	if cfg.RequiredAcks != nil {
		a, err := acksFromInt(*cfg.RequiredAcks)
		if err != nil {
			return nil, err
		}
		acks = a
		if *cfg.RequiredAcks != -1 {
			opts = append(opts, kgo.DisableIdempotentWrite())
		}
	}
	opts = append(opts, kgo.RequiredAcks(acks))

	if cfg.MaxAttempts != nil {
		if *cfg.MaxAttempts < 0 {
			return nil, fmt.Errorf("invalid maxAttempts %d (must be >= 0)", *cfg.MaxAttempts)
		}
		opts = append(opts, kgo.RecordRetries(*cfg.MaxAttempts), kgo.UnknownTopicRetries(*cfg.MaxAttempts))
	}
	if cfg.BatchBytes > 0 && cfg.BatchBytes <= math.MaxInt32 {
		opts = append(opts, kgo.ProducerBatchMaxBytes(int32(cfg.BatchBytes)))
	}
	if cfg.BatchTimeout > 0 {
		opts = append(opts, kgo.ProducerLinger(time.Duration(cfg.BatchTimeout)))
	}
	if cfg.WriteTimeout > 0 {
		opts = append(opts, kgo.ProduceRequestTimeout(time.Duration(cfg.WriteTimeout)))
	}

	return opts, nil
}

func acksFromInt(n int) (kgo.Acks, error) {
	switch n {
	case -1:
		return kgo.AllISRAcks(), nil
	case 0:
		return kgo.NoAck(), nil
	case 1:
		return kgo.LeaderAck(), nil
	default:
		return kgo.Acks{}, fmt.Errorf("invalid requiredAcks %d (must be -1, 0, or 1)", n)
	}
}

func compressionCodec(name string) (kgo.CompressionCodec, bool) {
	switch name {
	case codecGzip:
		return kgo.GzipCompression(), true
	case codecSnappy:
		return kgo.SnappyCompression(), true
	case codecLz4:
		return kgo.Lz4Compression(), true
	case codecZstd:
		return kgo.ZstdCompression(), true
	default:
		return kgo.CompressionCodec{}, false
	}
}

// partitioner maps a named balancer to a franz-go partitioner. A custom
// function, BALANCER_CRC32, or an unset value returns nil (franz-go's default,
// murmur2-compatible, partitioner is used).
func partitioner(balancer any) kgo.Partitioner {
	name, ok := balancer.(string)
	if !ok {
		return nil
	}
	switch name {
	case balancerRoundRobin:
		return kgo.RoundRobinPartitioner()
	case balancerLeastBytes:
		return kgo.LeastBackupPartitioner()
	case balancerHash, balancerMurmur2:
		return kgo.StickyKeyPartitioner(nil)
	default:
		return nil
	}
}

// Produce writes the messages to Kafka, blocking until the broker acknowledges
// the batch. It throws (returns an error) on the first produce failure.
func (w *Writer) Produce(config ProduceConfig) error {
	if w.vu.State() == nil {
		return errors.New("produce must be called in the VU context (default/setup/teardown function), not in init")
	}
	if w.client == nil {
		return errors.New("writer is closed")
	}
	records := make([]*kgo.Record, 0, len(config.Messages))
	for i := range config.Messages {
		records = append(records, marshalRecord(&config.Messages[i]))
	}

	results := w.client.ProduceSync(w.vu.Context(), records...)

	// Count only records that actually succeeded (per-record results), so a
	// failed or partially-failed produce does not report messages as written.
	msgCount := make(map[string]int64)
	msgBytes := make(map[string]int64)
	var errCount int64
	for _, res := range results {
		if res.Err != nil {
			errCount++
			continue
		}
		topic := res.Record.Topic
		if topic == "" {
			topic = w.defaultTopic
		}
		msgCount[topic]++
		msgBytes[topic] += int64(len(res.Record.Key) + len(res.Record.Value))
	}
	w.collector.flushProduce(w.vu, msgCount, msgBytes, errCount)

	if err := results.FirstErr(); err != nil {
		return fmt.Errorf("producing messages: %w", err)
	}
	return nil
}

// Close flushes buffered messages and closes the underlying client. franz-go's
// Close fails buffered records, so flush first to drain them.
func (w *Writer) Close() {
	if w.client == nil {
		return
	}
	ctx := context.Background()
	if w.vu != nil {
		ctx = w.vu.Context()
	}
	_ = w.client.Flush(ctx)
	w.collector.flushClose(w.vu)
	w.client.Close()
	w.client = nil
}

// marshalRecord converts a message to a franz-go record. An empty Topic falls
// back to the writer's default produce topic.
func marshalRecord(m *ProduceMessage) *kgo.Record {
	record := &kgo.Record{
		Topic: m.Topic,
		Key:   toBytes(m.Key),
		Value: toBytes(m.Value),
	}
	if !m.Time.IsZero() {
		record.Timestamp = m.Time
	}
	for key, value := range m.Headers {
		record.Headers = append(record.Headers, kgo.RecordHeader{Key: key, Value: toBytes(value)})
	}
	return record
}

// toBytes coerces a JS value to bytes: a string becomes its UTF-8 bytes,
// byte-like arrays / buffers keep their raw bytes, and anything else is
// formatted as a string.
func toBytes(v any) []byte {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		return []byte(x)
	default:
		if b, ok := coerceBytes(x); ok {
			return b
		}
		return fmt.Appendf(nil, "%v", x)
	}
}
