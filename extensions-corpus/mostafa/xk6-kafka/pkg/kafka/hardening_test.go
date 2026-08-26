package kafka

import (
	"encoding/binary"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireGoErrorMessage(t *testing.T, fn func(), expected string) {
	t.Helper()

	defer func() {
		t.Helper()

		err := recover()
		require.NotNil(t, err)

		errObj, ok := err.(*sobek.Object)
		require.True(t, ok)
		assert.Equal(t, GoErrorPrefix+expected, errObj.ToString().String())
	}()

	fn()
}

func TestWriterClassRejectsNonObjectConfig(t *testing.T) {
	test := getTestModuleInstance(t)

	requireGoErrorMessage(t, func() {
		test.module.writerClass(sobek.ConstructorCall{
			Arguments: []sobek.Value{test.rt.ToValue("invalid")},
		})
	}, "Invalid writer config, OriginalError: expected object, got string")
}

func TestProducerClassRejectsNonObjectConfig(t *testing.T) {
	test := getTestModuleInstance(t)

	requireGoErrorMessage(t, func() {
		test.module.producerClass(sobek.ConstructorCall{
			Arguments: []sobek.Value{test.rt.ToValue("invalid")},
		})
	}, "Invalid writer config, OriginalError: expected object, got string")
}

func TestReaderClassRejectsNonObjectConfig(t *testing.T) {
	test := getTestModuleInstance(t)

	requireGoErrorMessage(t, func() {
		test.module.readerClass(sobek.ConstructorCall{
			Arguments: []sobek.Value{test.rt.ToValue("invalid")},
		})
	}, "Invalid reader config, OriginalError: expected object, got string")
}

func TestConsumerClassRejectsNonObjectConfig(t *testing.T) {
	test := getTestModuleInstance(t)

	requireGoErrorMessage(t, func() {
		test.module.consumerClass(sobek.ConstructorCall{
			Arguments: []sobek.Value{test.rt.ToValue("invalid")},
		})
	}, "Invalid reader config, OriginalError: expected object, got string")
}

func TestConnectionClassRejectsMissingAddress(t *testing.T) {
	test := getTestModuleInstance(t)

	requireGoErrorMessage(t, func() {
		test.module.connectionClass(sobek.ConstructorCall{
			Arguments: []sobek.Value{test.rt.ToValue(map[string]any{})},
		})
	}, "Invalid connection config, OriginalError: address must not be empty")
}

func TestConnectionDeleteTopicRejectsNonStringTopic(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	admin := test.module.connectionClass(sobek.ConstructorCall{
		Arguments: []sobek.Value{test.rt.ToValue(map[string]any{
			"address": "localhost:9092",
		})},
	})
	require.NotNil(t, admin)

	deleteTopic := admin.Get("deleteTopic").Export().(func(sobek.FunctionCall) sobek.Value)
	requireGoErrorMessage(t, func() {
		deleteTopic(sobek.FunctionCall{
			Arguments: []sobek.Value{test.rt.ToValue(42)},
		})
	}, "Invalid topic config, OriginalError: topic must not be empty")
}

func TestAdminClientClassRejectsMissingAddress(t *testing.T) {
	test := getTestModuleInstance(t)

	requireGoErrorMessage(t, func() {
		test.module.adminClientClass(sobek.ConstructorCall{
			Arguments: []sobek.Value{test.rt.ToValue(map[string]any{})},
		})
	}, "Invalid connection config, OriginalError: address must not be empty")
}

func TestWriterClassRejectsBalancerOnConfluentCompatibilityPath(t *testing.T) {
	test := getTestModuleInstance(t)

	requireGoErrorMessage(t, func() {
		test.module.writerClass(sobek.ConstructorCall{
			Arguments: []sobek.Value{test.rt.ToValue(map[string]any{
				"brokers":  []string{"localhost:9092"},
				"topic":    "test-topic",
				"balancer": balancerRoundRobin,
			})},
		})
	}, "Writer balancer configuration is not supported on the Confluent compatibility path.")
}

func TestSchemaRegistryClientClassRejectsMissingURL(t *testing.T) {
	test := getTestModuleInstance(t)

	requireGoErrorMessage(t, func() {
		test.module.schemaRegistryClientClass(sobek.ConstructorCall{
			Arguments: []sobek.Value{test.rt.ToValue(map[string]any{})},
		})
	}, "Invalid schema registry config, OriginalError: url must not be empty")
}

func TestSerializeRejectsMissingMetadata(t *testing.T) {
	test := getTestModuleInstance(t)

	requireGoErrorMessage(t, func() {
		test.module.serialize(nil)
	}, "serialize metadata is required")
}

func TestDeserializeRejectsMissingMetadata(t *testing.T) {
	test := getTestModuleInstance(t)

	requireGoErrorMessage(t, func() {
		test.module.deserialize(nil)
	}, "deserialize metadata is required")
}

func TestSerializeSupportsProtobufBytesFormat(t *testing.T) {
	test := getTestModuleInstance(t)

	test.moveToVUCode()

	rawPayload := []byte{10, 5, 'v', 'a', 'l', 'u', 'e'}
	serialized := test.module.serialize(&Container{
		Data: rawPayload,
		Schema: &Schema{
			ID:          1,
			Schema:      `syntax = "proto3"; message Value { string field = 1; }`,
			Subject:     "test-subject",
			MessageName: "Value",
		},
		SchemaType:     Protobuf,
		ProtobufFormat: "bytes",
	})

	require.NotNil(t, serialized)
	assert.Greater(t, len(serialized), MagicPrefixSize)
}

func TestDeserializeSupportsProtobufBytesFormat(t *testing.T) {
	test := getTestModuleInstance(t)

	test.moveToVUCode()

	rawPayload := []byte{10, 5, 'v', 'a', 'l', 'u', 'e'}
	serialized := test.module.serialize(&Container{
		Data: rawPayload,
		Schema: &Schema{
			ID:          1,
			Schema:      `syntax = "proto3"; message Value { string field = 1; }`,
			Subject:     "test-subject",
			MessageName: "Value",
		},
		SchemaType:     Protobuf,
		ProtobufFormat: "bytes",
	})

	deserialized := test.module.deserialize(&Container{
		Data: serialized,
		Schema: &Schema{
			ID:          1,
			Schema:      `syntax = "proto3"; message Value { string field = 1; }`,
			Subject:     "test-subject",
			MessageName: "Value",
		},
		SchemaType:     Protobuf,
		ProtobufFormat: "bytes",
	})

	decoded, ok := deserialized.([]byte)
	require.True(t, ok)
	assert.Equal(t, rawPayload, decoded)
}

func TestEncodeWireFormatRejectsOutOfRangeSchemaID(t *testing.T) {
	test := getTestModuleInstance(t)

	requireGoErrorMessage(t, func() {
		test.module.encodeWireFormat([]byte("value"), -1)
	}, "Invalid schema id -1: must be within uint32 range")
}

func TestSerializeWithRegistryUsesScopedCache(t *testing.T) {
	test := getTestModuleInstance(t)
	avroType := Avro
	const localSchemaID = 7

	globalSchema := &Schema{
		ID:            99,
		Schema:        avroSchemaForSRTests,
		SchemaType:    &avroType,
		Version:       1,
		Subject:       "shared-subject",
		EnableCaching: true,
	}
	localSchema := &Schema{
		ID:            localSchemaID,
		Schema:        avroSchemaForSRTests,
		SchemaType:    &avroType,
		Version:       1,
		Subject:       "shared-subject",
		EnableCaching: true,
	}

	test.module.schemaCache["shared-subject"] = globalSchema
	registry := &schemaRegistryState{
		cache: map[string]*Schema{
			"shared-subject": localSchema,
		},
	}

	serialized := test.module.serializeWithRegistry(&Container{
		Data: map[string]any{"field": "value"},
		Schema: &Schema{
			Subject:       "shared-subject",
			EnableCaching: true,
		},
		SchemaType: Avro,
	}, registry)

	require.GreaterOrEqual(t, len(serialized), MagicPrefixSize)
	assert.EqualValues(t, localSchemaID, binary.BigEndian.Uint32(serialized[1:5]))
}
