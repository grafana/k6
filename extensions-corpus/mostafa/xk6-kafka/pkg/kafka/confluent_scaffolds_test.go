package kafka

import (
	"bytes"
	"context"
	"testing"
	"time"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriterConfigToConfluentConfigMap(t *testing.T) {
	config, err := writerConfigToConfluentConfigMap(&WriterConfig{
		Brokers:      []string{"localhost:9092", "localhost:9093"},
		RequiredAcks: -1,
		BatchSize:    10,
		BatchBytes:   2048,
		BatchTimeout: 250 * time.Millisecond,
		Compression:  codecGzip,
		SASL: SASLConfig{
			Algorithm: saslScramSha256,
			Username:  "user",
			Password:  "pass",
		},
		TLS: TLSConfig{
			EnableTLS:             true,
			InsecureSkipTLSVerify: true,
			ServerCaPem:           "ca",
			ClientCertPem:         "cert",
			ClientKeyPem:          "key",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "localhost:9092,localhost:9093", config["bootstrap.servers"])
	assert.Equal(t, "all", config["acks"])
	assert.Equal(t, 10, config["batch.num.messages"])
	assert.Equal(t, 2048, config["batch.size"])
	assert.Equal(t, 250, config["linger.ms"])
	assert.Equal(t, "gzip", config["compression.type"])
	assert.Equal(t, "SASL_SSL", config["security.protocol"])
	assert.Equal(t, "SCRAM-SHA-256", config["sasl.mechanism"])
	assert.Equal(t, "user", config["sasl.username"])
	assert.Equal(t, "pass", config["sasl.password"])
	assert.Equal(t, "ca", config["ssl.ca.pem"])
	assert.Equal(t, "cert", config["ssl.certificate.pem"])
	assert.Equal(t, "key", config["ssl.key.pem"])
	assert.Equal(t, false, config["enable.ssl.certificate.verification"])
}

func TestWriterConfigToConfluentConfigMapPreservesZeroRequiredAcks(t *testing.T) {
	config, err := writerConfigToConfluentConfigMap(&WriterConfig{
		Brokers: []string{"localhost:9092"},
	})
	require.NoError(t, err)
	assert.Equal(t, "0", config["acks"])
}

func TestWriterConfigToConfluentConfigMapDisablesDeliveryReportPayloads(t *testing.T) {
	config, err := writerConfigToConfluentConfigMap(&WriterConfig{
		Brokers: []string{"localhost:9092"},
	})
	require.NoError(t, err)
	assert.Equal(t, false, config["go.delivery.reports"])
	assert.Equal(t, "none", config["go.delivery.report.fields"])
}

func TestProducerWaitsForAck(t *testing.T) {
	assert.False(t, producerWaitsForAck(&WriterConfig{}))
	assert.True(t, producerWaitsForAck(&WriterConfig{RequiredAcks: 1}))
	assert.True(t, producerWaitsForAck(&WriterConfig{RequiredAcks: -1}))
	assert.True(t, producerWaitsForAck(nil))
}

func TestConfluentScaffoldsWithMockCluster(t *testing.T) {
	mockCluster, err := ckafka.NewMockCluster(3)
	require.NoError(t, err)
	defer mockCluster.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	adminClient, err := NewAdminClientFromConnectionConfig(&ConnectionConfig{
		Address: mockCluster.BootstrapServers(),
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, adminClient.Close())
	}()

	topicName := "confluent-scaffold-topic"
	// MockCluster topic creation is more reliable than Admin API create in this smoke path.
	require.NoError(t, mockCluster.CreateTopic(topicName, 1, 1))

	topics, err := adminClient.ListTopics(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, topics)

	var listedTopic *TopicInfo
	for _, topic := range topics {
		if topic.Topic == topicName {
			listedTopic = &topic
			break
		}
	}
	require.NotNil(t, listedTopic)
	assert.Equal(t, 1, listedTopic.Partitions)
	assert.NoError(t, listedTopic.Error)

	metadata, err := adminClient.GetMetadata(ctx, topicName)
	require.NoError(t, err)
	require.Equal(t, topicName, metadata.Topic)
	require.Len(t, metadata.Partitions, 1)

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
		Headers: map[string]any{
			"header": "value",
		},
	}}))
	require.NoError(t, producer.Flush(ctx))
	assert.GreaterOrEqual(t, producer.Stats().Pending, 0)

	consumer, err := NewConsumerFromReaderConfig(&ReaderConfig{
		Brokers:     []string{mockCluster.BootstrapServers()},
		GroupID:     "confluent-scaffold-group",
		GroupTopics: []string{topicName},
		StartOffset: firstOffset,
	})
	require.NoError(t, err)
	defer func() {
		require.NoError(t, consumer.Close())
	}()

	messages, err := consumer.Consume(ctx, 1)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, topicName, messages[0].Topic)
	assert.True(t, bytes.Equal([]byte("key"), messages[0].Key))
	assert.True(t, bytes.Equal([]byte("value"), messages[0].Value))
	assert.Equal(t, []byte("value"), messages[0].Headers["header"])

	position, err := consumer.Position(0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, position, int64(1))

	require.NoError(t, consumer.Seek(0, 0))

	require.NoError(t, consumer.CommitOffsets(ctx))
}
