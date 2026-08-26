package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type ProducerStats struct {
	Pending int
}

const producerFlushPollTimeoutMs = 100

type Producer struct {
	client       *ckafka.Producer
	config       ckafka.ConfigMap
	defaultTopic string
	waitForAck   bool
	closeOnce    sync.Once
	closeErr     error
	doneChan     chan struct{}
}

func NewProducerFromWriterConfig(writerConfig *WriterConfig) (*Producer, error) {
	config, err := writerConfigToConfluentConfigMap(writerConfig)
	if err != nil {
		return nil, err
	}

	saslContext, err := NewSaslContext(writerConfig.SASL, writerConfig.Brokers, SASLContextOpts{})
	if err != nil {
		return nil, err
	}

	client, err := ckafka.NewProducer(&config)
	if err != nil {
		return nil, NewXk6KafkaError(failedCreateProducer, "Failed to create producer.", err)
	}

	defaultTopic := ""
	if writerConfig != nil {
		defaultTopic = writerConfig.Topic
	}

	doneChan := make(chan struct{})
	go handleProducerClientEvents(saslContext, client, client.Events(), doneChan)

	return &Producer{
		client:       client,
		config:       cloneConfluentConfigMap(config),
		defaultTopic: defaultTopic,
		waitForAck:   producerWaitsForAck(writerConfig),
		doneChan:     doneChan,
	}, nil
}

func (p *Producer) Produce(ctx context.Context, msgs []Message) error {
	if p == nil || p.client == nil {
		return newMissingConfigError("producer")
	}
	if len(msgs) == 0 {
		return nil
	}
	ctx = ensureContext(ctx)
	if err := ctx.Err(); err != nil {
		return NewXk6KafkaError(writerError, "Producer context cancelled.", err)
	}

	var deliveryChan chan ckafka.Event
	if p.waitForAck {
		deliveryChan = make(chan ckafka.Event, len(msgs))
	}

	for _, msg := range msgs {
		if err := ctx.Err(); err != nil {
			return NewXk6KafkaError(writerError, "Producer context cancelled.", err)
		}

		topic := msg.Topic
		if topic == "" {
			topic = p.defaultTopic
		}
		if topic == "" {
			return newInvalidConfigError("producer message", errTopicMustNotBeEmpty)
		}

		kafkaMsg := &ckafka.Message{
			TopicPartition: ckafka.TopicPartition{
				Topic:     &topic,
				Partition: ckafka.PartitionAny,
			},
			Key:       msg.Key,
			Value:     msg.Value,
			Timestamp: msg.Time,
			Headers:   confluentHeaders(msg.Headers),
		}

		if err := p.produceMessage(ctx, kafkaMsg, deliveryChan); err != nil {
			return NewXk6KafkaError(writerError, "Failed to produce message.", err)
		}
	}

	if !p.waitForAck {
		return nil
	}

	for pending := len(msgs); pending > 0; pending-- {
		select {
		case <-ctx.Done():
			return NewXk6KafkaError(writerError, "Producer context cancelled.", ctx.Err())
		case event := <-deliveryChan:
			switch produced := event.(type) {
			case *ckafka.Message:
				if produced.TopicPartition.Error != nil {
					return NewXk6KafkaError(writerError, "Failed to deliver produced message.", produced.TopicPartition.Error)
				}
			case ckafka.Error:
				return NewXk6KafkaError(writerError, "Producer reported an asynchronous error.", produced)
			}
		}
	}

	return nil
}

func (p *Producer) Flush(ctx context.Context) error {
	if p == nil || p.client == nil {
		return newMissingConfigError("producer")
	}
	ctx = ensureContext(ctx)

	for {
		select {
		case <-ctx.Done():
			return NewXk6KafkaError(failedFlushProducer, "Producer flush cancelled.", ctx.Err())
		default:
		}

		if remaining := p.client.Flush(producerFlushPollTimeoutMs); remaining == 0 {
			return nil
		}
	}
}

func (p *Producer) Close() error {
	if p == nil || p.client == nil {
		return nil
	}

	p.closeOnce.Do(func() {
		client := p.client
		if client == nil {
			return
		}

		if !p.waitForAck {
			ctx, cancel := context.WithTimeout(context.Background(), defaultConfluentTimeout)
			p.closeErr = p.Flush(ctx)
			cancel()
		}
		p.client = nil
		close(p.doneChan)
		client.Close()
	})

	return p.closeErr
}

func (p *Producer) Stats() ProducerStats {
	if p == nil || p.client == nil {
		return ProducerStats{}
	}

	return ProducerStats{Pending: p.client.Len()}
}

func (p *Producer) produceMessage(
	ctx context.Context,
	msg *ckafka.Message,
	deliveryChan chan ckafka.Event,
) error {
	for {
		err := p.client.Produce(msg, deliveryChan)
		if err == nil {
			return nil
		}

		var kafkaErr ckafka.Error
		if !errors.As(err, &kafkaErr) || kafkaErr.Code() != ckafka.ErrQueueFull {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}

func confluentHeaders(headers map[string]any) []ckafka.Header {
	if len(headers) == 0 {
		return nil
	}

	kafkaHeaders := make([]ckafka.Header, 0, len(headers))
	for key, value := range headers {
		kafkaHeaders = append(kafkaHeaders, ckafka.Header{
			Key:   key,
			Value: fmt.Appendf(nil, "%v", value),
		})
	}

	return kafkaHeaders
}

func producerWaitsForAck(writerConfig *WriterConfig) bool {
	return writerConfig == nil || writerConfig.RequiredAcks != 0
}

func handleProducerClientEvents(
	saslContext SASLContext,
	client *ckafka.Producer,
	eventChan chan ckafka.Event,
	doneChan <-chan struct{},
) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-doneChan:
			cancel()
		case <-ctx.Done():
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}

			switch event.(type) {
			case ckafka.OAuthBearerTokenRefresh:
				_ = refreshOAuthToken(ctx, saslContext, client)
			default:
				// Ignore other event types
			}
		}
	}
}
