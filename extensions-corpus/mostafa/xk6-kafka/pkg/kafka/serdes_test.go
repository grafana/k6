package kafka

import (
	"testing"
	"unsafe"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSerdes tests serialization and deserialization of messages with different schemas.
func TestSerdes(t *testing.T) {
	test := getTestModuleInstance(t)

	test.createTopic()
	writer := test.newWriter()
	defer func() {
		_ = writer.Close()
	}()
	reader := test.newReader()
	defer func() {
		_ = reader.Close()
	}()

	// Switch to VU code.
	test.moveToVUCode()

	containers := []*Container{
		{
			Data:       "string",
			SchemaType: String,
		},
		{
			Data:       []byte("byte array"),
			SchemaType: Bytes,
		},
		{
			Data:       []byte{62, 79, 74, 65, 20, 61, 72, 72, 61, 79}, // byte array
			SchemaType: Bytes,
		},
		{
			Data: map[string]any{
				"string": "some-string",
				"number": 1.1,
				"bool":   true,
				"null":   nil,
				"array":  []any{1.0, 2.0, 3.0},
				"object": map[string]any{
					"string": "string-value",
					"number": 1.1,
					"bool":   true,
					"null":   nil,
					"array":  []any{1.0, 2.0, 3.0},
				},
			},
			SchemaType: Json,
		},
		{
			Data: map[string]any{"key": "value"},
			Schema: &Schema{
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
			},
			SchemaType: Json,
		},
		{
			Data: map[string]any{"key": "value"},
			Schema: &Schema{
				ID: 2,
				Schema: `{
					"type":"record",
					"name":"Schema",
					"namespace":"io.confluent.kafka.avro",
					"fields":[{"name":"key","type":"string"}]}`,
				Version: 1,
				Subject: "avro-schema",
			},
			SchemaType: Avro,
		},
	}

	for _, container := range containers {
		// Test with a schema registry, which fails and manually (de)serializes the data.
		serialized := test.module.serialize(container)
		assert.NotNil(t, serialized)
		// 4 bytes for magic byte, 1 byte for schema ID, and the rest is the data.
		assert.GreaterOrEqual(t, len(serialized), 5)

		// Send data to Kafka.
		test.module.produceWithProducer(writer, &ProduceConfig{
			Messages: []Message{
				{
					Value: serialized,
				},
			},
		})

		// Read data from Kafka.
		messages := test.module.consumeWithConsumer(reader, &ConsumeConfig{
			Limit: 1,
		})
		assert.Equal(t, 1, len(messages))

		if value, ok := messages[0]["value"].(map[string]any); ok {
			// Deserialize the key or value (removes the magic bytes).
			deserialized := test.module.deserialize(&Container{
				Data:       value,
				Schema:     container.Schema,
				SchemaType: container.SchemaType,
			})
			assert.Equal(t, container.Data, deserialized)
		}
	}
}

type TestDataContainer struct {
	container *Container
	err       *Xk6KafkaError
}

func TestSerializeFails(t *testing.T) {
	test := getTestModuleInstance(t)

	testDataContainer := []TestDataContainer{
		{
			container: &Container{
				Data:       "string",
				SchemaType: "invalid-schema-type",
			},
			err: ErrUnknownSerdesType,
		},
		{
			container: &Container{
				Data:       1.1,
				SchemaType: String,
			},
			err: ErrInvalidDataType,
		},
		{
			container: &Container{
				Data:       []any{"test"},
				SchemaType: Bytes,
			},
			err: ErrFailedTypeCast,
		},
		{
			container: &Container{
				Data:       "test",
				SchemaType: Bytes,
			},
			err: ErrInvalidDataType,
		},
		{
			container: &Container{
				Data:       map[string]any{"key": unsafe.Pointer(nil)}, // #nosec G103
				SchemaType: Json,
			},
			err: ErrInvalidDataType,
		},
		{
			container: &Container{
				Data:       "test",
				SchemaType: Json,
			},
			err: ErrInvalidDataType,
		},
		{
			container: &Container{
				Data: map[string]any{"key": "value"},
				Schema: &Schema{
					ID:      1,
					Schema:  `{`, // Invalid JSONSchema.
					Version: 1,
					Subject: "json-schema",
				},
				SchemaType: Json,
			},
			err: ErrInvalidSchema,
		},
		{
			container: &Container{
				Data:       `{"key": "value"}`,
				SchemaType: Avro,
			},
			err: ErrInvalidDataType,
		},
		{
			container: &Container{
				Data: map[string]any{"unknown": "value"},
				Schema: &Schema{
					ID: 2,
					Schema: `{
						"type":"record",
						"name":"Schema",
						"namespace":"io.confluent.kafka.avro",
						"fields":[{"name":"key","type":"string"}]}`,
					Version: 1,
					Subject: "avro-schema",
				},
				SchemaType: Avro,
			},
			err: NewXk6KafkaError(failedToEncodeToBinary, "Failed to encode data into binary", ErrAvroMissingRequiredField),
		},
		{
			container: &Container{
				Data: map[string]any{"key": unsafe.Pointer(nil)}, // #nosec G103
				Schema: &Schema{
					ID: 2,
					Schema: `{
						"type":"record",
						"name":"Schema",
						"namespace":"io.confluent.kafka.avro",
						"fields":[{"name":"key","type":"string"}]}`,
					Version: 1,
					Subject: "avro-schema",
				},
				SchemaType: Avro,
			},
			err: ErrInvalidDataType,
		},
	}

	for _, testData := range testDataContainer {
		t.Run("serialize fails", func(t *testing.T) {
			defer func(t *testing.T) {
				t.Helper()

				err := recover()
				assert.Equal(t,
					err.(*sobek.Object).ToString().String(),
					GoErrorPrefix+testData.err.Error())
			}(t)

			err := test.module.serialize(testData.container)
			assert.Equal(t, err, testData.err)
		})
	}
}

func TestSerialize_AvroNestedUnionWithLogicalTypeIssue376(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	schema := &Schema{
		ID: 376,
		Schema: `{
			"type": "record",
			"name": "Set",
			"namespace": "com.example",
			"fields": [
				{
					"name": "document",
					"type": [
						"null",
						{
							"type": "record",
							"name": "Document",
							"fields": [
								{
									"name": "documentId",
									"type": {
										"type": "string",
										"avro.java.string": "String"
									}
								},
								{
									"name": "documentType",
									"type": [
										"null",
										{
											"type": "string",
											"avro.java.string": "String"
										}
									],
									"default": null
								},
								{
									"name": "documentValidTo",
									"type": [
										"null",
										{
											"type": "int",
											"logicalType": "date"
										}
									],
									"default": null
								}
							]
						}
					],
					"default": null
				}
			]
		}`,
		Version: 1,
		Subject: "issue-376",
	}

	testCases := []struct {
		name          string
		documentType  any
		documentValid any
	}{
		{
			name:          "wrapped int.date",
			documentType:  map[string]any{"string": "OP"},
			documentValid: map[string]any{"int.date": float64(20474)},
		},
		{
			name:          "wrapped int",
			documentType:  map[string]any{"string": "OP"},
			documentValid: map[string]any{"int": float64(20475)},
		},
		{
			name:          "direct scalar",
			documentType:  "OP",
			documentValid: float64(20476),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			container := &Container{
				Data: map[string]any{
					"document": map[string]any{
						"com.example.Document": map[string]any{
							"documentId":      "BR9876543",
							"documentType":    testCase.documentType,
							"documentValidTo": testCase.documentValid,
						},
					},
				},
				Schema:     schema,
				SchemaType: Avro,
			}

			assert.NotPanics(t, func() {
				serialized := test.module.serialize(container)
				require.NotNil(t, serialized)
				require.GreaterOrEqual(t, len(serialized), 5)
			})
		})
	}
}

