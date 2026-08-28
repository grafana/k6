package kafka

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	extensionapi "go.k6.io/k6-extension-api"
)

// defaultMaxWait is the consume poll deadline / fetch wait when maxWait is unset
// (franz-go's fetch-wait default).
const defaultMaxWait = 5 * time.Second

// commitTimeout bounds the offset commit performed when a group consumer closes.
const commitTimeout = 10 * time.Second

// ReaderConfig is the consumer configuration (see index.d.ts ReaderConfig).
// Accepted-but-ignored fields (queueCapacity, readLagInterval, …) are omitted:
// unknown JS keys are dropped during decoding.
type ReaderConfig struct {
	Brokers           []string    `js:"brokers"`
	GroupID           string      `js:"groupID"`
	GroupTopics       []string    `js:"groupTopics"`
	Topic             string      `js:"topic"`
	Partition         int32       `js:"partition"`
	MinBytes          int         `js:"minBytes"`
	MaxBytes          int         `js:"maxBytes"`
	MaxWait           string      `js:"maxWait"`
	GroupBalancers    []string    `js:"groupBalancers"`
	HeartbeatInterval int64       `js:"heartbeatInterval"`
	CommitInterval    int64       `js:"commitInterval"`
	SessionTimeout    int64       `js:"sessionTimeout"`
	RebalanceTimeout  int64       `js:"rebalanceTimeout"`
	StartOffset       string      `js:"startOffset"`
	Offset            *int64      `js:"offset"`
	MaxAttempts       *int        `js:"maxAttempts"`
	IsolationLevel    string      `js:"isolationLevel"`
	SASL              *SASLConfig `js:"sasl"`
	TLS               *TLSConfig  `js:"tls"`
}

// ConsumeConfig is the argument to consume.
type ConsumeConfig struct {
	Limit         int  `js:"limit"`
	NanoPrecision bool `js:"nanoPrecision"`
	ExpectTimeout bool `js:"expectTimeout"`
}

// ConsumedMessage is a message returned by consume (see index.d.ts Message).
type ConsumedMessage struct {
	Topic         string         `js:"topic"`
	Partition     int            `js:"partition"`
	Offset        int64          `js:"offset"`
	HighWaterMark int64          `js:"highWaterMark"`
	Key           []byte         `js:"key"`
	Value         []byte         `js:"value"`
	Headers       map[string]any `js:"headers"`
	Time          string         `js:"time"`
}

// Reader reads messages from Kafka.
type Reader struct {
	vu        extensionapi.VU
	client    *kgo.Client
	maxWait   time.Duration
	collector *metricsCollector
	group     bool // true for a consumer-group reader (commits offsets on close)
}

// openReader builds a consumer client (group or direct) from the config.
func openReader(vu extensionapi.VU, cfg ReaderConfig, collector *metricsCollector) (*Reader, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("at least one broker is required")
	}
	if cfg.GroupID != "" {
		if len(cfg.GroupTopics) == 0 && cfg.Topic == "" {
			return nil, errors.New("a consumer group requires groupTopics or topic")
		}
	} else {
		if cfg.Topic == "" {
			return nil, errors.New("a topic is required for direct (non-group) consumption")
		}
		if cfg.Offset != nil && *cfg.Offset < -1 {
			return nil, fmt.Errorf("invalid offset %d (must be >= -1)", *cfg.Offset)
		}
	}

	maxWait := defaultMaxWait
	if cfg.MaxWait != "" {
		d, err := time.ParseDuration(cfg.MaxWait)
		if err != nil {
			return nil, fmt.Errorf("invalid maxWait %q: %w", cfg.MaxWait, err)
		}
		maxWait = d
	}

	opts, err := readerOptions(vu, cfg, maxWait, collector)
	if err != nil {
		return nil, err
	}
	client, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating consumer: %w", err)
	}
	return &Reader{vu: vu, client: client, maxWait: maxWait, collector: collector, group: cfg.GroupID != ""}, nil
}

