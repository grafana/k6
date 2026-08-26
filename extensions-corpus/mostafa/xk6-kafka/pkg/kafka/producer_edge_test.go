package kafka

import (
	"testing"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProducerProduceNilReceiver(t *testing.T) {
	t.Parallel()
	var p *Producer
	err := p.Produce(t.Context(), []Message{{Topic: "t", Value: []byte("v")}})
	require.Error(t, err)
}

func TestProducerProduceNilClient(t *testing.T) {
	t.Parallel()
	p := &Producer{client: nil, defaultTopic: "t"}
	err := p.Produce(t.Context(), []Message{{Value: []byte("v")}})
	require.Error(t, err)
}

func TestProducerProduceEmptyMessages(t *testing.T) {
	t.Parallel()
	mockCluster, err := ckafka.NewMockCluster(1)
	require.NoError(t, err)
	defer mockCluster.Close()

	p, err := NewProducerFromWriterConfig(&WriterConfig{
		Brokers: []string{mockCluster.BootstrapServers()},
		Topic:   "t",
	})
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	require.NoError(t, p.Produce(t.Context(), nil))
	require.NoError(t, p.Produce(t.Context(), []Message{}))
}

func TestProducerProduceMissingTopic(t *testing.T) {
	t.Parallel()
	mockCluster, err := ckafka.NewMockCluster(1)
	require.NoError(t, err)
	defer mockCluster.Close()

	p, err := NewProducerFromWriterConfig(&WriterConfig{
		Brokers: []string{mockCluster.BootstrapServers()},
		Topic:   "",
	})
	require.NoError(t, err)
	defer func() { _ = p.Close() }()

	err = p.Produce(t.Context(), []Message{{Value: []byte("x")}})
	require.Error(t, err)
}

func TestProducerFlushNilReceiver(t *testing.T) {
	t.Parallel()
	var p *Producer
	err := p.Flush(t.Context())
	require.Error(t, err)
}

func TestProducerStatsNilReceiver(t *testing.T) {
	t.Parallel()
	var p *Producer
	assert.Equal(t, ProducerStats{}, p.Stats())
}
