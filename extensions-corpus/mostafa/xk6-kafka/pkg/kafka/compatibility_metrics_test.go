package kafka

import (
	"context"
	"testing"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProducerClassReportsLegacyMetricsOnConfluentPath(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	mockCluster, err := ckafka.NewMockCluster(3)
	require.NoError(t, err)
	defer mockCluster.Close()

	topicName := "producer-compat-metrics"
	require.NoError(t, mockCluster.CreateTopic(topicName, 1, 1))

	producer := test.module.producerClass(sobek.ConstructorCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"brokers": []string{mockCluster.BootstrapServers()},
				"topic":   topicName,
			}),
		},
	})
	require.NotNil(t, producer)

	produce := producer.Get("produce").Export().(func(sobek.FunctionCall) sobek.Value)
	result := produce(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"messages": []map[string]any{
					{
						"key": test.module.serialize(&Container{
							Data:       "key",
							SchemaType: String,
						}),
						"value": test.module.serialize(&Container{
							Data:       "value",
							SchemaType: String,
						}),
					},
				},
			}),
		},
	}).Export()
	assert.Nil(t, result)

	closeProducer := producer.Get("close").Export().(func(sobek.FunctionCall) sobek.Value)
	assert.Nil(t, closeProducer(sobek.FunctionCall{}).Export())

	metricsValues := test.getMetricValues()
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.WriterWrites.Name])
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.WriterMessages.Name])
	assert.Greater(t, metricsValues[test.module.metrics.WriterBytes.Name], 0.0)
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterErrors.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterAsync.Name])
}

func TestConsumerClassReportsLegacyMetricsOnConfluentPath(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	mockCluster, err := ckafka.NewMockCluster(3)
	require.NoError(t, err)
	defer mockCluster.Close()

	topicName := "consumer-compat-metrics"
	require.NoError(t, mockCluster.CreateTopic(topicName, 1, 1))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	consumer := test.module.consumerClass(sobek.ConstructorCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"brokers": []string{mockCluster.BootstrapServers()},
				"topic":   topicName,
				"maxWait": "5s",
			}),
		},
	})
	require.NotNil(t, consumer)

	producer, err := NewProducerFromWriterConfig(&WriterConfig{
		Brokers: []string{mockCluster.BootstrapServers()},
		Topic:   topicName,
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, producer.Close())
	}()

	require.NoError(t, producer.Produce(ctx, []Message{{
		Key:   []byte("key"),
		Value: []byte("value"),
	}}))
	require.NoError(t, producer.Flush(ctx))

	consume := consumer.Get("consume").Export().(func(sobek.FunctionCall) sobek.Value)
	messages := consume(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"maxMessages": 1,
			}),
		},
	}).Export().([]map[string]any)
	require.Len(t, messages, 1)

	closeConsumer := consumer.Get("close").Export().(func(sobek.FunctionCall) sobek.Value)
	assert.Nil(t, closeConsumer(sobek.FunctionCall{}).Export())

	metricsValues := test.getMetricValues()
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.ReaderDials.Name])
	assert.GreaterOrEqual(t, metricsValues[test.module.metrics.ReaderFetches.Name], 1.0)
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.ReaderMessages.Name])
	assert.Greater(t, metricsValues[test.module.metrics.ReaderBytes.Name], 0.0)
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderErrors.Name])
}

func TestConsumerClassReturnsPartialMessagesOnTimeout(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	mockCluster, err := ckafka.NewMockCluster(3)
	require.NoError(t, err)
	defer mockCluster.Close()

	topicName := "consumer-timeout-partial"
	require.NoError(t, mockCluster.CreateTopic(topicName, 1, 1))

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	consumer := test.module.consumerClass(sobek.ConstructorCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"brokers": []string{mockCluster.BootstrapServers()},
				"topic":   topicName,
				"maxWait": "1s",
			}),
		},
	})
	require.NotNil(t, consumer)

	producer, err := NewProducerFromWriterConfig(&WriterConfig{
		Brokers: []string{mockCluster.BootstrapServers()},
		Topic:   topicName,
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, producer.Close())
	}()

	require.NoError(t, producer.Produce(ctx, []Message{{
		Value: []byte("value"),
	}}))
	require.NoError(t, producer.Flush(ctx))

	consume := consumer.Get("consume").Export().(func(sobek.FunctionCall) sobek.Value)
	messages := consume(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"maxMessages":   2,
				"expectTimeout": true,
			}),
		},
	}).Export().([]map[string]any)
	require.Len(t, messages, 1)

	closeConsumer := consumer.Get("close").Export().(func(sobek.FunctionCall) sobek.Value)
	assert.Nil(t, closeConsumer(sobek.FunctionCall{}).Export())

	metricsValues := test.getMetricValues()
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.ReaderMessages.Name])
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.ReaderTimeouts.Name])
}
