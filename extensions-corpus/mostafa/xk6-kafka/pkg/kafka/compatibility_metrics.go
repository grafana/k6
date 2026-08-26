package kafka

import (
	"fmt"
	"strconv"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/metrics"
)

type producerMetricGroup struct {
	messages int
	writes   int
	bytes    int
}

type consumerMetricKey struct {
	topic     string
	partition int
}

type consumerMetricGroup struct {
	messages      int
	bytes         int
	lastOffset    int64
	highWaterMark int64
}

const legacyWriterMessageOverheadBytes = 23

// The legacy k6 metric names are preserved in v2, but their values are now
// derived from the Confluent-backed compatibility path rather than kafka-go's
// internal stats structs.
func (k *Kafka) reportProducerCompatibilityMetrics(
	producer *Producer,
	messages []Message,
	elapsed time.Duration,
	produceErr error,
) {
	state := k.vu.State()
	if state == nil {
		logger.WithField("error", ErrForbiddenInInitContext).Error(ErrForbiddenInInitContext)
		common.Throw(k.vu.Runtime(), ErrForbiddenInInitContext)
	}

	ctx := k.vu.Context()
	if ctx == nil {
		err := NewXk6KafkaError(cannotReportStats, "Cannot report writer stats, no context.", nil)
		logger.WithField("error", err).Info(err)
		common.Throw(k.vu.Runtime(), err)
	}

	ctm := state.Tags.GetCurrentValues()
	groups := collectProducerMetricGroups(producer, messages)
	if len(groups) == 0 {
		groups = map[string]producerMetricGroup{"": {}}
	}

	errorValue := 0.0
	if produceErr != nil {
		errorValue = 1 / float64(len(groups))
	}

	now := time.Now()
	for topic, group := range groups {
		sampleTags := ctm.Tags.With("topic", topic)

		writes := 0.0
		messageCount := 0.0
		messageBytes := 0.0
		batchSize := 0.0
		batchBytes := 0.0
		batchTime := time.Duration(0)
		writeTime := time.Duration(0)

		if produceErr == nil {
			writes = float64(group.writes)
			messageCount = float64(group.messages)
			messageBytes = float64(group.bytes)
			if group.writes > 0 {
				batchSize = float64(group.messages) / float64(group.writes)
				batchBytes = float64(group.bytes) / float64(group.writes)
				batchTime = elapsed / time.Duration(group.writes)
			}
			writeTime = elapsed
		}

		metrics.PushIfNotDone(ctx, state.Samples, metrics.ConnectedSamples{
			Samples: []metrics.Sample{
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterWrites,
						Tags:   sampleTags,
					},
					Value:    writes,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterMessages,
						Tags:   sampleTags,
					},
					Value:    messageCount,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterBytes,
						Tags:   sampleTags,
					},
					Value:    messageBytes,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterErrors,
						Tags:   sampleTags,
					},
					Value:    errorValue,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterBatchTime,
						Tags:   sampleTags,
					},
					Value:    metrics.D(batchTime),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterBatchQueueTime,
						Tags:   sampleTags,
					},
					Value:    metrics.D(0),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterWriteTime,
						Tags:   sampleTags,
					},
					Value:    metrics.D(writeTime),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterWaitTime,
						Tags:   sampleTags,
					},
					Value:    metrics.D(0),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterRetries,
						Tags:   sampleTags,
					},
					Value:    0,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterBatchSize,
						Tags:   sampleTags,
					},
					Value:    batchSize,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterBatchBytes,
						Tags:   sampleTags,
					},
					Value:    batchBytes,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterMaxAttempts,
						Tags:   sampleTags,
					},
					Value:    compatibilityConfigFloat(producer.config, "message.send.max.retries"),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterMaxBatchSize,
						Tags:   sampleTags,
					},
					Value:    compatibilityConfigFloat(producer.config, "batch.num.messages"),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterBatchTimeout,
						Tags:   sampleTags,
					},
					Value:    metrics.D(compatibilityConfigDuration(producer.config, "linger.ms")),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterReadTimeout,
						Tags:   sampleTags,
					},
					Value:    metrics.D(compatibilityConfigDuration(producer.config, "socket.timeout.ms")),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterWriteTimeout,
						Tags:   sampleTags,
					},
					Value:    metrics.D(compatibilityConfigDuration(producer.config, "message.timeout.ms")),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterRequiredAcks,
						Tags:   sampleTags,
					},
					Value:    compatibilityRequiredAcks(producer.config),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.WriterAsync,
						Tags:   sampleTags,
					},
					Value:    metrics.B(false),
					Metadata: ctm.Metadata,
				},
			},
			Tags: sampleTags,
			Time: now,
		})
	}
}

