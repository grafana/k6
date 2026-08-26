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

func TestJSCompatibilityConstructorsWithMockCluster(t *testing.T) {
	mockCluster, err := ckafka.NewMockCluster(3)
	require.NoError(t, err)
	defer mockCluster.Close()

	test := getTestModuleInstance(t)
	test.moveToVUCode()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	test.vu.CtxField = ctx

	producerTopic := "compat-producer-topic"
	writerTopic := "compat-writer-topic"
	require.NoError(t, mockCluster.CreateTopic(producerTopic, 1, 1))
	require.NoError(t, mockCluster.CreateTopic(writerTopic, 1, 1))

	producer := test.module.producerClass(sobek.ConstructorCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"brokers": []string{mockCluster.BootstrapServers()},
				"topic":   producerTopic,
			}),
		},
	})
	require.NotNil(t, producer)

	produce := producer.Get("produce").Export().(func(sobek.FunctionCall) sobek.Value)
	assert.Nil(t, produce(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"messages": []map[string]any{{
					"key":   "producer-key",
					"value": "producer-value",
				}},
			}),
		},
	}).Export())

	writer := test.module.writerClass(sobek.ConstructorCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"brokers": []string{mockCluster.BootstrapServers()},
				"topic":   writerTopic,
			}),
		},
	})
	require.NotNil(t, writer)

	writerProduce := writer.Get("produce").Export().(func(sobek.FunctionCall) sobek.Value)
	assert.Nil(t, writerProduce(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"messages": []map[string]any{{
					"key":   "writer-key",
					"value": "writer-value",
				}},
			}),
		},
	}).Export())

	consumer := test.module.consumerClass(sobek.ConstructorCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"brokers":     []string{mockCluster.BootstrapServers()},
				"topic":       producerTopic,
				"startOffset": firstOffset,
			}),
		},
	})
	require.NotNil(t, consumer)

	consume := consumer.Get("consume").Export().(func(sobek.FunctionCall) sobek.Value)
	consumed := consume(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"maxMessages": 1,
			}),
		},
	}).Export().([]map[string]any)
	require.Len(t, consumed, 1)
	assert.Equal(t, producerTopic, consumed[0]["topic"])
	assert.Equal(t, []byte("producer-value"), consumed[0]["value"])

	reader := test.module.readerClass(sobek.ConstructorCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"brokers":     []string{mockCluster.BootstrapServers()},
				"topic":       writerTopic,
				"startOffset": firstOffset,
			}),
		},
	})
	require.NotNil(t, reader)

	readerConsume := reader.Get("consume").Export().(func(sobek.FunctionCall) sobek.Value)
	read := readerConsume(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"limit": 1,
			}),
		},
	}).Export().([]map[string]any)
	require.Len(t, read, 1)
	assert.Equal(t, writerTopic, read[0]["topic"])
	assert.Equal(t, []byte("writer-value"), read[0]["value"])

	adminClient := test.module.adminClientClass(sobek.ConstructorCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"brokers": []string{mockCluster.BootstrapServers()},
			}),
		},
	})
	require.NotNil(t, adminClient)

	listTopics := adminClient.Get("listTopics").Export().(func(sobek.FunctionCall) sobek.Value)
	topics := listTopics(sobek.FunctionCall{}).Export().([]map[string]any)
	require.NotEmpty(t, topics)
	var producerTopicInfo map[string]any
	for _, topicInfo := range topics {
		if topicInfo["topic"] == producerTopic {
			producerTopicInfo = topicInfo
			break
		}
	}
	require.NotNil(t, producerTopicInfo)
	assert.Equal(t, producerTopic, producerTopicInfo["topic"])
	assert.Equal(t, producerTopic, producerTopicInfo["Topic"])
	assert.Equal(t, 1, producerTopicInfo["partitions"])
	assert.Equal(t, 1, producerTopicInfo["Partitions"])

	getMetadata := adminClient.Get("getMetadata").Export().(func(sobek.FunctionCall) sobek.Value)
	metadata := getMetadata(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(producerTopic),
		},
	}).Export().(map[string]any)
	require.NotNil(t, metadata)
	assert.Equal(t, producerTopic, metadata["topic"])
	assert.Equal(t, producerTopic, metadata["Topic"])

	partitions, ok := metadata["partitions"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, partitions, 1)
	assert.Equal(t, partitions, metadata["Partitions"])
	assert.Equal(t, int32(0), partitions[0]["id"])
	assert.Equal(t, int32(0), partitions[0]["ID"])

	producerStats := producer.Get("stats").Export().(func(sobek.FunctionCall) sobek.Value)
	producerStatsValue := producerStats(sobek.FunctionCall{}).Export().(map[string]any)
	assert.Equal(t, producerStatsValue["pending"], producerStatsValue["Pending"])

	consumerStats := consumer.Get("stats").Export().(func(sobek.FunctionCall) sobek.Value)
	consumerStatsValue := consumerStats(sobek.FunctionCall{}).Export().(map[string]any)
	assert.Equal(t, consumerStatsValue["assignments"], consumerStatsValue["Assignments"])

	deleteTopic := adminClient.Get("deleteTopic").Export().(func(sobek.FunctionCall) sobek.Value)
	assert.Panics(t, func() {
		deleteTopic(sobek.FunctionCall{})
	})
	assert.Panics(t, func() {
		deleteTopic(sobek.FunctionCall{
			Arguments: []sobek.Value{test.rt.ToValue(123)},
		})
	})

	connection := test.module.connectionClass(sobek.ConstructorCall{
		Arguments: []sobek.Value{
			test.rt.ToValue(map[string]any{
				"address": mockCluster.BootstrapServers(),
			}),
		},
	})
	require.NotNil(t, connection)

	connectionListTopics := connection.Get("listTopics").Export().(func(sobek.FunctionCall) sobek.Value)
	connectionTopics := connectionListTopics(sobek.FunctionCall{}).Export().([]string)
	assert.Contains(t, connectionTopics, producerTopic)
	assert.Contains(t, connectionTopics, writerTopic)
}

func TestJSCompatibilityGroupIDAliasAndElementConstants(t *testing.T) {
	mockCluster, err := ckafka.NewMockCluster(3)
	require.NoError(t, err)
	defer mockCluster.Close()

	test := getTestModuleInstance(t)
	test.moveToVUCode()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	test.vu.CtxField = ctx

	groupTopic := "compat-group-id-alias-topic"
	require.NoError(t, mockCluster.CreateTopic(groupTopic, 1, 1))

	assert.NotPanics(t, func() {
		consumer := test.module.consumerClass(sobek.ConstructorCall{
			Arguments: []sobek.Value{
				test.rt.ToValue(map[string]any{
					"brokers":     []string{mockCluster.BootstrapServers()},
					"groupID":     "compat-group",
					"groupTopics": []string{groupTopic},
					"startOffset": firstOffset,
				}),
			},
		})
		require.NotNil(t, consumer)
		closeFn := consumer.Get("close").Export().(func(sobek.FunctionCall) sobek.Value)
		_ = closeFn(sobek.FunctionCall{})
	})

	assert.Equal(t, "key", test.module.exports.Get("KEY").Export())
	assert.Equal(t, "value", test.module.exports.Get("VALUE").Export())
}
