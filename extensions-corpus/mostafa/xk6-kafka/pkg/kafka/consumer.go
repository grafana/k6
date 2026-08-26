package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type ConsumerStats struct {
	Assignments int
}

const (
	maxInt32Value = 1<<31 - 1
	minInt32Value = -1 << 31
)

type Consumer struct {
	client      *ckafka.Consumer
	saslContext SASLContext
	config      ckafka.ConfigMap
	topic       string

	mu          sync.Mutex
	closeCond   *sync.Cond
	activeCalls int
	closing     bool
}

var errConsumerClosing = errors.New("consumer is closing")

func NewConsumerFromReaderConfig(readerConfig *ReaderConfig) (*Consumer, error) {
	config, err := readerConfigToConfluentConfigMap(readerConfig)
	if err != nil {
		return nil, err
	}
	if readerConfig != nil && readerConfig.GroupID == "" {
		if err := setConfluentConfigValue(
			config,
			"group.id",
			fmt.Sprintf("xk6-kafka-reader-%d", time.Now().UnixNano()),
		); err != nil {
			return nil, err
		}
		if err := setConfluentConfigValue(config, "enable.auto.commit", false); err != nil {
			return nil, err
		}
	}

	saslContext, err := NewSaslContext(readerConfig.SASL, readerConfig.Brokers, SASLContextOpts{})
	if err != nil {
		return nil, err
	}

	client, err := ckafka.NewConsumer(&config)
	if err != nil {
		return nil, NewXk6KafkaError(failedCreateConsumer, "Failed to create consumer.", err)
	}

	consumer := &Consumer{
		client:      client,
		saslContext: saslContext,
		config:      cloneConfluentConfigMap(config),
	}
	consumer.closeCond = sync.NewCond(&consumer.mu)

	if readerConfig == nil {
		return nil, newMissingConfigError("reader config")
	}

	switch {
	case readerConfig.GroupID != "":
		topics := append([]string(nil), readerConfig.GroupTopics...)
		if len(topics) == 0 && readerConfig.Topic != "" {
			topics = []string{readerConfig.Topic}
		}
		if len(topics) == 0 {
			_ = client.Close()
			return nil, newInvalidConfigError("reader config", errGroupTopicsMustNotBeEmpty)
		}
		if len(topics) == 1 {
			consumer.topic = topics[0]
		}
		if err := client.SubscribeTopics(topics, nil); err != nil {
			_ = client.Close()
			return nil, NewXk6KafkaError(failedCreateConsumer, "Failed to subscribe consumer.", err)
		}
	default:
		if readerConfig.Topic == "" {
			_ = client.Close()
			return nil, newInvalidConfigError("reader config", errTopicMustNotBeEmpty)
		}

		offset, err := confluentOffset(readerConfig.StartOffset, readerConfig.Offset)
		if err != nil {
			_ = client.Close()
			return nil, err
		}

		consumer.topic = readerConfig.Topic
		partition, err := consumerPartition(readerConfig.Partition, "reader config")
		if err != nil {
			_ = client.Close()
			return nil, err
		}
		if err := client.Assign([]ckafka.TopicPartition{{
			Topic:     &consumer.topic,
			Partition: partition,
			Offset:    offset,
		}}); err != nil {
			_ = client.Close()
			return nil, NewXk6KafkaError(failedCreateConsumer, "Failed to assign consumer.", err)
		}
	}

	return consumer, nil
}

func (c *Consumer) Consume(ctx context.Context, limit int) ([]Message, error) {
	if c == nil {
		return nil, newMissingConfigError("consumer")
	}
	client, err := c.beginOperation()
	if err != nil {
		if errors.Is(err, errConsumerClosing) {
			return nil, consumerReadError(err)
		}
		return nil, err
	}
	defer c.endOperation()

	if limit <= 0 {
		limit = 1
	}
	ctx = ensureContext(ctx)

	messages := make([]Message, 0, limit)
	for len(messages) < limit {
		if c.closeRequested() {
			return messages, consumerReadError(errConsumerClosing)
		}
		if err := consumerContextCause(ctx); err != nil {
			return messages, consumerContextError(err)
		}

		msg, err := c.consumerReadMessage(ctx, client)
		if err != nil {
			var kafkaErr ckafka.Error
			if errors.As(err, &kafkaErr) && kafkaErr.IsTimeout() {
				continue
			}
			return messages, normalizeConsumerReadError(ctx, err)
		}

		if msg != nil {
			messages = append(messages, confluentMessageToMessage(msg))
		}
	}

	return messages, nil
}

