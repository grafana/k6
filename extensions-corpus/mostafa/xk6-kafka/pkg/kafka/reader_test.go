package kafka

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConsumerMaxWaitExceeded tests the consume function when no messages are sent.
// The reader should not hang.
func TestConsumerMaxWaitExceeded(t *testing.T) {
	test := getTestModuleInstance(t)
	test.createTopic()
	writer := test.newWriter()
	defer func() {
		_ = writer.Close()
	}()

	// Create a reader to consume messages.
	reader, err := NewConsumerFromReaderConfig(&ReaderConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   test.topicName,
		MaxWait: Duration{250 * time.Millisecond},
	})
	require.NoError(t, err)
	assert.NotNil(t, reader)
	defer func() {
		_ = reader.Close()
	}()

	// Switch to VU code.
	test.moveToVUCode()

	test.module.produceWithProducer(writer, &ProduceConfig{
		Messages: []Message{
			{
				Value: test.module.serialize(&Container{
					Data:       "value1",
					SchemaType: String,
				}),
			},
		},
	})

	// Allow receiving messages consumed before MaxWait
	messages := test.module.consumeWithConsumer(reader, &ConsumeConfig{Limit: 2, ExpectTimeout: true})
	assert.Equal(t, 1, len(messages))

	// Check that message was consumed.
	metricsValues := test.getCounterMetricsValues()
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.ReaderDials.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderErrors.Name])
	assert.Equal(t, 6.0, metricsValues[test.module.metrics.ReaderBytes.Name])
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.ReaderMessages.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderRebalances.Name])

	// Fail on deadline in the default case
	assert.Panics(t, func() {
		test.module.consumeWithConsumer(reader, &ConsumeConfig{Limit: 2})
	})
}

// TestConsumerPanicsOnClose tests the consume function when the reader is being
// closed. Closing the reader while reading is considered unexpected and should
// panic.
func TestConsumerPanicsOnClose(t *testing.T) {
	test := getTestModuleInstance(t)
	test.createTopic()

	// Create a reader to consume messages.
	reader, err := NewConsumerFromReaderConfig(&ReaderConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   test.topicName,
		MaxWait: Duration{time.Second * 3},
	})
	require.NoError(t, err)
	defer func() {
		_ = reader.Close()
	}()

	go func() {
		// Wait for a bit so consumption starts.
		time.Sleep(250 * time.Millisecond)
		// Close reader to cause consume to panic.
		_ = reader.Close()
	}()

	// Switch to VU code.
	test.moveToVUCode()

	// Consume a message in the VU function.
	assert.Panics(t, func() {
		test.module.consumeWithConsumer(reader, &ConsumeConfig{Limit: 1})
	})
}

// TestConsume tests the consume function.
// nolint: funlen
func TestConsume(t *testing.T) {
	test := getTestModuleInstance(t)
	test.createTopic()
	writer := test.newWriter()
	defer func() {
		_ = writer.Close()
	}()

	assert.True(t, test.topicExists())

	// Create a reader to consume messages.
	assert.NotPanics(t, func() {
		reader, err := NewConsumerFromReaderConfig(&ReaderConfig{
			Brokers: []string{"localhost:9092"},
			Topic:   test.topicName,
		})
		require.NoError(t, err)
		assert.NotNil(t, reader)
		defer func() {
			_ = reader.Close()
		}()

		// Switch to VU code.
		test.moveToVUCode()

		// Produce a message in the VU function.
		assert.NotPanics(t, func() {
			test.module.produceWithProducer(writer, &ProduceConfig{
				Messages: []Message{
					{
						Key: test.module.serialize(&Container{
							Data:       "key1",
							SchemaType: String,
						}),
						Value: test.module.serialize(&Container{
							Data:       "value1",
							SchemaType: String,
						}),
						Offset: 0,
					},
				},
			})
		})

		// Consume a message in the VU function.
		assert.NotPanics(t, func() {
			messages := test.module.consumeWithConsumer(reader, &ConsumeConfig{Limit: 1})
			assert.Equal(t, 1, len(messages))

			result := test.module.deserialize(&Container{
				Data:       messages[0]["key"],
				SchemaType: String,
			})

			if key, ok := result.([]byte); ok {
				assert.Equal(t, "key1", string(key))
			}

			result = test.module.deserialize(&Container{
				Data:       messages[0]["value"],
				SchemaType: String,
			})
			if value, ok := result.([]byte); ok {
				assert.Equal(t, "value1", string(value))
			}
		})
	})

	// Check if one message was consumed.
	metricsValues := test.getCounterMetricsValues()
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.ReaderDials.Name])
	assert.Equal(t, 2.0, metricsValues[test.module.metrics.ReaderFetches.Name])
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.ReaderMessages.Name])
	assert.Equal(t, 10.0, metricsValues[test.module.metrics.ReaderBytes.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderRebalances.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderTimeouts.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderErrors.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderDialTime.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderReadTime.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderWaitTime.Name])
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.ReaderFetchSize.Name])
	assert.Equal(t, 10.0, metricsValues[test.module.metrics.ReaderFetchBytes.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderOffset.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderLag.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderMinBytes.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderMaxBytes.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderMaxWait.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderQueueLength.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderQueueCapacity.Name])
}

