package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
)

var (
	// Group balancers.
	groupBalancerRange        = "group_balancer_range"
	groupBalancerRoundRobin   = "group_balancer_round_robin"
	groupBalancerRackAffinity = "group_balancer_rack_affinity"

	// Isolation levels.
	isolationLevelReadUncommitted = "isolation_level_read_uncommitted"
	isolationLevelReadCommitted   = "isolation_level_read_committed"

	// Start offsets.
	lastOffset  = "start_offsets_last_offset"
	firstOffset = "start_offsets_first_offset"

	RebalanceTimeout       = time.Second * 5
	HeartbeatInterval      = time.Second * 3
	SessionTimeout         = time.Second * 30
	PartitionWatchInterval = time.Second * 5
	JoinGroupBackoff       = time.Second * 5
	RetentionTime          = time.Hour * 24
)

const consumerSeekArgumentCount = 2

type ReaderConfig struct {
	WatchPartitionChanges  bool          `json:"watchPartitionChanges"`
	ConnectLogger          bool          `json:"connectLogger"`
	Partition              int           `json:"partition"`
	QueueCapacity          int           `json:"queueCapacity"`
	MinBytes               int           `json:"minBytes"`
	MaxBytes               int           `json:"maxBytes"`
	MaxAttempts            int           `json:"maxAttempts"`
	GroupID                string        `json:"groupId"`
	Topic                  string        `json:"topic"`
	IsolationLevel         string        `json:"isolationLevel"`
	StartOffset            string        `json:"startOffset"`
	Offset                 int64         `json:"offset"`
	Brokers                []string      `json:"brokers"`
	GroupTopics            []string      `json:"groupTopics"`
	GroupBalancers         []string      `json:"groupBalancers"`
	MaxWait                Duration      `json:"maxWait"`
	ReadBatchTimeout       time.Duration `json:"readBatchTimeout"`
	ReadLagInterval        time.Duration `json:"readLagInterval"`
	HeartbeatInterval      time.Duration `json:"heartbeatInterval"`
	CommitInterval         time.Duration `json:"commitInterval"`
	PartitionWatchInterval time.Duration `json:"partitionWatchInterval"`
	SessionTimeout         time.Duration `json:"sessionTimeout"`
	RebalanceTimeout       time.Duration `json:"rebalanceTimeout"`
	JoinGroupBackoff       time.Duration `json:"joinGroupBackoff"`
	RetentionTime          time.Duration `json:"retentionTime"`
	ReadBackoffMin         time.Duration `json:"readBackoffMin"`
	ReadBackoffMax         time.Duration `json:"readBackoffMax"`
	OffsetOutOfRangeError  bool          `json:"offsetOutOfRangeError"` // deprecated, do not use
	SASL                   SASLConfig    `json:"sasl"`
	TLS                    TLSConfig     `json:"tls"`
}

type ConsumeConfig struct {
	Limit         int64 `json:"limit"`
	MaxMessages   int64 `json:"maxMessages"`
	NanoPrecision bool  `json:"nanoPrecision"`
	ExpectTimeout bool  `json:"expectTimeout"`
}

type Duration struct {
	time.Duration
}

func (d Duration) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(d.String())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal duration: %w", err)
	}
	return data, nil
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return fmt.Errorf("failed to unmarshal duration: %w", err)
	}

	switch value := v.(type) {
	case string:
		var err error
		d.Duration, err = time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("failed to parse duration: %w", err)
		}
		return nil
	default:
		return ErrInvalidDuration
	}
}

// readerClass is the deprecated Reader compatibility constructor exposed to JS.
// nolint: funlen
func (k *Kafka) readerClass(call sobek.ConstructorCall) *sobek.Object {
	return k.compatConsumerClass(call)
}

func (k *Kafka) consumerClass(call sobek.ConstructorCall) *sobek.Object {
	return k.compatConsumerClass(call)
}