func consumerContextError(err error) error {
	return NewXk6KafkaError(failedReadMessage, "Consumer context cancelled.", err)
}

func consumerReadError(err error) error {
	return NewXk6KafkaError(failedReadMessage, "Failed to consume message.", err)
}

func normalizeConsumerReadError(ctx context.Context, err error) error {
	if ctxErr := consumerContextCause(ctx); ctxErr != nil {
		return consumerContextError(ctxErr)
	}

	return consumerReadError(err)
}

func consumerContextCause(ctx context.Context) error {
	if ctx == nil {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return context.DeadlineExceeded
	}

	return nil
}

func (c *Consumer) Seek(partition int, offset int64) error {
	if c == nil {
		return newMissingConfigError("consumer")
	}
	if c.topic == "" {
		return newInvalidConfigError("consumer", errSeekRequiresSingleConfiguredTopic)
	}
	client, err := c.beginOperation()
	if err != nil {
		if errors.Is(err, errConsumerClosing) {
			return NewXk6KafkaError(failedSetOffset, "Failed to seek consumer offset.", err)
		}
		return err
	}
	defer c.endOperation()

	partitionValue, err := consumerPartition(partition, "partition")
	if err != nil {
		return err
	}

	err = client.Seek(ckafka.TopicPartition{
		Topic:     &c.topic,
		Partition: partitionValue,
		Offset:    ckafka.Offset(offset),
	}, -1)
	if err != nil {
		return NewXk6KafkaError(failedSetOffset, "Failed to seek consumer offset.", err)
	}

	return nil
}

func (c *Consumer) Position(partition int) (int64, error) {
	if c == nil {
		return 0, newMissingConfigError("consumer")
	}
	if c.topic == "" {
		return 0, newInvalidConfigError("consumer", errPositionRequiresSingleConfiguredTopic)
	}
	client, err := c.beginOperation()
	if err != nil {
		if errors.Is(err, errConsumerClosing) {
			return 0, NewXk6KafkaError(failedSetOffset, "Failed to query consumer position.", err)
		}
		return 0, err
	}
	defer c.endOperation()

	partitionValue, err := consumerPartition(partition, "partition")
	if err != nil {
		return 0, err
	}

	positions, err := client.Position([]ckafka.TopicPartition{{
		Topic:     &c.topic,
		Partition: partitionValue,
	}})
	if err != nil {
		return 0, NewXk6KafkaError(failedSetOffset, "Failed to query consumer position.", err)
	}
	if len(positions) == 0 {
		return 0, NewXk6KafkaError(failedSetOffset, "Failed to query consumer position.", errNoPositionsReturned)
	}

	return int64(positions[0].Offset), nil
}

func (c *Consumer) CommitOffsets(ctx context.Context) error {
	if c == nil {
		return newMissingConfigError("consumer")
	}
	client, err := c.beginOperation()
	if err != nil {
		if errors.Is(err, errConsumerClosing) {
			return NewXk6KafkaError(failedCommitConsumer, "Failed to commit consumer offsets.", err)
		}
		return err
	}
	defer c.endOperation()

	ctx = ensureContext(ctx)
	if err := ctx.Err(); err != nil {
		return NewXk6KafkaError(failedCommitConsumer, "Consumer context cancelled.", err)
	}

	if _, err := client.Commit(); err != nil {
		return NewXk6KafkaError(failedCommitConsumer, "Failed to commit consumer offsets.", err)
	}

	return nil
}

