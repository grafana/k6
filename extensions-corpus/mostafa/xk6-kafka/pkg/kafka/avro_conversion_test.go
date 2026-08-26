package kafka

import (
	"testing"
	"time"

	"github.com/hamba/avro/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testUserSchemaJSON = `{
		"type": "record",
		"name": "User",
		"namespace": "com.example",
		"fields": [
			{"name": "id", "type": "int"},
			{"name": "name", "type": "string"}
		]
	}`
	testDocumentLogicalDateSchemaJSON = `{
		"type": "record",
		"name": "Document",
		"namespace": "com.example",
		"fields": [
			{
				"name": "documentValidTo",
				"type": [
					"null",
					{
						"type": "int",
						"logicalType": "date"
					}
				]
			}
		]
	}`
)

func TestConvertPrimitiveType_Bytes(t *testing.T) {
	schema, err := avro.Parse(`{"type": "bytes"}`)
	require.NoError(t, err)

	tests := []struct {
		name    string
		data    any
		want    []byte
		wantErr bool
	}{
		{
			name:    "array of float64",
			data:    []any{float64(1), float64(2), float64(3)},
			want:    []byte{1, 2, 3},
			wantErr: false,
		},
		{
			name:    "array of int",
			data:    []any{1, 2, 3},
			want:    []byte{1, 2, 3},
			wantErr: false,
		},
		{
			name:    "array of int32",
			data:    []any{int32(1), int32(2), int32(3)},
			want:    []byte{1, 2, 3},
			wantErr: false,
		},
		{
			name:    "already bytes",
			data:    []byte{1, 2, 3},
			want:    []byte{1, 2, 3},
			wantErr: false,
		},
		{
			name:    "invalid type",
			data:    "not an array",
			want:    nil,
			wantErr: false, // Returns as-is
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := convertPrimitiveType(testCase.data, schema)
			if testCase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if testCase.want != nil {
					assert.Equal(t, testCase.want, got)
				}
			}
		})
	}
}

func TestConvertPrimitiveType_Int(t *testing.T) {
	schema, err := avro.Parse(`{"type": "int"}`)
	require.NoError(t, err)

	tests := []struct {
		name    string
		data    any
		want    int32
		wantErr bool
	}{
		{
			name:    "valid float64",
			data:    float64(42),
			want:    int32(42),
			wantErr: false,
		},
		{
			name:    "already int32",
			data:    int32(42),
			want:    int32(42),
			wantErr: false,
		},
		{
			name:    "non-integer float64",
			data:    float64(42.5),
			want:    0,
			wantErr: true,
		},
		{
			name:    "out of range",
			data:    float64(2147483648), // > int32 max
			want:    0,
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := convertPrimitiveType(testCase.data, schema)
			if testCase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.want, got)
			}
		})
	}
}

func TestConvertPrimitiveType_Long(t *testing.T) {
	schema, err := avro.Parse(`{"type": "long"}`)
	require.NoError(t, err)

	tests := []struct {
		name    string
		data    any
		want    int64
		wantErr bool
	}{
		{
			name:    "valid float64",
			data:    float64(42),
			want:    int64(42),
			wantErr: false,
		},
		{
			name:    "already int64",
			data:    int64(42),
			want:    int64(42),
			wantErr: false,
		},
		{
			name:    "non-integer float64",
			data:    float64(42.5),
			want:    0,
			wantErr: true,
		},
		{
			name:    "large valid number",
			data:    float64(9000000000000000000), // Large but safe for float64->int64
			want:    int64(9000000000000000000),
			wantErr: false,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := convertPrimitiveType(testCase.data, schema)
			if testCase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.want, got)
			}
		})
	}
}

func TestConvertPrimitiveType_Fixed(t *testing.T) {
	schema, err := avro.Parse(`{"type": "fixed", "name": "MD5", "size": 16}`)
	require.NoError(t, err)

	tests := []struct {
		name    string
		data    any
		wantErr bool
	}{
		{
			name: "array of float64",
			data: []any{
				float64(1), float64(2), float64(3), float64(4),
				float64(5), float64(6), float64(7), float64(8),
				float64(9), float64(10), float64(11), float64(12),
				float64(13), float64(14), float64(15), float64(16),
			},
			wantErr: false,
		},
		{
			name:    "array of int",
			data:    []any{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
			wantErr: false,
		},
		{
			name:    "wrong size - too few",
			data:    []any{float64(1), float64(2), float64(3)},
			wantErr: true,
		},
		{
			name: "wrong size - too many",
			data: []any{
				float64(1), float64(2), float64(3), float64(4),
				float64(5), float64(6), float64(7), float64(8),
				float64(9), float64(10), float64(11), float64(12),
				float64(13), float64(14), float64(15), float64(16),
				float64(17), float64(18),
			},
			wantErr: true,
		},
		{
			name:    "value out of byte range",
			data:    []any{float64(256)},
			wantErr: true,
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := convertPrimitiveType(testCase.data, schema)
			if testCase.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConvertUnionField_Null(t *testing.T) {
	schema, err := avro.Parse(`["null", "string"]`)
	require.NoError(t, err)
	unionSchema := schema.(*avro.UnionSchema)

	got, err := convertUnionField(nil, unionSchema)
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestConvertUnionField_Enum(t *testing.T) {
	enumSchema := `{
		"type": "enum",
		"name": "Status",
		"namespace": "com.example",
		"symbols": ["ACTIVE", "INACTIVE"]
	}`
	unionSchemaJSON := `["null", ` + enumSchema + `]`
	schema, err := avro.Parse(unionSchemaJSON)
	require.NoError(t, err)
	unionSchema := schema.(*avro.UnionSchema)

	got, err := convertUnionField("ACTIVE", unionSchema)
	assert.NoError(t, err)
	assert.Equal(t, map[string]any{"com.example.Status": "ACTIVE"}, got)
}

func TestConvertUnionField_Record(t *testing.T) {
	recordSchema := testUserSchemaJSON
	unionSchemaJSON := `["null", ` + recordSchema + `]`
	schema, err := avro.Parse(unionSchemaJSON)
	require.NoError(t, err)
	unionSchema := schema.(*avro.UnionSchema)

	// Test with unwrapped record
	data := map[string]any{
		"id":   float64(123),
		"name": "test",
	}
	got, err := convertUnionField(data, unionSchema)
	assert.NoError(t, err)
	assert.IsType(t, map[string]any{}, got)
	result := got.(map[string]any)
	assert.Len(t, result, 1)
	assert.Contains(t, result, "com.example.User")
	userData := result["com.example.User"].(map[string]any)
	assert.Equal(t, int32(123), userData["id"])
	assert.Equal(t, "test", userData["name"])
}

func TestConvertUnionField_RecordAlreadyWrapped(t *testing.T) {
	recordSchema := `{
		"type": "record",
		"name": "User",
		"namespace": "com.example",
		"fields": [
			{"name": "id", "type": "int"},
			{"name": "name", "type": "string"}
		]
	}`
	unionSchemaJSON := `["null", ` + recordSchema + `]`
	schema, err := avro.Parse(unionSchemaJSON)
	require.NoError(t, err)
	unionSchema := schema.(*avro.UnionSchema)

	// Test with already wrapped record
	wrappedData := map[string]any{
		"com.example.User": map[string]any{
			"id":   float64(123),
			"name": "test",
		},
	}
	got, err := convertUnionField(wrappedData, unionSchema)
	assert.NoError(t, err)
	assert.IsType(t, map[string]any{}, got)
	result := got.(map[string]any)
	assert.Len(t, result, 1)
	assert.Contains(t, result, "com.example.User")
	userData := result["com.example.User"].(map[string]any)
	assert.Equal(t, int32(123), userData["id"])
	assert.Equal(t, "test", userData["name"])
}

func TestConvertUnionField_Primitive(t *testing.T) {
	schema, err := avro.Parse(`["null", "int", "string"]`)
	require.NoError(t, err)
	unionSchema := schema.(*avro.UnionSchema)

	tests := []struct {
		name string
		data any
		want any
	}{
		{
			name: "int value",
			data: float64(42),
			want: int32(42),
		},
		{
			name: "string value",
			data: "hello",
			want: "hello",
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := convertUnionField(testCase.data, unionSchema)
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestConvertUnionField_WrappedPrimitive(t *testing.T) {
	// Test union with int and logical type (date)
	schema, err := avro.Parse(`["null", {"type": "int", "logicalType": "date"}]`)
	require.NoError(t, err)
	unionSchema := schema.(*avro.UnionSchema)

	tests := []struct {
		name string
		data any
		want any
	}{
		{
			name: "wrapped int value",
			data: map[string]any{"int": float64(20474)},
			want: map[string]any{"int.date": int32(20474)},
		},
		{
			name: "wrapped int with logical type suffix",
			data: map[string]any{"int.date": float64(20474)},
			want: map[string]any{"int.date": int32(20474)},
		},
		{
			name: "wrapped int with float64 value",
			data: map[string]any{"int": float64(42)},
			want: map[string]any{"int.date": int32(42)},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := convertUnionField(testCase.data, unionSchema)
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestConvertUnionField_WrappedPrimitiveMultipleTypes(t *testing.T) {
	schema, err := avro.Parse(`["null", "int", "string", "long"]`)
	require.NoError(t, err)
	unionSchema := schema.(*avro.UnionSchema)

	tests := []struct {
		name string
		data any
		want any
	}{
		{
			name: "wrapped int",
			data: map[string]any{"int": float64(42)},
			want: int32(42),
		},
		{
			name: "wrapped string",
			data: map[string]any{"string": "hello"},
			want: "hello",
		},
		{
			name: "wrapped long",
			data: map[string]any{"long": float64(9000000000000000000)},
			want: int64(9000000000000000000),
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := convertUnionField(testCase.data, unionSchema)
			assert.NoError(t, err)
			assert.Equal(t, testCase.want, got)
		})
	}
}

func TestConvertUnionField_UnknownWrappedPrimitiveKey(t *testing.T) {
	schema, err := avro.Parse(`["null", "int", "string"]`)
	require.NoError(t, err)
	unionSchema := schema.(*avro.UnionSchema)

	data := map[string]any{"unknown": float64(42)}
	got, err := convertUnionField(data, unionSchema)
	assert.NoError(t, err)
	// Unknown wrapper keys must not be treated as successful primitive matches.
	assert.Equal(t, data, got)
}

// TestConvertFloat64ToIntForIntegerFields_UnionWithLogicalType tests the exact scenario from issue #376
// where a union type contains an int with logical type "date".
func TestConvertFloat64ToIntForIntegerFields_UnionWithLogicalType(t *testing.T) {
	schema, err := avro.Parse(testDocumentLogicalDateSchemaJSON)
	require.NoError(t, err)

	// Test case 1: wrapped as {"int": value} - should work
	data1 := map[string]any{
		"documentValidTo": map[string]any{"int": float64(20474)},
	}
	got1, err := convertFloat64ToIntForIntegerFields(data1, schema)
	assert.NoError(t, err)
	result1 := got1.(map[string]any)
	assert.Equal(t, map[string]any{"int.date": int32(20474)}, result1["documentValidTo"])

	// Test case 2: wrapped as {"int.date": value} - should also work (strips logical type suffix)
	data2 := map[string]any{
		"documentValidTo": map[string]any{"int.date": float64(20474)},
	}
	got2, err := convertFloat64ToIntForIntegerFields(data2, schema)
	assert.NoError(t, err)
	result2 := got2.(map[string]any)
	assert.Equal(t, map[string]any{"int.date": int32(20474)}, result2["documentValidTo"])

	// Test case 3: null value - should work
	data3 := map[string]any{
		"documentValidTo": nil,
	}
	got3, err := convertFloat64ToIntForIntegerFields(data3, schema)
	assert.NoError(t, err)
	result3 := got3.(map[string]any)
	assert.Nil(t, result3["documentValidTo"])
}

func TestConvertFloat64ToIntForIntegerFields_NestedUnionWithLogicalTypeIssue376(t *testing.T) {
	schemaJSON := `{
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
	}`
	schema, err := avro.Parse(schemaJSON)
	require.NoError(t, err)

	testCases := []struct {
		name          string
		documentType  any
		documentValid any
		expectedDay   int32
	}{
		{
			name:          "wrapped int.date",
			documentType:  map[string]any{"string": "OP"},
			documentValid: map[string]any{"int.date": float64(20474)},
			expectedDay:   int32(20474),
		},
		{
			name:          "wrapped int",
			documentType:  map[string]any{"string": "OP"},
			documentValid: map[string]any{"int": float64(20475)},
			expectedDay:   int32(20475),
		},
		{
			name:          "direct scalar",
			documentType:  "OP",
			documentValid: float64(20476),
			expectedDay:   int32(20476),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			data := map[string]any{
				"document": map[string]any{
					"com.example.Document": map[string]any{
						"documentId":      "BR9876543",
						"documentType":    testCase.documentType,
						"documentValidTo": testCase.documentValid,
					},
				},
			}

			got, convErr := convertFloat64ToIntForIntegerFields(data, schema)
			require.NoError(t, convErr)

			result := got.(map[string]any)
			documentUnion := result["document"].(map[string]any)
			document := documentUnion["com.example.Document"].(map[string]any)

			documentValidTo, ok := document["documentValidTo"].(map[string]any)
			require.True(t, ok, "documentValidTo should be wrapped with logical discriminator")
			assert.Equal(t, map[string]any{"int.date": testCase.expectedDay}, documentValidTo)
		})
	}
}

// TestSerializeDeserializeRoundTrip_UnionWithLogicalType verifies that after
// roundtrip the logical union date field is deserialized as an unwrapped value.
func TestSerializeDeserializeRoundTrip_UnionWithLogicalType(t *testing.T) {
	avroSerde := &AvroSerde{}
	schema := &Schema{
		ID:      376,
		Schema:  testDocumentLogicalDateSchemaJSON,
		Version: 1,
		Subject: "issue-376-roundtrip",
	}

	originalData := map[string]any{
		"documentValidTo": map[string]any{"int.date": float64(20474)},
	}

	serialized, serdeErr := avroSerde.Serialize(originalData, schema)
	require.Nil(t, serdeErr, "Serialize should not return error")
	require.NotNil(t, serialized, "Serialized data should not be nil")

	deserialized, deserErr := avroSerde.Deserialize(serialized, schema)
	require.Nil(t, deserErr, "Deserialize should not return error")
	require.NotNil(t, deserialized, "Deserialized data should not be nil")

	result := deserialized.(map[string]any)
	value := result["documentValidTo"]
	switch typedValue := value.(type) {
	case int:
		assert.Equal(t, 20474, typedValue)
	case int32:
		assert.Equal(t, int32(20474), typedValue)
	case time.Time:
		assert.Equal(t, int64(20474*86400), typedValue.Unix())
	default:
		t.Fatalf("unexpected type for documentValidTo: %T", value)
	}
}

func TestConvertFloat64ToIntForIntegerFields_RecordWithUnions(t *testing.T) {
	schemaJSON := `{
		"type": "record",
		"name": "TestRecord",
		"namespace": "com.example",
		"fields": [
			{"name": "id", "type": "int"},
			{"name": "optional", "type": ["null", "string"]},
			{"name": "enumField", "type": ["null", {
				"type": "enum",
				"name": "Status",
				"namespace": "com.example",
				"symbols": ["ACTIVE", "INACTIVE"]
			}]}
		]
	}`
	schema, err := avro.Parse(schemaJSON)
	require.NoError(t, err)

	data := map[string]any{
		"id":        float64(123),
		"optional":  "test",
		"enumField": "ACTIVE",
	}

	got, err := convertFloat64ToIntForIntegerFields(data, schema)
	assert.NoError(t, err)
	result := got.(map[string]any)
	assert.Equal(t, int32(123), result["id"])
	assert.Equal(t, "test", result["optional"])
	assert.IsType(t, map[string]any{}, result["enumField"])
	enumMap := result["enumField"].(map[string]any)
	assert.Equal(t, "ACTIVE", enumMap["com.example.Status"])
}

func TestConvertFloat64ToIntForIntegerFields_NestedRecord(t *testing.T) {
	schemaJSON := `{
		"type": "record",
		"name": "Outer",
		"namespace": "com.example",
		"fields": [
			{"name": "inner", "type": {
				"type": "record",
				"name": "Inner",
				"namespace": "com.example",
				"fields": [
					{"name": "value", "type": "long"}
				]
			}}
		]
	}`
	schema, err := avro.Parse(schemaJSON)
	require.NoError(t, err)

	data := map[string]any{
		"inner": map[string]any{
			"value": float64(456),
		},
	}

	got, err := convertFloat64ToIntForIntegerFields(data, schema)
	assert.NoError(t, err)
	result := got.(map[string]any)
	inner := result["inner"].(map[string]any)
	assert.Equal(t, int64(456), inner["value"])
}

func TestConvertFloat64ToIntForIntegerFields_Array(t *testing.T) {
	schemaJSON := `{
		"type": "array",
		"items": "int"
	}`
	schema, err := avro.Parse(schemaJSON)
	require.NoError(t, err)

	data := []any{float64(1), float64(2), float64(3)}

	got, err := convertFloat64ToIntForIntegerFields(data, schema)
	assert.NoError(t, err)
	result := got.([]any)
	assert.Equal(t, []any{int32(1), int32(2), int32(3)}, result)
}

func TestConvertFloat64ToIntForIntegerFields_Map(t *testing.T) {
	schemaJSON := `{
		"type": "map",
		"values": "long"
	}`
	schema, err := avro.Parse(schemaJSON)
	require.NoError(t, err)

	data := map[string]any{
		"key1": float64(100),
		"key2": float64(200),
	}

	got, err := convertFloat64ToIntForIntegerFields(data, schema)
	assert.NoError(t, err)
	result := got.(map[string]any)
	assert.Equal(t, int64(100), result["key1"])
	assert.Equal(t, int64(200), result["key2"])
}

func TestUnwrapUnionValue_Null(t *testing.T) {
	schema, err := avro.Parse(`["null", "string"]`)
	require.NoError(t, err)
	unionSchema := schema.(*avro.UnionSchema)

	got, err := unwrapUnionValue(nil, unionSchema)
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestUnwrapUnionValue_WrappedEnum(t *testing.T) {
	enumSchema := `{
		"type": "enum",
		"name": "Status",
		"namespace": "com.example",
		"symbols": ["ACTIVE", "INACTIVE"]
	}`
	unionSchemaJSON := `["null", ` + enumSchema + `]`
	schema, err := avro.Parse(unionSchemaJSON)
	require.NoError(t, err)
	unionSchema := schema.(*avro.UnionSchema)

	wrapped := map[string]any{
		"com.example.Status": "ACTIVE",
	}

	got, err := unwrapUnionValue(wrapped, unionSchema)
	assert.NoError(t, err)
	assert.Equal(t, "ACTIVE", got)
}

func TestUnwrapUnionValue_WrappedRecord(t *testing.T) {
	recordSchema := `{
		"type": "record",
		"name": "User",
		"namespace": "com.example",
		"fields": [
			{"name": "id", "type": "int"},
			{"name": "name", "type": "string"}
		]
	}`
	unionSchemaJSON := `["null", ` + recordSchema + `]`
	schema, err := avro.Parse(unionSchemaJSON)
	require.NoError(t, err)
	unionSchema := schema.(*avro.UnionSchema)

	wrapped := map[string]any{
		"com.example.User": map[string]any{
			"id":   int32(123),
			"name": "test",
		},
	}

	got, err := unwrapUnionValue(wrapped, unionSchema)
	assert.NoError(t, err)
	result := got.(map[string]any)
	assert.Equal(t, int32(123), result["id"])
	assert.Equal(t, "test", result["name"])
}

func TestUnwrapUnionValues_RecordWithUnions(t *testing.T) {
	schemaJSON := `{
		"type": "record",
		"name": "TestRecord",
		"namespace": "com.example",
		"fields": [
			{"name": "optional", "type": ["null", "string"]},
			{"name": "enumField", "type": ["null", {
				"type": "enum",
				"name": "Status",
				"namespace": "com.example",
				"symbols": ["ACTIVE", "INACTIVE"]
			}]},
			{"name": "recordField", "type": ["null", {
				"type": "record",
				"name": "User",
				"namespace": "com.example",
				"fields": [{"name": "id", "type": "int"}]
			}]}
		]
	}`
	schema, err := avro.Parse(schemaJSON)
	require.NoError(t, err)

	data := map[string]any{
		"optional":  "test",
		"enumField": map[string]any{"com.example.Status": "ACTIVE"},
		"recordField": map[string]any{
			"com.example.User": map[string]any{"id": int32(123)},
		},
	}

	got, err := unwrapUnionValues(data, schema)
	assert.NoError(t, err)
	result := got.(map[string]any)
	assert.Equal(t, "test", result["optional"])
	assert.Equal(t, "ACTIVE", result["enumField"])
	userData := result["recordField"].(map[string]any)
	assert.Equal(t, int32(123), userData["id"])
}

func TestUnwrapUnionValues_Array(t *testing.T) {
	schemaJSON := `{
		"type": "array",
		"items": ["null", "string"]
	}`
	schema, err := avro.Parse(schemaJSON)
	require.NoError(t, err)

	data := []any{
		nil,
		"hello",
		map[string]any{"string": "world"},
	}

	got, err := unwrapUnionValues(data, schema)
	assert.NoError(t, err)
	result := got.([]any)
	assert.Nil(t, result[0])
	assert.Equal(t, "hello", result[1])
	// For wrapped union values in arrays, unwrapUnionValue tries to match types
	// If it can't find a match, it returns as-is, so we check the wrapped format
	assert.IsType(t, map[string]any{}, result[2])
	wrapped := result[2].(map[string]any)
	assert.Equal(t, "world", wrapped["string"])
}

func TestUnwrapUnionValues_Map(t *testing.T) {
	schemaJSON := `{
		"type": "map",
		"values": ["null", "int"]
	}`
	schema, err := avro.Parse(schemaJSON)
	require.NoError(t, err)

	data := map[string]any{
		"key1": nil,
		"key2": int32(42),
		"key3": map[string]any{"int": int32(100)},
	}

	got, err := unwrapUnionValues(data, schema)
	assert.NoError(t, err)
	result := got.(map[string]any)
	assert.Nil(t, result["key1"])
	assert.Equal(t, int32(42), result["key2"])
	// For wrapped union values, unwrapUnionValue should unwrap them
	// But if it can't match, it returns as-is
	wrapped := result["key3"]
	if wrappedMap, ok := wrapped.(map[string]any); ok {
		assert.Equal(t, int32(100), wrappedMap["int"])
	} else {
		assert.Equal(t, int32(100), wrapped)
	}
}

func TestSerializeDeserializeRoundTrip_WithUnions(t *testing.T) {
	avroSerde := &AvroSerde{}
	schemaJSON := `{
		"type": "record",
		"name": "TestRecord",
		"namespace": "com.example",
		"fields": [
			{"name": "id", "type": "int"},
			{"name": "optional", "type": ["null", "string"]},
			{"name": "enumField", "type": ["null", {
				"type": "enum",
				"name": "Status",
				"namespace": "com.example",
				"symbols": ["ACTIVE", "INACTIVE"]
			}]},
			{"name": "bytesField", "type": ["null", "bytes"]}
		]
	}`
	schema := &Schema{
		ID:      1,
		Schema:  schemaJSON,
		Version: 1,
		Subject: "test",
	}

	originalData := map[string]any{
		"id":         float64(123),
		"optional":   "test",
		"enumField":  "ACTIVE",
		"bytesField": []any{float64(1), float64(2), float64(3)},
	}

	// Serialize
	serialized, serdeErr := avroSerde.Serialize(originalData, schema)
	require.Nil(t, serdeErr, "Serialize should not return error")
	require.NotNil(t, serialized, "Serialized data should not be nil")

	// Deserialize
	deserialized, deserErr := avroSerde.Deserialize(serialized, schema)
	require.Nil(t, deserErr, "Deserialize should not return error")
	require.NotNil(t, deserialized, "Deserialized data should not be nil")

	result := deserialized.(map[string]any)
	// After deserialization, int values may be returned as int (platform-dependent)
	// Check that the value is correct regardless of type
	idValue := result["id"]
	if idInt32, ok := idValue.(int32); ok {
		assert.Equal(t, int32(123), idInt32)
	} else if idInt, ok := idValue.(int); ok {
		assert.Equal(t, 123, idInt)
	} else {
		t.Errorf("unexpected type for id: %T", idValue)
	}
	assert.Equal(t, "test", result["optional"])
	assert.Equal(t, "ACTIVE", result["enumField"])
	assert.Equal(t, []byte{1, 2, 3}, result["bytesField"])
}

func TestAvroMarshal_UnionLogicalTypeDateAcceptedShapes(t *testing.T) {
	schema, err := avro.Parse(testDocumentLogicalDateSchemaJSON)
	require.NoError(t, err)

	// Direct int32 value is rejected by hamba/avro for this logical union shape.
	_, err = avro.Marshal(schema, map[string]any{"documentValidTo": int32(20474)})
	assert.Error(t, err)

	// Wrapped using primitive type name is also rejected.
	_, err = avro.Marshal(schema, map[string]any{"documentValidTo": map[string]any{"int": int32(20474)}})
	assert.Error(t, err)

	// Wrapped using logical union type discriminator works.
	_, err = avro.Marshal(schema, map[string]any{"documentValidTo": map[string]any{"int.date": int32(20474)}})
	assert.NoError(t, err)
}
