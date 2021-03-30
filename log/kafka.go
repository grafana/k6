package log

// https://github.com/gfremex/logrus-kafka-hook

import (
	"context"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/sirupsen/logrus"
	"log"
	"strings"
	"time"
)

type KafkaHook struct {
	// Id of the hook
	id string

	// Log levels allowed
	levels []logrus.Level

	labels [][2]string

	fallbackLogger logrus.FieldLogger

	// sarama.AsyncProducer
	producer sarama.AsyncProducer

	// Log entry formatter
	formatter logrus.Formatter

	topics []string

	brokers []string
}

func KafkaFromConfigLine(ctx context.Context, fallbackLogger logrus.FieldLogger, line string) (*KafkaHook, error) {
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Producer.RequiredAcks = sarama.WaitForLocal // Only wait for the leader to ack
	kafkaConfig.Producer.Compression = sarama.CompressionNone
	kafkaConfig.Producer.Flush.Frequency = 500 * time.Millisecond // Flush batches every 500ms

	hook := KafkaHook{
		levels:         logrus.AllLevels,
		formatter:      &logrus.JSONFormatter{},
		fallbackLogger: fallbackLogger,
	}
	err := hook.parseArgs(line)
	if err != nil {
		return nil, err
	}

	hook.producer, err = sarama.NewAsyncProducer(hook.brokers, kafkaConfig)
	if err != nil {
		return nil, err
	}

	// We will just log to STDOUT if we're not able to produce messages.
	// Note: messages will only be returned here after all retry attempts are exhausted.
	go func() {
		for err := range hook.producer.Errors() {
			log.Printf("Failed to send log entry to kafka: %v\n", err)
		}
	}()

	return &hook, nil
}

func (hook *KafkaHook) parseArgs(line string) error {
	tokens, err := tokenize(line)
	if err != nil {
		return fmt.Errorf("error while parsing loki configuration %w", err)
	}

	for _, token := range tokens {
		key := token.key
		value := token.value

		var err error
		switch key {
		case "kafka":
			hook.brokers = strings.Split(value, ",")
		case "topics":
			hook.topics = strings.Split(value, ",")
		case "format":
			switch value {
			case "json":
				hook.formatter = &logrus.JSONFormatter{}
			case "gelf":
				hook.formatter = new(GelfFormatter)
			}
		case "level":
			hook.levels, err = getLevels(value)
			if err != nil {
				return err
			}
		default:
			if strings.HasPrefix(key, "label.") {
				labelKey := strings.TrimPrefix(key, "label.")
				hook.labels = append(hook.labels, [2]string{labelKey, value})
			}
			continue
		}
	}

	return nil
}

func (hook *KafkaHook) Id() string {
	return hook.id
}

func (hook *KafkaHook) Levels() []logrus.Level {
	return hook.levels
}

func (hook *KafkaHook) Fire(entry *logrus.Entry) error {
	// Get field time
	t, _ := entry.Data["time"].(time.Time)

	// Convert it to bytes
	b, err := t.MarshalBinary()

	if err != nil {
		return err
	}

	// Format before writing
	for _, label := range hook.labels {
		entry.Data[label[0]] = label[1]
	}
	b, err = hook.formatter.Format(entry)

	if err != nil {
		return err
	}

	value := sarama.ByteEncoder(b)

	for _, topic := range hook.topics {
		hook.producer.Input() <- &sarama.ProducerMessage{
			Key:   nil,
			Topic: topic,
			Value: value,
		}
	}

	return nil
}