// readerOptions assembles the franz-go options for a consumer.
func readerOptions(vu extensionapi.VU, cfg ReaderConfig, maxWait time.Duration, collector *metricsCollector) ([]kgo.Opt, error) {
	opts, err := clientOptions(vu, cfg.Brokers, cfg.SASL, cfg.TLS)
	if err != nil {
		return nil, err
	}
	if collector != nil {
		opts = append(opts, kgo.WithHooks(collector.hooks()...))
	}

	if cfg.MinBytes > 0 && cfg.MinBytes <= math.MaxInt32 {
		opts = append(opts, kgo.FetchMinBytes(int32(cfg.MinBytes)))
	}
	if cfg.MaxBytes > 0 && cfg.MaxBytes <= math.MaxInt32 {
		opts = append(opts, kgo.FetchMaxBytes(int32(cfg.MaxBytes)))
	}
	opts = append(opts, kgo.FetchMaxWait(maxWait))
	opts = append(opts, kgo.FetchIsolationLevel(isolationLevel(cfg.IsolationLevel)))
	if cfg.MaxAttempts != nil {
		if *cfg.MaxAttempts < 0 {
			return nil, fmt.Errorf("invalid maxAttempts %d (must be >= 0)", *cfg.MaxAttempts)
		}
		opts = append(opts, kgo.RequestRetries(*cfg.MaxAttempts))
	}

	reset := startOffsetReset(cfg.StartOffset)
	if cfg.GroupID != "" {
		return append(opts, groupOptions(cfg, reset)...), nil
	}
	return append(opts, directOption(cfg, reset)), nil
}

// groupOptions builds the consumer-group options.
func groupOptions(cfg ReaderConfig, reset kgo.Offset) []kgo.Opt {
	topics := cfg.GroupTopics
	if len(topics) == 0 {
		topics = []string{cfg.Topic}
	}
	opts := []kgo.Opt{
		kgo.ConsumerGroup(cfg.GroupID),
		kgo.ConsumeTopics(topics...),
		kgo.ConsumeResetOffset(reset),
		kgo.Balancers(effectiveGroupBalancers(cfg.GroupBalancers)...),
	}
	if cfg.HeartbeatInterval > 0 {
		opts = append(opts, kgo.HeartbeatInterval(time.Duration(cfg.HeartbeatInterval)))
	}
	if cfg.SessionTimeout > 0 {
		opts = append(opts, kgo.SessionTimeout(time.Duration(cfg.SessionTimeout)))
	}
	if cfg.RebalanceTimeout > 0 {
		opts = append(opts, kgo.RebalanceTimeout(time.Duration(cfg.RebalanceTimeout)))
	}
	if cfg.CommitInterval > 0 {
		opts = append(opts, kgo.AutoCommitInterval(time.Duration(cfg.CommitInterval)))
	}
	return opts
}

// directOption builds the direct (single-partition) consumption option;
// `partition` defaults to 0 and `offset` takes precedence over `startOffset`.
func directOption(cfg ReaderConfig, reset kgo.Offset) kgo.Opt {
	at := reset
	if cfg.Offset != nil {
		at = directOffset(*cfg.Offset)
	}
	return kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{
		cfg.Topic: {cfg.Partition: at},
	})
}

func isolationLevel(name string) kgo.IsolationLevel {
	if name == isolationReadCommitted {
		return kgo.ReadCommitted()
	}
	return kgo.ReadUncommitted()
}

// startOffsetReset maps a START_OFFSETS value to a franz-go reset offset.
func startOffsetReset(name string) kgo.Offset {
	if name == startOffsetLast {
		return kgo.NewOffset().AtEnd()
	}
	return kgo.NewOffset().AtStart()
}

// directOffset maps the numeric `offset` to a franz-go offset: 0 = start,
// -1 = end, any positive value = that exact offset. Values below -1 are rejected
// during construction, so they never reach here.
func directOffset(offset int64) kgo.Offset {
	switch {
	case offset == -1:
		return kgo.NewOffset().AtEnd()
	case offset > 0:
		return kgo.NewOffset().At(offset)
	default:
		return kgo.NewOffset().AtStart()
	}
}

// groupBalancers maps the named group balancers to franz-go balancers.
// GROUP_BALANCER_RACK_AFFINITY has no franz-go equivalent and is ignored (never
// mapped to cooperative-sticky, which has one-way migration semantics); when no
// balancers remain, the caller defaults to range.
func groupBalancers(names []string) []kgo.GroupBalancer {
	var balancers []kgo.GroupBalancer
	for _, name := range names {
		switch name {
		case groupBalancerRange:
			balancers = append(balancers, kgo.RangeBalancer())
		case groupBalancerRoundRobin:
			balancers = append(balancers, kgo.RoundRobinBalancer())
		}
	}
	return balancers
}

// effectiveGroupBalancers resolves the balancers actually used: the mapped ones,
// or the range default when none map (unset, or only the ignored rack-affinity).
// This avoids franz-go's hidden cooperative-sticky default.
func effectiveGroupBalancers(names []string) []kgo.GroupBalancer {
	if balancers := groupBalancers(names); len(balancers) > 0 {
		return balancers
	}
	return []kgo.GroupBalancer{kgo.RangeBalancer()}
}