func (c *Consumer) Close() error {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	if c.client == nil {
		c.mu.Unlock()
		return nil
	}

	c.closing = true
	for c.activeCalls > 0 {
		c.closeCond.Wait()
	}

	client := c.client
	c.client = nil
	c.mu.Unlock()

	if err := client.Close(); err != nil {
		return NewXk6KafkaError(failedCreateConsumer, "Failed to close consumer.", err)
	}
	return nil
}

func (c *Consumer) Stats() ConsumerStats {
	if c == nil {
		return ConsumerStats{}
	}
	client, err := c.beginOperation()
	if err != nil {
		return ConsumerStats{}
	}
	defer c.endOperation()

	assignments, err := client.Assignment()
	if err != nil {
		return ConsumerStats{}
	}

	return ConsumerStats{Assignments: len(assignments)}
}

func (c *Consumer) beginOperation() (*ckafka.Consumer, error) {
	if c == nil {
		return nil, newMissingConfigError("consumer")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client == nil {
		return nil, newMissingConfigError("consumer")
	}
	if c.closing {
		return nil, errConsumerClosing
	}

	c.activeCalls++
	return c.client, nil
}

func consumerPartition(partition int, component string) (int32, error) {
	if partition < minInt32Value || partition > maxInt32Value {
		return 0, newInvalidConfigError(component, fmt.Errorf("%w: %d", errPartitionOutOfRange, partition))
	}

	return int32(partition), nil
}

func (c *Consumer) endOperation() {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeCalls > 0 {
		c.activeCalls--
	}
	if c.activeCalls == 0 && c.closeCond != nil {
		c.closeCond.Broadcast()
	}
}

func (c *Consumer) closeRequested() bool {
	if c == nil {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return c.closing
}

func confluentMessageToMessage(msg *ckafka.Message) Message {
	converted := Message{
		Key:       msg.Key,
		Value:     msg.Value,
		Time:      msg.Timestamp,
		Partition: int(msg.TopicPartition.Partition),
		Offset:    int64(msg.TopicPartition.Offset),
		Headers:   map[string]any{},
	}

	if msg.TopicPartition.Topic != nil {
		converted.Topic = *msg.TopicPartition.Topic
	}

	for _, header := range msg.Headers {
		converted.Headers[header.Key] = header.Value
	}

	return converted
}

func (c *Consumer) consumerReadMessage(ctx context.Context, client *ckafka.Consumer) (*ckafka.Message, error) {
	timeout := confluentPollTimeout(ctx)
	if timeout == 0 {
		return nil, consumerContextError(consumerContextCause(ctx))
	}

	var absTimeout time.Time
	var timeoutMs int

	if timeout > 0 {
		absTimeout = time.Now().Add(timeout)
		timeoutMs = int(timeout.Milliseconds())
	} else {
		timeoutMs = int(timeout.Milliseconds())
	}

	for {
		event, err := c.poll(ctx, client, timeoutMs)
		if err != nil {
			return nil, err
		}

		message, ok := event.(*ckafka.Message)
		if ok {
			return message, nil
		}

		if timeout > 0 {
			// Calculate remaining time
			timeoutMs = int(max(0, time.Until(absTimeout).Milliseconds()))
		}

		if timeoutMs == 0 && event == nil {
			return nil, ckafka.NewError(ckafka.ErrTimedOut, "", false)
		}
	}
}

func (c *Consumer) poll(
	ctx context.Context,
	client *ckafka.Consumer,
	timeoutMs int,
) (ckafka.Event, error) {
	event := client.Poll(timeoutMs)

	switch e := event.(type) {
	case *ckafka.Message:
		if e.TopicPartition.Error != nil {
			return event, e.TopicPartition.Error
		}
		return event, nil
	case ckafka.OAuthBearerTokenRefresh:
		err := refreshOAuthToken(ctx, c.saslContext, client)
		if err != nil {
			return event, err
		}
	case ckafka.Error:
		return nil, e
	default:
		// Ignore other event types
	}
	return event, nil
}