// TestConsumeWithoutKey tests the consume function without a key.
func TestConsumeWithoutKey(t *testing.T) {
	test := getTestModuleInstance(t)
	test.createTopic()
	writer := test.newWriter()
	defer func() {
		_ = writer.Close()
	}()

	reader, err := NewConsumerFromReaderConfig(&ReaderConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   test.topicName,
	})
	require.NoError(t, err)
	assert.NotNil(t, reader)
	defer func() {
		_ = reader.Close()
	}()

	// Create a reader to consume messages.
	assert.NotPanics(t, func() {
		// Switch to VU code.
		test.moveToVUCode()

		// Produce a message in the VU function.
		assert.NotPanics(t, func() {
			test.module.produceWithProducer(writer, &ProduceConfig{
				Messages: []Message{
					{
						Value: test.module.serialize(&Container{
							Data:       "value1",
							SchemaType: String,
						}),
					},
				},
			})
		})

		// Consume a message in the VU function.
		assert.NotPanics(t, func() {
			messages := test.module.consumeWithConsumer(reader, &ConsumeConfig{Limit: 1})
			assert.Equal(t, 1, len(messages))
			assert.NotContains(t, messages[0], "key")

			result := test.module.deserialize(&Container{
				Data:       messages[0]["value"],
				SchemaType: String,
			})
			if value, ok := result.([]byte); ok {
				assert.Equal(t, "value1", string(value))
			}
		})
	})

	// Check if one message was consumed.
	metricsValues := test.getCounterMetricsValues()
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.ReaderDials.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderErrors.Name])
	assert.Equal(t, 6.0, metricsValues[test.module.metrics.ReaderBytes.Name])
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.ReaderMessages.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderRebalances.Name])
}

// TestConsumerContextCancelled tests the consume function and fails on a cancelled context.
func TestConsumerContextCancelled(t *testing.T) {
	test := getTestModuleInstance(t)
	test.createTopic()
	writer := test.newWriter()
	defer func() {
		_ = writer.Close()
	}()

	reader, err := NewConsumerFromReaderConfig(&ReaderConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   test.topicName,
	})
	require.NoError(t, err)
	assert.NotNil(t, reader)
	defer func() {
		_ = reader.Close()
	}()

	// Create a reader to consume messages.
	assert.NotPanics(t, func() {
		// Switch to VU code.
		test.moveToVUCode()

		// Produce a message in the VU function.
		assert.NotPanics(t, func() {
			test.module.produceWithProducer(writer, &ProduceConfig{
				Messages: []Message{
					{
						Value: test.module.serialize(&Container{
							Data:       "value1",
							SchemaType: String,
						}),
						Offset: 2,
					},
				},
			})
		})

		test.cancelContext()

		// Consume a message in the VU function.
		assert.Panics(t, func() {
			test.module.consumeWithConsumer(reader, &ConsumeConfig{Limit: 1})
		})
	})

	// Check if no message was consumed.
	metricsValues := test.getCounterMetricsValues()
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderDials.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderErrors.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderBytes.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderMessages.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderRebalances.Name])
}

