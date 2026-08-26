package kafka

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSerializeJSON tests the serialization of a JSON object.
func TestSerializeJSON(t *testing.T) {
	jsonSerde := &JSONSerde{}
	expected := []byte(`{"key":"value"}`)
	actual, err := jsonSerde.Serialize(map[string]any{"key": "value"}, nil)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// TestSerializeJSONWithSchema tests the serialization of a JSON object with a schema.
func TestSerializeJSONWithSchema(t *testing.T) {
	jsonSerde := &JSONSerde{}
	expected := []byte(`{"key":"value"}`)
	schema := &Schema{
		ID: 1,
		Schema: `{
			"$schema": "http://json-schema.org/draft-04/schema#",
			"type": "object",
			"properties": {
				"key": {"type": "string"}
			},
			"required": ["key"]
		}`,
		Version: 1,
		Subject: "json-schema",
	}
	actual, err := jsonSerde.Serialize(map[string]any{"key": "value"}, schema)
	assert.Nil(t, err)
	assert.Equal(t, expected, actual)
}

// TestSerializeJSONFailsOnInvalidSchema tests the serialization of a JSON object
// and fails on an invalid schema.
func TestSerializeJSONFailsOnInvalidSchema(t *testing.T) {
	jsonSerde := &JSONSerde{}
	schema := &Schema{
		ID:      1,
		Schema:  `{`,
		Version: 1,
		Subject: "json-schema",
	}
	actual, err := jsonSerde.Serialize(map[string]any{"value": "key"}, schema)
	assert.Nil(t, actual)
	assert.NotNil(t, err)
	assert.Equal(t, ErrInvalidSchema, err)
}

// TestSerializeJSONFailsOnValidation tests the serialization of a JSON object
// and fails on schema validation errors.
func TestSerializeJSONFailsOnValidation(t *testing.T) {
	jsonSerde := &JSONSerde{}
	schema := &Schema{
		ID: 1,
		Schema: `{
			"$schema": "http://json-schema.org/draft-04/schema#",
			"type": "object",
			"properties": {
				"key": {"type": "string"}
			},
			"required": ["key"]
		}`,
		Version: 1,
		Subject: "json-schema",
	}
	actual, err := jsonSerde.Serialize(map[string]any{"value": "key"}, schema)
	assert.Nil(t, actual)
	assert.NotNil(t, err)
	assert.Equal(t, "Failed to validate JSON against schema", err.Message)
	assert.Equal(t, failedValidateJSON, err.Code)
}

// TestSerializeJSONFailsOnBadType tests the serialization of a JSON object
// and fails on nested invalid data type.
func TestSerializeJSONFailsOnBadType(t *testing.T) {
	jsonSerde := &JSONSerde{}
	actual, err := jsonSerde.Serialize(map[string]any{"key": unsafe.Pointer(nil)}, nil) // #nosec G103
	assert.Nil(t, actual)
	assert.NotNil(t, err)
	assert.Equal(t, ErrInvalidDataType, err)
}

// TestSerializeJSONFailsOnInvalidDataType tests the serialization of a JSON object
// and fails on invalid data type.
func TestSerializeJSONFailsOnInvalidDataType(t *testing.T) {
	jsonSerde := &JSONSerde{}
	actual, err := jsonSerde.Serialize(1, nil)
	assert.Nil(t, actual)
	assert.NotNil(t, err)
	assert.Equal(t, ErrInvalidDataType, err)
}

func TestDeserializeJSONWithoutSchema(t *testing.T) {
	jsonSerde := &JSONSerde{}
	out, err := jsonSerde.Deserialize([]byte(`{"a":1,"b":"x"}`), nil)
	assert.Nil(t, err)
	m, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), m["a"])
	assert.Equal(t, "x", m["b"])
}

func TestDeserializeJSONInvalidBytes(t *testing.T) {
	jsonSerde := &JSONSerde{}
	out, err := jsonSerde.Deserialize([]byte(`{`), nil)
	assert.Nil(t, out)
	require.NotNil(t, err)
	assert.Equal(t, failedUnmarshalJSON, err.Code)
}

func TestDeserializeJSONValidationFailure(t *testing.T) {
	jsonSerde := &JSONSerde{}
	schema := &Schema{
		ID: 1,
		Schema: `{
			"$schema": "http://json-schema.org/draft-04/schema#",
			"type": "object",
			"properties": {"key": {"type": "string"}},
			"required": ["key"]
		}`,
		Version: 1,
		Subject: "s",
	}
	out, err := jsonSerde.Deserialize([]byte(`{"key":1}`), schema)
	assert.Nil(t, out)
	require.NotNil(t, err)
	assert.Equal(t, failedDecodeJSONFromBinary, err.Code)
}