// Consume polls and returns up to `limit` messages, or until maxWait. By
// default a timeout before `limit` throws; with expectTimeout it returns the
// partial batch.
func (r *Reader) Consume(config ConsumeConfig) ([]ConsumedMessage, error) {
	if !inVUContext(r.vu) {
		return nil, errors.New("consume must be called in the VU context (default/setup/teardown function), not in init")
	}
	if r.client == nil {
		return nil, errors.New("reader is closed")
	}
	if config.Limit <= 0 {
		return []ConsumedMessage{}, nil
	}

	ctx, cancel := context.WithTimeout(r.vu.Context(), r.maxWait)
	defer cancel()

	messages := make([]ConsumedMessage, 0, config.Limit)
	msgCount := make(map[string]int64)
	msgBytes := make(map[string]int64)
	var lags, offsets []perTopicTrend
	// flush emits reader metrics. Message counts/bytes/lag/offset are reported
	// only when the call actually returns messages (withMessages); on error,
	// cancellation, or a non-expected timeout the call returns no messages, so
	// only the error/timeout counters (and drained hook metrics) are emitted.
	flush := func(withMessages, fetchErr, timedOut bool) {
		mc, mb, lg, of := msgCount, msgBytes, lags, offsets
		if !withMessages {
			mc, mb, lg, of = nil, nil, nil, nil
		}
		r.collector.flushConsume(r.vu, mc, mb, lg, of, fetchErr, timedOut)
	}

	for len(messages) < config.Limit {
		fetches := r.client.PollRecords(ctx, config.Limit-len(messages))
		fetches.EachPartition(func(p kgo.FetchTopicPartition) {
			for _, record := range p.Records {
				messages = append(messages, decodeRecord(record, p.HighWatermark, config.NanoPrecision))
				msgCount[record.Topic]++
				msgBytes[record.Topic] += int64(len(record.Key) + len(record.Value))
				lag := readerLag(p.HighWatermark, record.Offset)
				lags = append(lags, perTopicTrend{record.Topic, float64(lag)})
				offsets = append(offsets, perTopicTrend{record.Topic, float64(record.Offset)})
			}
		})
		if ctx.Err() != nil {
			break
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				if !errors.Is(e.Err, context.DeadlineExceeded) && !errors.Is(e.Err, context.Canceled) {
					flush(false, true, false) // fetch error: no messages returned
					return nil, fmt.Errorf("consuming from %s: %w", e.Topic, e.Err)
				}
			}
			break
		}
	}

	if len(messages) < config.Limit && ctx.Err() != nil {
		// A canceled parent (VU stopping) is not a timeout: surface it as
		// cancellation regardless of expectTimeout.
		if parentErr := r.vu.Context().Err(); parentErr != nil {
			flush(false, false, false) // cancellation: no messages, not a timeout
			return nil, fmt.Errorf("consume canceled: %w", parentErr)
		}
		if config.ExpectTimeout {
			flush(true, false, true) // returns the partial batch: count it
			return messages, nil
		}
		flush(false, false, true) // timeout, returns nil: count the timeout only
		return nil, fmt.Errorf("consume timed out after %s with %d of %d messages", r.maxWait, len(messages), config.Limit)
	}
	flush(true, false, false) // success: count the returned messages
	return messages, nil
}

// Close closes the underlying client, releasing connections (and leaving the
// group for a group consumer).
func (r *Reader) Close() {
	if r.client == nil {
		return
	}
	r.collector.flushClose(r.vu)
	if r.group {
		// Commit consumed offsets so a later member of the same group resumes
		// after them instead of reprocessing. franz-go's periodic autocommit may
		// not have fired for a short-lived consumer, so commit explicitly.
		base := context.Background()
		if r.vu != nil {
			base = r.vu.Context()
		}
		ctx, cancel := context.WithTimeout(base, commitTimeout)
		_ = r.client.CommitUncommittedOffsets(ctx)
		cancel()
	}
	r.client.Close()
	r.client = nil
}

// readerLag is the consumer lag for a message: messages remaining after this
// one, i.e. high watermark minus offset minus one, floored at zero.
func readerLag(highWatermark, offset int64) int64 {
	return max(int64(0), highWatermark-offset-1)
}

// decodeRecord converts a franz-go record to a ConsumedMessage.
func decodeRecord(record *kgo.Record, highWaterMark int64, nano bool) ConsumedMessage {
	layout := time.RFC3339
	if nano {
		layout = time.RFC3339Nano
	}
	headers := make(map[string]any, len(record.Headers))
	for _, h := range record.Headers {
		headers[h.Key] = h.Value
	}
	return ConsumedMessage{
		Topic:         record.Topic,
		Partition:     int(record.Partition),
		Offset:        record.Offset,
		HighWaterMark: highWaterMark,
		Key:           record.Key,
		Value:         record.Value,
		Headers:       headers,
		Time:          record.Timestamp.Format(layout),
	}
}