// TestConsumeJSON tests the consume function with a JSON value.
func TestConsumeJSON(t *testing.T) {
	test := getTestModuleInstance(t)
	test.createTopic()
	writer := test.newWriter()
	defer func() {
		_ = writer.Close()
	}()

	reader, err := NewConsumerFromReaderConfig(&ReaderConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   test.topicName,
	})
	require.NoError(t, err)
	assert.NotNil(t, reader)
	defer func() {
		_ = reader.Close()
	}()

	// Create a reader to consume messages.
	assert.NotPanics(t, func() {
		// Switch to VU code.
		test.moveToVUCode()

		serialized, jsonErr := json.Marshal(map[string]any{"field": "value"})
		assert.Nil(t, jsonErr)

		// Produce a message in the VU function.
		assert.NotPanics(t, func() {
			test.module.produceWithProducer(writer, &ProduceConfig{
				Messages: []Message{
					{
						Value: serialized,
					},
				},
			})
		})

		// Consume the message.
		assert.NotPanics(t, func() {
			messages := test.module.consumeWithConsumer(reader, &ConsumeConfig{Limit: 1})
			assert.Equal(t, 1, len(messages))

			result := test.module.deserialize(&Container{
				Data:       messages[0]["value"],
				SchemaType: Json,
			})
			if data, ok := result.(map[string]any); ok {
				assert.Equal(t, "value", data["field"])
			}
		})

		// Check if one message was consumed.
		metricsValues := test.getCounterMetricsValues()
		assert.Equal(t, 1.0, metricsValues[test.module.metrics.ReaderDials.Name])
		assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderErrors.Name])
		assert.Equal(t, 17.0, metricsValues[test.module.metrics.ReaderBytes.Name])
		assert.Equal(t, 1.0, metricsValues[test.module.metrics.ReaderMessages.Name])
		assert.Equal(t, 0.0, metricsValues[test.module.metrics.ReaderRebalances.Name])
	})
}

// TestReaderClass tests the reader class.
func TestReaderClass(t *testing.T) {
	test := getTestModuleInstance(t)

	test.moveToVUCode()
	test.createTopic()
	writer := test.newWriter()
	defer func() {
		_ = writer.Close()
	}()

	test.module.produceWithProducer(writer, &ProduceConfig{
		Messages: []Message{
			{
				Key: test.module.serialize(&Container{
					Data:       "key",
					SchemaType: String,
				}),
				Value: test.module.serialize(&Container{
					Data:       "value",
					SchemaType: String,
				}),
			},
		},
	})

	assert.NotPanics(t, func() {
		reader := test.module.readerClass(sobek.ConstructorCall{
			Arguments: []sobek.Value{
				test.module.vu.Runtime().ToValue(
					map[string]any{
						"brokers": []string{"localhost:9092"},
						"topic":   test.topicName,
						"maxWait": "3s",
					},
				),
			},
		})
		assert.NotNil(t, reader)
		thisVal := reader.Get("This").Export()
		this, ok := thisVal.(*Consumer)
		assert.True(t, ok)
		assert.NotNil(t, this)
		assert.Equal(t, "localhost:9092", this.config["bootstrap.servers"])
		assert.Equal(t, test.topicName, this.topic)
		assert.Equal(t, 3000, this.config["fetch.wait.max.ms"])

		consumeVal := reader.Get("consume").Export()
		consume, ok := consumeVal.(func(sobek.FunctionCall) sobek.Value)
		assert.True(t, ok)
		messages := consume(sobek.FunctionCall{
			Arguments: []sobek.Value{
				test.module.vu.Runtime().ToValue(
					map[string]any{
						"limit": 1,
					},
				),
			},
		}).Export().([]map[string]any)
		assert.Equal(t, 1, len(messages))
		deserializedKey := test.module.deserialize(&Container{
			Data:       messages[0]["key"],
			SchemaType: String,
		})
		assert.Equal(t, "key", deserializedKey)
		deserializedValue := test.module.deserialize(&Container{
			Data:       messages[0]["value"],
			SchemaType: String,
		})
		assert.Equal(t, "value", deserializedValue)

		// Close the reader.
		closeVal := reader.Get("close").Export()
		closeFunc, ok := closeVal.(func(sobek.FunctionCall) sobek.Value)
		assert.True(t, ok)
		assert.NotNil(t, closeFunc)
		result := closeFunc(sobek.FunctionCall{}).Export()
		assert.Nil(t, result)
	})
}