func (k *Kafka) reportConsumerCompatibilityMetrics(
	consumer *Consumer,
	messages []Message,
	elapsed time.Duration,
	consumeErr error,
	timedOut bool,
) {
	state := k.vu.State()
	if state == nil {
		logger.WithField("error", ErrForbiddenInInitContext).Error(ErrForbiddenInInitContext)
		common.Throw(k.vu.Runtime(), ErrForbiddenInInitContext)
	}

	ctx := k.vu.Context()
	if ctx == nil {
		err := NewXk6KafkaError(noContextError, "No context.", nil)
		logger.WithField("error", err).Info(err)
		common.Throw(k.vu.Runtime(), err)
	}

	ctm := state.Tags.GetCurrentValues()
	groups := collectConsumerMetricGroups(consumer, messages)
	if len(groups) == 0 {
		groups = map[consumerMetricKey]consumerMetricGroup{{
			topic:     consumerCompatibilityTopic(consumer),
			partition: 0,
		}: {}}
	}

	timeoutValue := 0.0
	errorValue := 0.0
	if consumeErr != nil {
		if timedOut {
			timeoutValue = 1 / float64(len(groups))
		} else {
			errorValue = 1 / float64(len(groups))
		}
	}

	now := time.Now()
	clientID := compatibilityConfigString(consumer.config, "client.id")
	for key, group := range groups {
		sampleTags := ctm.Tags.With("topic", key.topic)
		sampleTags = sampleTags.With("clientid", clientID)
		sampleTags = sampleTags.With("partition", strconv.Itoa(key.partition))

		fetches := 0.0
		dials := 0.0
		messageCount := float64(group.messages)
		messageBytes := float64(group.bytes)
		fetchSize := float64(group.messages)
		fetchBytes := float64(group.bytes)
		readTime := time.Duration(0)

		if len(messages) > 0 || consumeErr != nil {
			dials = 1 / float64(len(groups))
			fetches = compatibilityConsumerFetches(group.messages, len(groups), consumeErr)
			readTime = elapsed
		}

		lag := float64(0)
		if group.highWaterMark > group.lastOffset {
			lag = float64(group.highWaterMark - group.lastOffset)
		}

		metrics.PushIfNotDone(ctx, state.Samples, metrics.ConnectedSamples{
			Samples: []metrics.Sample{
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderDials,
						Tags:   sampleTags,
					},
					Value:    dials,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderFetches,
						Tags:   sampleTags,
					},
					Value:    fetches,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderMessages,
						Tags:   sampleTags,
					},
					Value:    messageCount,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderBytes,
						Tags:   sampleTags,
					},
					Value:    messageBytes,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderRebalances,
						Tags:   sampleTags,
					},
					Value:    0,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderTimeouts,
						Tags:   sampleTags,
					},
					Value:    timeoutValue,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderErrors,
						Tags:   sampleTags,
					},
					Value:    errorValue,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderDialTime,
						Tags:   sampleTags,
					},
					Value:    metrics.D(0),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderReadTime,
						Tags:   sampleTags,
					},
					Value:    metrics.D(readTime),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderWaitTime,
						Tags:   sampleTags,
					},
					Value:    metrics.D(0),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderFetchSize,
						Tags:   sampleTags,
					},
					Value:    fetchSize,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderFetchBytes,
						Tags:   sampleTags,
					},
					Value:    fetchBytes,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderOffset,
						Tags:   sampleTags,
					},
					Value:    float64(group.lastOffset),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderLag,
						Tags:   sampleTags,
					},
					Value:    lag,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderMinBytes,
						Tags:   sampleTags,
					},
					Value:    compatibilityConfigFloat(consumer.config, "fetch.min.bytes"),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderMaxBytes,
						Tags:   sampleTags,
					},
					Value:    compatibilityConfigFloat(consumer.config, "fetch.max.bytes"),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderMaxWait,
						Tags:   sampleTags,
					},
					Value:    metrics.D(compatibilityConfigDuration(consumer.config, "fetch.wait.max.ms")),
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderQueueLength,
						Tags:   sampleTags,
					},
					Value:    0,
					Metadata: ctm.Metadata,
				},
				{
					Time: now,
					TimeSeries: metrics.TimeSeries{
						Metric: k.metrics.ReaderQueueCapacity,
						Tags:   sampleTags,
					},
					Value:    0,
					Metadata: ctm.Metadata,
				},
			},
			Tags: sampleTags,
			Time: now,
		})
	}
}