func TestSerializeDeserialize_ProtobufObjectMode(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	schema := &Schema{
		ID:          9,
		Schema:      `syntax = "proto3"; message User { string name = 1; int32 age = 2; }`,
		Subject:     "test-user",
		MessageName: "User",
	}

	serialized := test.module.serialize(&Container{
		Data: map[string]any{
			"name": "Mostafa",
			"age":  33.0,
		},
		Schema:     schema,
		SchemaType: Protobuf,
	})
	require.NotNil(t, serialized)

	deserialized := test.module.deserialize(&Container{
		Data:       serialized,
		Schema:     schema,
		SchemaType: Protobuf,
	})

	decoded, ok := deserialized.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Mostafa", decoded["name"])
	assert.Equal(t, float64(33), decoded["age"])
}

func TestSerializeDeserialize_ProtobufStandaloneDependencies(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	schema := &Schema{
		ID: 0,
		Schema: `
syntax = "proto3";
import "common.proto";
message UserWrapper {
  common.User user = 1;
}`,
		Dependencies: map[string]string{
			"common.proto": `
syntax = "proto3";
package common;
message User {
  string name = 1;
}
`,
		},
		Subject:     "test-user-wrapper",
		MessageName: "UserWrapper",
	}

	serialized := test.module.serialize(&Container{
		Data: map[string]any{
			"user": map[string]any{
				"name": "Mostafa",
			},
		},
		Schema:     schema,
		SchemaType: Protobuf,
	})
	require.NotNil(t, serialized)

	deserialized := test.module.deserialize(&Container{
		Data:       serialized,
		Schema:     schema,
		SchemaType: Protobuf,
	})

	decoded, ok := deserialized.(map[string]any)
	require.True(t, ok)
	user, ok := decoded["user"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Mostafa", user["name"])
}

func TestDeserializeWithoutSchemaBranches(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	t.Run("bytes to string", func(t *testing.T) {
		out := test.module.deserialize(&Container{
			Data:       []byte("hello"),
			SchemaType: String,
		})
		assert.Equal(t, "hello", out)
	})

	t.Run("bytes passthrough", func(t *testing.T) {
		out := test.module.deserialize(&Container{
			Data:       []byte{1, 2, 3},
			SchemaType: Bytes,
		})
		assert.Equal(t, []byte{1, 2, 3}, out)
	})

	t.Run("json bytes to map", func(t *testing.T) {
		out := test.module.deserialize(&Container{
			Data:       []byte(`{"key":"value"}`),
			SchemaType: Json,
		})
		asMap, ok := out.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "value", asMap["key"])
	})

	t.Run("non-json bytes remain bytes", func(t *testing.T) {
		out := test.module.deserialize(&Container{
			Data:       []byte("not-json"),
			SchemaType: Json,
		})
		assert.Equal(t, []byte("not-json"), out)
	})

	t.Run("plain string returns bytes", func(t *testing.T) {
		out := test.module.deserialize(&Container{
			Data:       "plain",
			SchemaType: String,
		})
		assert.Equal(t, []byte("plain"), out)
	})

	t.Run("base64 string decodes via serde", func(t *testing.T) {
		out := test.module.deserialize(&Container{
			Data:       "aGVsbG8=",
			SchemaType: String,
		})
		assert.Equal(t, "hello", out)
	})

	t.Run("protobuf without schema errors", func(t *testing.T) {
		requireGoErrorMessage(t, func() {
			test.module.deserialize(&Container{
				Data:       []byte("abc"),
				SchemaType: Protobuf,
			})
		}, "schema metadata is required")
	})
}
