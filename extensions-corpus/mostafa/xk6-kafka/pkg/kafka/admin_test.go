package kafka

import (
	"context"
	"testing"

	ckafka "github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminClientDeleteTopicEmptyName(t *testing.T) {
	t.Parallel()
	mockCluster, err := ckafka.NewMockCluster(1)
	require.NoError(t, err)
	defer mockCluster.Close()

	ctx := context.Background()
	admin, err := NewAdminClientFromConnectionConfig(&ConnectionConfig{
		Address: mockCluster.BootstrapServers(),
	})
	require.NoError(t, err)
	defer func() { _ = admin.Close() }()

	err = admin.DeleteTopic(ctx, "")
	require.Error(t, err)
}

func TestAdminClientNilReceiver(t *testing.T) {
	t.Parallel()
	var a *AdminClient
	ctx := context.Background()

	_, err := a.ListTopics(ctx)
	require.Error(t, err)

	_, err = a.GetMetadata(ctx, "t")
	require.Error(t, err)

	err = a.CreateTopic(ctx, TopicConfig{Topic: "x"})
	require.Error(t, err)

	err = a.DeleteTopic(ctx, "x")
	require.Error(t, err)

	assert.NoError(t, a.Close())
}

func TestAdminClientTracksProducerForLifecycle(t *testing.T) {
	t.Parallel()
	mockCluster, err := ckafka.NewMockCluster(1)
	require.NoError(t, err)
	defer mockCluster.Close()

	admin, err := NewAdminClientFromConnectionConfig(&ConnectionConfig{
		Address: mockCluster.BootstrapServers(),
	})
	require.NoError(t, err)
	defer func() { _ = admin.Close() }()

	require.NotNil(t, admin.client)
	require.NotNil(t, admin.pClient)
	require.NotNil(t, admin.doneChan)
}

func TestAdminClientCloseIsIdempotentAndClearsClients(t *testing.T) {
	t.Parallel()
	mockCluster, err := ckafka.NewMockCluster(1)
	require.NoError(t, err)
	defer mockCluster.Close()

	admin, err := NewAdminClientFromConnectionConfig(&ConnectionConfig{
		Address: mockCluster.BootstrapServers(),
	})
	require.NoError(t, err)

	require.NoError(t, admin.Close())
	require.Nil(t, admin.client)
	require.Nil(t, admin.pClient)
	require.Nil(t, admin.doneChan)

	require.NoError(t, admin.Close())
}

func TestAdminClientOperationsFailAfterClose(t *testing.T) {
	t.Parallel()
	mockCluster, err := ckafka.NewMockCluster(1)
	require.NoError(t, err)
	defer mockCluster.Close()

	admin, err := NewAdminClientFromConnectionConfig(&ConnectionConfig{
		Address: mockCluster.BootstrapServers(),
	})
	require.NoError(t, err)

	require.NoError(t, admin.Close())

	_, err = admin.ListTopics(context.Background())
	require.Error(t, err)

	_, err = admin.GetMetadata(context.Background(), "any-topic")
	require.Error(t, err)

	err = admin.CreateTopic(context.Background(), TopicConfig{Topic: "any-topic"})
	require.Error(t, err)

	err = admin.DeleteTopic(context.Background(), "any-topic")
	require.Error(t, err)
}