func collectProducerMetricGroups(producer *Producer, messages []Message) map[string]producerMetricGroup {
	groups := make(map[string]producerMetricGroup)
	for _, msg := range messages {
		topic := msg.Topic
		if topic == "" && producer != nil {
			topic = producer.defaultTopic
		}

		group := groups[topic]
		group.messages++
		group.writes++
		group.bytes += compatibilityProducerMessageBytes(msg)
		groups[topic] = group
	}

	return groups
}

func collectConsumerMetricGroups(
	consumer *Consumer,
	messages []Message,
) map[consumerMetricKey]consumerMetricGroup {
	groups := make(map[consumerMetricKey]consumerMetricGroup)
	for _, msg := range messages {
		key := consumerMetricKey{
			topic:     msg.Topic,
			partition: msg.Partition,
		}
		if key.topic == "" {
			key.topic = consumerCompatibilityTopic(consumer)
		}

		group := groups[key]
		group.messages++
		group.bytes += compatibilityMessageBytes(msg)
		group.lastOffset = msg.Offset
		group.highWaterMark = msg.HighWaterMark
		groups[key] = group
	}

	return groups
}

func consumerCompatibilityTopic(consumer *Consumer) string {
	if consumer == nil {
		return ""
	}

	return consumer.topic
}

func compatibilityMessageBytes(msg Message) int {
	size := len(msg.Key) + len(msg.Value)
	for key, value := range msg.Headers {
		size += len(key)
		size += len(fmt.Appendf(nil, "%v", value))
	}

	return size
}

func compatibilityProducerMessageBytes(msg Message) int {
	return legacyWriterMessageOverheadBytes + compatibilityMessageBytes(msg)
}

func compatibilityConsumerFetches(messageCount int, groupCount int, consumeErr error) float64 {
	if groupCount <= 0 {
		return 0
	}
	if messageCount == 0 && consumeErr == nil {
		return 0
	}

	return float64(messageCount) + 1/float64(groupCount)
}

func compatibilityConfigFloat(config ckafka.ConfigMap, key string) float64 {
	value, ok := config[key]
	if !ok {
		return 0
	}

	return compatibilityValueToFloat(value)
}

func compatibilityConfigString(config ckafka.ConfigMap, key string) string {
	value, ok := config[key]
	if !ok {
		return ""
	}

	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func compatibilityConfigDuration(config ckafka.ConfigMap, key string) time.Duration {
	value, ok := config[key]
	if !ok {
		return 0
	}

	milliseconds, ok := compatibilityValueToInt(value)
	if !ok {
		return 0
	}

	return time.Duration(milliseconds) * time.Millisecond
}

func compatibilityRequiredAcks(config ckafka.ConfigMap) float64 {
	value, ok := config["acks"]
	if !ok {
		return 0
	}

	switch typed := value.(type) {
	case string:
		if typed == "all" {
			return -1
		}
	default:
		return compatibilityValueToFloat(typed)
	}

	parsed, err := strconv.Atoi(fmt.Sprint(value))
	if err != nil {
		return 0
	}

	return float64(parsed)
}

func compatibilityValueToFloat(value any) float64 {
	switch typed := value.(type) {
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case float32:
		return float64(typed)
	case float64:
		return typed
	case bool:
		return metrics.B(typed)
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0
		}
		return parsed
	default:
		return 0
	}
}

func compatibilityValueToInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float32:
		return int(typed), true
	case float64:
		return int(typed), true
	case string:
		parsed, err := strconv.Atoi(typed)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}
