package kafka

import (
	"testing"
	"time"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProduce tests the produce function.
// nolint: funlen
func TestProduce(t *testing.T) {
	test := getTestModuleInstance(t)
	test.createTopic()

	assert.NotPanics(t, func() {
		writer, err := NewProducerFromWriterConfig(&WriterConfig{
			Brokers: []string{"localhost:9092"},
			Topic:   test.topicName,
		})
		require.NoError(t, err)
		assert.NotNil(t, writer)
		defer func() {
			_ = writer.Close()
		}()

		// Produce a message in the init context.
		assert.Panics(t, func() {
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
					},
					{
						Key: test.module.serialize(&Container{
							Data:       "key2",
							SchemaType: String,
						}),
						Value: test.module.serialize(&Container{
							Data:       "value2",
							SchemaType: String,
						}),
					},
				},
			})
		})

		test.moveToVUCode()

		// Produce two messages in the VU function.
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
					},
					{
						Key: test.module.serialize(&Container{
							Data:       "key2",
							SchemaType: String,
						}),
						Value: test.module.serialize(&Container{
							Data:       "value2",
							SchemaType: String,
						}),
					},
				},
			})
		})
	})

	// Check if two message were produced.
	metricsValues := test.getCounterMetricsValues()
	assert.Equal(t, 2.0, metricsValues[test.module.metrics.WriterWrites.Name])
	assert.Equal(t, 2.0, metricsValues[test.module.metrics.WriterMessages.Name])
	assert.Equal(t, 66.0, metricsValues[test.module.metrics.WriterBytes.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterErrors.Name])
	assert.GreaterOrEqual(t, 1.0, metricsValues[test.module.metrics.WriterWriteTime.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterWaitTime.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterRetries.Name])
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.WriterBatchSize.Name])
	assert.Equal(t, 33.0, metricsValues[test.module.metrics.WriterBatchBytes.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterMaxAttempts.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterMaxBatchSize.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterBatchTimeout.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterReadTimeout.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterWriteTimeout.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterRequiredAcks.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterAsync.Name])
}

// TestProduceWithoutKey tests the produce function without a key.
func TestProduceWithoutKey(t *testing.T) {
	test := getTestModuleInstance(t)
	test.createTopic()

	assert.NotPanics(t, func() {
		writer, err := NewProducerFromWriterConfig(&WriterConfig{
			Brokers: []string{"localhost:9092"},
		})
		require.NoError(t, err)
		assert.NotNil(t, writer)
		defer func() {
			_ = writer.Close()
		}()

		test.moveToVUCode()

		// Produce two messages in the VU function.
		assert.NotPanics(t, func() {
			test.module.produceWithProducer(writer, &ProduceConfig{
				Messages: []Message{
					{
						Value: test.module.serialize(&Container{
							Data:       "value1",
							SchemaType: String,
						}),
						Topic:  test.topicName,
						Offset: 0,
						Time:   time.Now(),
					},
					{
						Value: test.module.serialize(&Container{
							Data:       "value2",
							SchemaType: String,
						}),
						Topic: test.topicName,
					},
				},
			})
		})
	})

	// Check if two message were produced.
	metricsValues := test.getCounterMetricsValues()
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterErrors.Name])
	// Notice the smaller size because the key is not present (64 -> 56).
	assert.Equal(t, 58.0, metricsValues[test.module.metrics.WriterBytes.Name])
	assert.Equal(t, 2.0, metricsValues[test.module.metrics.WriterMessages.Name])
	assert.Equal(t, 2.0, metricsValues[test.module.metrics.WriterWrites.Name])
}

