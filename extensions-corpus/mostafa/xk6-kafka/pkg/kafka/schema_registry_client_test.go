package kafka

import (
	"testing"

	cschemaregistry "github.com/confluentinc/confluent-kafka-go/v2/schemaregistry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConfluentSchemaRegistryAdapterNilClient(t *testing.T) {
	t.Parallel()
	assert.Nil(t, newConfluentSchemaRegistryAdapter(nil, true))
	assert.Nil(t, newConfluentSchemaRegistryAdapter(nil, false))
}

func TestConfluentSchemaRegistryAdapterClearsCachesWhenDisabled(t *testing.T) {
	client, err := cschemaregistry.NewClient(
		cschemaregistry.NewConfig("mock://schema-registry-cache-disabled"),
	)
	require.NoError(t, err)

	adapter := newConfluentSchemaRegistryAdapter(client, false)
	require.NotNil(t, adapter)

	_, err = adapter.CreateSchema("test-subject", avroSchemaForSRTests, Avro)
	require.NoError(t, err)

	first, err := adapter.GetLatestSchema("test-subject")
	require.NoError(t, err)
	assert.Equal(t, 1, first.Version())

	_, err = adapter.CreateSchema(
		"test-subject",
		`{"type":"record","name":"Schema","fields":[{"name":"field","type":"int"}]}`,
		Avro,
	)
	require.NoError(t, err)

	second, err := adapter.GetLatestSchema("test-subject")
	require.NoError(t, err)
	assert.Equal(t, 2, second.Version())

	require.NoError(t, adapter.Close())
}

func TestConfluentSchemaRegistryAdapterCreateSchemaReturnsRegisteredVersion(t *testing.T) {
	client, err := cschemaregistry.NewClient(
		cschemaregistry.NewConfig("mock://schema-registry-create-version"),
	)
	require.NoError(t, err)

	adapter := newConfluentSchemaRegistryAdapter(client, true)
	require.NotNil(t, adapter)

	created, err := adapter.CreateSchema("test-subject", avroSchemaForSRTests, Avro)
	require.NoError(t, err)
	require.NotNil(t, created)

	assert.Equal(t, 1, created.Version())
	assert.Equal(t, avroSchemaForSRTests, created.Schema())
	assert.Equal(t, Avro, *created.SchemaType())
}
