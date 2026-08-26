package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSerializeAvro serializes a JSON object into Avro binary.
func TestSerializeAvro(t *testing.T) {
	avroSerde := &AvroSerde{}
	expected := []byte{0x0a, 0x76, 0x61, 0x6c, 0x75, 0x65}
	schema := &Schema{
		ID: 2,
		Schema: `{
			"type":"record",
			"name":"Schema",
			"namespace":"io.confluent.kafka.avro",
			"fields":[{"name":"key","type":"string"}]}`,
		Version: 1,
		Subject: "avro-schema",
	}
	actual, err := avroSerde.Serialize(map[string]any{"key": "value"}, schema)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// TestSerializeAvroFailsOnInvalidDataType tests the serialization of a JSON object
// into Avro binary and fails on invalid data type.
func TestSerializeAvroFailsOnInvalidDataType(t *testing.T) {
	avroSerde := &AvroSerde{}
	schema := &Schema{
		ID: 2,
		Schema: `{
			"type":"record",
			"name":"Schema",
			"namespace":"io.confluent.kafka.avro",
			"fields":[{"name":"key","type":"string"}]}`,
		Version: 1,
		Subject: "avro-schema",
	}
	actual, err := avroSerde.Serialize(`{"key":"value"}`, schema)
	assert.Nil(t, actual)
	assert.NotNil(t, err)
	assert.Equal(t, ErrInvalidDataType, err)
}

// TestSerializeAvroFailsOnValidation tests the serialization of a JSON object
// into Avro binary and fails on schema validation error.
func TestSerializeAvroFailsOnValidation(t *testing.T) {
	avroSerde := &AvroSerde{}
	schema := &Schema{
		ID: 2,
		Schema: `{
			"type":"record",
			"name":"Schema",
			"namespace":"io.confluent.kafka.avro",
			"fields":[{"name":"key","type":"string"}]}`,
		Version: 1,
		Subject: "avro-schema",
	}
	actual, err := avroSerde.Serialize(map[string]any{"value": "key"}, schema)
	assert.Nil(t, actual)
	assert.NotNil(t, err)
	assert.Equal(t, "Failed to encode data into binary", err.Message)
	assert.Equal(t, failedToEncodeToBinary, err.Code)
}

// TestDeserializeAvro tests the deserialization of a JSON object from Avro binary.
func TestDeserializeAvro(t *testing.T) {
	avroSerde := &AvroSerde{}
	schema := &Schema{
		ID: 2,
		Schema: `{
			"type":"record",
			"name":"Schema",
			"namespace":"io.confluent.kafka.avro",
			"fields":[{"name":"key","type":"string"}]}`,
		Version: 1,
		Subject: "avro-schema",
	}
	expected := map[string]any{"key": "value"}
	actual, err := avroSerde.Deserialize([]byte{0x0a, 0x76, 0x61, 0x6c, 0x75, 0x65}, schema)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// TestDeserializeFailsOnDecodingBinary tests the deserialization of a JSON object
// from Avro binary and fails on decoding the received binary (invalid data).
func TestDeserializeFailsOnDecodingBinary(t *testing.T) {
	avroSerde := &AvroSerde{}
	schema := &Schema{
		ID: 2,
		Schema: `{
			"type":"record",
			"name":"Schema",
			"namespace":"io.confluent.kafka.avro",
			"fields":[{"name":"key","type":"string"}]}`,
		Version: 1,
		Subject: "avro-schema",
	}
	actual, err := avroSerde.Deserialize([]byte{0x76, 0x61, 0x6c, 0x75, 0x65}, schema)
	assert.Nil(t, actual)
	assert.NotNil(t, err)
	assert.Equal(t, "Failed to decode data", err.Message)
	assert.Equal(t, failedToDecodeFromBinary, err.Code)
}