// TestProducerContextCancelled tests the produce function with a cancelled context.
func TestProducerContextCancelled(t *testing.T) {
	test := getTestModuleInstance(t)
	test.createTopic()

	assert.NotPanics(t, func() {
		writer, err := NewProducerFromWriterConfig(&WriterConfig{
			Brokers: []string{"localhost:9092"},
			Topic:   test.topicName,
		})
		require.NoError(t, err)
		assert.NotNil(t, writer)
		defer func() {
			_ = writer.Close()
		}()

		test.moveToVUCode()

		// This will cancel the context, so the produce will fail.
		test.cancelContext()

		// Produce two messages in the VU function.
		assert.Panics(t, func() {
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
					},
					{
						Key: test.module.serialize(&Container{
							Data:       "key2",
							SchemaType: String,
						}),
						Value: test.module.serialize(&Container{
							Data:       "value2",
							SchemaType: String,
						}),
					},
				},
			})
		})
	})

	// Cancelled context is immediately reflected in metrics, because
	// we need the context object to update the metrics.
	metricsValues := test.getCounterMetricsValues()
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterErrors.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterBytes.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterMessages.Name])
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterWrites.Name])
}

// TestProduceJSON tests the produce function with a JSON value.
func TestProduceJSON(t *testing.T) {
	test := getTestModuleInstance(t)
	test.createTopic()

	assert.NotPanics(t, func() {
		writer, err := NewProducerFromWriterConfig(&WriterConfig{
			Brokers: []string{"localhost:9092"},
			Topic:   test.topicName,
		})
		require.NoError(t, err)
		assert.NotNil(t, writer)
		defer func() {
			_ = writer.Close()
		}()

		test.moveToVUCode()

		// Produce a message in the VU function.
		assert.NotPanics(t, func() {
			test.module.produceWithProducer(writer, &ProduceConfig{
				Messages: []Message{
					{
						Value: test.module.serialize(&Container{
							Data:       map[string]any{"field": "value"},
							SchemaType: Json,
						}),
					},
				},
			})
		})
	})

	// Check if one message was produced.
	metricsValues := test.getCounterMetricsValues()
	assert.Equal(t, 0.0, metricsValues[test.module.metrics.WriterErrors.Name])
	assert.Equal(t, 40, int(metricsValues[test.module.metrics.WriterBytes.Name]))
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.WriterMessages.Name])
	assert.Equal(t, 1.0, metricsValues[test.module.metrics.WriterWrites.Name])
}

// TestWriterClass tests the writer class.
func TestWriterClass(t *testing.T) {
	test := getTestModuleInstance(t)

	test.moveToVUCode()
	test.createTopic()

	assert.NotPanics(t, func() {
		writer := test.module.writerClass(sobek.ConstructorCall{
			Arguments: []sobek.Value{
				test.module.vu.Runtime().ToValue(
					map[string]any{
						"brokers": []string{"localhost:9092"},
						"topic":   test.topicName,
					},
				),
			},
		})
		assert.NotNil(t, writer)

		// Produce a message.
		produce := writer.Get("produce").Export().(func(sobek.FunctionCall) sobek.Value)
		result := produce(sobek.FunctionCall{
			Arguments: []sobek.Value{
				test.module.vu.Runtime().ToValue(
					map[string]any{
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
					},
				),
			},
		}).Export()
		assert.Nil(t, result)

		// Close the writer.
		closeVal := writer.Get("close").Export()
		closeFunc, ok := closeVal.(func(sobek.FunctionCall) sobek.Value)
		assert.True(t, ok)
		assert.NotNil(t, closeFunc)
		result = closeFunc(sobek.FunctionCall{}).Export()
		assert.Nil(t, result)

		reader := test.newReader()
		defer func() {
			_ = reader.Close()
		}()

		messages := test.module.consumeWithConsumer(reader, &ConsumeConfig{Limit: 1, ExpectTimeout: true})
		assert.Len(t, messages, 1)
	})
}