func (k *Kafka) compatConsumerClass(call sobek.ConstructorCall) *sobek.Object {
	runtime := k.vu.Runtime()
	var readerConfig ReaderConfig
	if len(call.Arguments) == 0 {
		common.Throw(runtime, ErrNotEnoughArguments)
	}

	readerConfigParams := exportArgumentMap(runtime, call.Argument(0), "reader config")
	if groupID, ok := readerConfigParams["groupID"]; ok {
		if _, hasGroupId := readerConfigParams["groupId"]; !hasGroupId {
			readerConfigParams["groupId"] = groupID
		}
	}
	decodeArgumentMap(runtime, readerConfigParams, &readerConfig, "reader config")

	consumer, err := NewConsumerFromReaderConfig(&readerConfig)
	if err != nil {
		common.Throw(runtime, err)
	}

	consumerObject := runtime.NewObject()
	if err := consumerObject.Set("This", consumer); err != nil {
		common.Throw(runtime, err)
	}

	err = consumerObject.Set("consume", func(call sobek.FunctionCall) sobek.Value {
		var consumeConfig *ConsumeConfig
		if len(call.Arguments) == 0 {
			common.Throw(runtime, ErrNotEnoughArguments)
		}

		decodeArgument(runtime, call.Argument(0), &consumeConfig, "consume config")

		return runtime.ToValue(k.consumeWithConsumer(consumer, consumeConfig))
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	err = consumerObject.Set("seek", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) < consumerSeekArgumentCount {
			common.Throw(runtime, ErrNotEnoughArguments)
		}

		var partition int
		var offset int64
		if err := runtime.ExportTo(call.Argument(0), &partition); err != nil {
			common.Throw(runtime, newInvalidConfigError("partition", err))
		}
		if err := runtime.ExportTo(call.Argument(1), &offset); err != nil {
			common.Throw(runtime, newInvalidConfigError("offset", err))
		}

		if err := consumer.Seek(partition, offset); err != nil {
			common.Throw(runtime, err)
		}
		return sobek.Undefined()
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	err = consumerObject.Set("position", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			common.Throw(runtime, ErrNotEnoughArguments)
		}

		var partition int
		if err := runtime.ExportTo(call.Argument(0), &partition); err != nil {
			common.Throw(runtime, newInvalidConfigError("partition", err))
		}

		position, err := consumer.Position(partition)
		if err != nil {
			common.Throw(runtime, err)
		}
		return runtime.ToValue(position)
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	err = consumerObject.Set("commitOffsets", func(_ sobek.FunctionCall) sobek.Value {
		if err := consumer.CommitOffsets(ensureContext(k.vu.Context())); err != nil {
			common.Throw(runtime, err)
		}
		return sobek.Undefined()
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	err = consumerObject.Set("stats", func(_ sobek.FunctionCall) sobek.Value {
		stats := consumer.Stats()
		return runtime.ToValue(map[string]any{
			"assignments": stats.Assignments,
			// Backward-compatible alias.
			"Assignments": stats.Assignments,
		})
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	err = consumerObject.Set("close", func(_ sobek.FunctionCall) sobek.Value {
		if err := consumer.Close(); err != nil {
			common.Throw(runtime, err)
		}

		return sobek.Undefined()
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	if err := freeze(consumerObject); err != nil {
		common.Throw(runtime, err)
	}

	return runtime.ToValue(consumerObject).ToObject(runtime)
}

func (c *ConsumeConfig) effectiveLimit() int {
	if c == nil {
		return 1
	}
	switch {
	case c.MaxMessages > 0:
		return int(c.MaxMessages)
	case c.Limit > 0:
		return int(c.Limit)
	default:
		return 1
	}
}

func (k *Kafka) consumeWithConsumer(
	consumer *Consumer,
	consumeConfig *ConsumeConfig,
) []map[string]any {
	if consumer == nil {
		throwConfigError(k.vu.Runtime(), newMissingConfigError("consumer"))
		return nil
	}
	if consumeConfig == nil {
		throwConfigError(k.vu.Runtime(), newMissingConfigError("consume config"))
		return nil
	}

	if state := k.vu.State(); state == nil {
		logger.WithField("error", ErrForbiddenInInitContext).Error(ErrForbiddenInInitContext)
		common.Throw(k.vu.Runtime(), ErrForbiddenInInitContext)
	}

	ctx := k.vu.Context()
	if ctx == nil {
		err := NewXk6KafkaError(noContextError, "No context.", nil)
		logger.WithField("error", err).Info(err)
		common.Throw(k.vu.Runtime(), err)
	}

	startedAt := time.Now()
	startupGraceDeadline := startedAt.Add(consumerCompatibilityStartupGrace)
	limit := consumeConfig.effectiveLimit()
	messages := make([]Message, 0, limit)
	for len(messages) < limit {
		nextMessages, timedOut, shouldRetry, err := consumeConsumerCompatibilityBatch(
			ctx,
			consumer,
			len(messages),
			startupGraceDeadline,
		)
		messages = append(messages, nextMessages...)
		if err != nil {
			if shouldRetry {
				continue
			}
			k.reportConsumerCompatibilityMetrics(consumer, messages, time.Since(startedAt), err, timedOut)

			if consumeConfig.ExpectTimeout && timedOut {
				return messagesToJS(messages, consumeConfig.NanoPrecision)
			}

			logger.WithField("error", err).Error(err)
			common.Throw(k.vu.Runtime(), err)
		}

		if len(messages) == limit {
			break
		}

		startupGraceDeadline = time.Time{}
	}

	k.reportConsumerCompatibilityMetrics(consumer, messages, time.Since(startedAt), nil, false)

	return messagesToJS(messages, consumeConfig.NanoPrecision)
}

const consumerCompatibilityStartupGrace = 1500 * time.Millisecond

func newConsumerCompatibilityTimeoutContext(
	parent context.Context,
	consumer *Consumer,
) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, confluentConsumerMaxWait(consumer))
}

func consumeConsumerCompatibilityBatch(
	parent context.Context,
	consumer *Consumer,
	messageCount int,
	startupGraceDeadline time.Time,
) ([]Message, bool, bool, error) {
	consumeCtx, cancel := newConsumerCompatibilityTimeoutContext(parent, consumer)
	defer cancel()

	nextMessages, err := consumer.Consume(consumeCtx, 1)
	if err == nil {
		return nextMessages, false, false, nil
	}

	timedOut := isConsumerCompatibilityTimeout(parent, consumeCtx, err)
	shouldRetry := shouldRetryConsumerCompatibilityRead(
		parent,
		consumeCtx,
		err,
		messageCount,
		time.Now().Before(startupGraceDeadline),
	)

	return nextMessages, timedOut, shouldRetry, err
}

func isConsumerCompatibilityTimeout(parentCtx, consumeCtx context.Context, err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	if parentCtx != nil && errors.Is(parentCtx.Err(), context.DeadlineExceeded) {
		return true
	}

	return consumeCtx != nil && errors.Is(consumeCtx.Err(), context.DeadlineExceeded)
}

func shouldRetryConsumerCompatibilityRead(
	parentCtx context.Context,
	consumeCtx context.Context,
	err error,
	messageCount int,
	startupGraceAvailable bool,
) bool {
	if !startupGraceAvailable || messageCount != 0 || err == nil {
		return false
	}

	if isConsumerCompatibilityTimeout(parentCtx, consumeCtx, err) {
		return true
	}

	if parentCtx != nil && parentCtx.Err() != nil {
		return false
	}

	return consumeCtx != nil && errors.Is(consumeCtx.Err(), context.Canceled) && errors.Is(err, context.Canceled)
}

func confluentConsumerMaxWait(consumer *Consumer) time.Duration {
	if consumer == nil {
		return defaultConfluentTimeout
	}

	raw, ok := consumer.config["fetch.wait.max.ms"]
	if !ok {
		return defaultConfluentTimeout
	}

	switch value := raw.(type) {
	case int:
		return time.Duration(value) * time.Millisecond
	case int32:
		return time.Duration(value) * time.Millisecond
	case int64:
		return time.Duration(value) * time.Millisecond
	case float64:
		return time.Duration(value) * time.Millisecond
	default:
		return defaultConfluentTimeout
	}
}

func messagesToJS(messages []Message, nanoPrecision bool) []map[string]any {
	converted := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		messageTime := msg.Time.Format(time.RFC3339)
		if nanoPrecision {
			messageTime = msg.Time.Format(time.RFC3339Nano)
		}

		message := map[string]any{
			"topic":         msg.Topic,
			"partition":     msg.Partition,
			"offset":        msg.Offset,
			"time":          messageTime,
			"highWaterMark": msg.HighWaterMark,
			"headers":       map[string]any{},
		}

		if headers, ok := message["headers"].(map[string]any); ok {
			maps.Copy(headers, msg.Headers)
		}

		if len(msg.Key) > 0 {
			message["key"] = msg.Key
		}
		if len(msg.Value) > 0 {
			message["value"] = msg.Value
		}

		converted = append(converted, message)
	}

	return converted
}