func TestWriterConfigParse(t *testing.T) {
	t.Run("basic config without balancer", func(t *testing.T) {
		var writerConfig WriterConfig
		m := map[string]any{
			"autoCreateTopic": true,
			"connectLogger":   true,
			"maxAttempts":     10,
			"batchSize":       100,
			"batchBytes":      1048576,
			"requiredAcks":    1,
			"topic":           "test-topic",
			"compression":     codecGzip,
			"brokers":         []string{"localhost:9092"},
			"batchTimeout":    time.Second * 10,
			"readTimeout":     time.Second * 30,
			"writeTimeout":    time.Second * 30,
			"sasl": map[string]any{
				"username":   "test-user",
				"password":   "test-password",
				"algorithm":  "PLAIN",
				"awsProfile": "default",
			},
			"tls": map[string]any{
				"enableTLS":             true,
				"insecureSkipTLSVerify": false,
				"minVersion":            "TLS12",
				"clientCertPem":         "cert-pem-content",
				"clientKeyPem":          "key-pem-content",
				"serverCaPem":           "ca-pem-content",
			},
		}
		require.NoError(t, writerConfig.Parse(m, sobek.New()))
		assert.Equal(t, WriterConfig{
			AutoCreateTopic: true,
			ConnectLogger:   true,
			MaxAttempts:     10,
			BatchSize:       100,
			BatchBytes:      1048576,
			RequiredAcks:    1,
			Topic:           "test-topic",
			Compression:     codecGzip,
			Brokers:         []string{"localhost:9092"},
			BatchTimeout:    time.Second * 10,
			ReadTimeout:     time.Second * 30,
			WriteTimeout:    time.Second * 30,
			SASL: SASLConfig{
				Username:   "test-user",
				Password:   "test-password",
				Algorithm:  "PLAIN",
				AWSProfile: "default",
			},
			TLS: TLSConfig{
				EnableTLS:             true,
				InsecureSkipTLSVerify: false,
				MinVersion:            "TLS12",
				ClientCertPem:         "cert-pem-content",
				ClientKeyPem:          "key-pem-content",
				ServerCaPem:           "ca-pem-content",
			},
		}, writerConfig)
	})

	t.Run("config with string balancer", func(t *testing.T) {
		var writerConfig WriterConfig
		m := map[string]any{
			"brokers":  []string{"localhost:9092"},
			"topic":    "test-topic",
			"balancer": balancerRoundRobin,
		}
		require.NoError(t, writerConfig.Parse(m, sobek.New()))
		assert.Equal(t, balancerRoundRobin, writerConfig.Balancer)
		assert.Nil(t, writerConfig.BalancerFunc)
	})

	t.Run("config with balancer function", func(t *testing.T) {
		runtime := sobek.New()
		// Create a JavaScript function that returns partition 5
		_, err := runtime.RunString(`
			function customBalancer(key, partitions) {
				return 5;
			}
		`)
		require.NoError(t, err)

		customBalancerFunc := runtime.Get("customBalancer")
		require.NotNil(t, customBalancerFunc)

		var writerConfig WriterConfig
		m := map[string]any{
			"brokers":  []string{"localhost:9092"},
			"topic":    "test-topic",
			"balancer": customBalancerFunc,
		}
		require.NoError(t, writerConfig.Parse(m, runtime))
		assert.Empty(t, writerConfig.Balancer)
		assert.NotNil(t, writerConfig.BalancerFunc)

		// Test that the balancer function works correctly
		testKey := []byte("test-key")
		partition := writerConfig.BalancerFunc(testKey, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
		assert.Equal(t, 5, partition)
	})

	t.Run("config without balancer uses default", func(t *testing.T) {
		var writerConfig WriterConfig
		m := map[string]any{
			"brokers": []string{"localhost:9092"},
			"topic":   "test-topic",
		}
		require.NoError(t, writerConfig.Parse(m, sobek.New()))
		assert.Empty(t, writerConfig.Balancer)
		assert.Nil(t, writerConfig.BalancerFunc)
	})

	t.Run("unknown balancer string", func(t *testing.T) {
		var writerConfig WriterConfig
		err := writerConfig.Parse(map[string]any{
			"brokers":  []string{"localhost:9092"},
			"topic":    "test-topic",
			"balancer": "unknown_balancer_name",
		}, sobek.New())
		require.Error(t, err)
		assert.ErrorIs(t, err, errUnknownBalancer)
	})

	t.Run("nil writer config receiver", func(t *testing.T) {
		var writerConfig *WriterConfig
		err := writerConfig.Parse(map[string]any{
			"brokers": []string{"localhost:9092"},
			"topic":   "test-topic",
		}, sobek.New())
		require.Error(t, err)
	})
}
