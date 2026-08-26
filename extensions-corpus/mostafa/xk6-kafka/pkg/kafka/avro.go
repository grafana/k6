package kafka

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/hamba/avro/v2"
)

var (
	// ErrCannotConvertToByte is returned when a value cannot be converted to byte.
	ErrCannotConvertToByte = errors.New("cannot convert value to byte")
	// ErrCannotConvertToInt32 is returned when a float64 cannot be converted to int32.
	ErrCannotConvertToInt32 = errors.New("cannot convert float64 to int32: not an integer")
	// ErrCannotConvertToInt64 is returned when a float64 cannot be converted to int64.
	ErrCannotConvertToInt64 = errors.New("cannot convert float64 to int64: not an integer")
)

type AvroSerde struct {
	Serdes
}

func convertNumericValueToByte(value any) (byte, error) {
	switch val := value.(type) {
	case float64:
		if val < 0 || val > math.MaxUint8 || math.Trunc(val) != val {
			return 0, fmt.Errorf("%w: %v", ErrCannotConvertToByte, value)
		}
		//nolint:gosec // value is range-checked and integral before narrowing conversion
		return byte(val), nil
	case int:
		if val < 0 || val > math.MaxUint8 {
			return 0, fmt.Errorf("%w: %v", ErrCannotConvertToByte, value)
		}
		//nolint:gosec // value is range-checked before narrowing conversion
		return byte(val), nil
	case int32:
		if val < 0 || val > math.MaxUint8 {
			return 0, fmt.Errorf("%w: %v", ErrCannotConvertToByte, value)
		}
		//nolint:gosec // value is range-checked before narrowing conversion
		return byte(val), nil
	case int64:
		if val < 0 || val > math.MaxUint8 {
			return 0, fmt.Errorf("%w: %v", ErrCannotConvertToByte, value)
		}
		//nolint:gosec // value is range-checked before narrowing conversion
		return byte(val), nil
	default:
		return 0, fmt.Errorf("%w: %T", ErrCannotConvertToByte, value)
	}
}

// convertPrimitiveType converts a primitive value to the correct Avro type.
// Handles float64->int32/int64 conversion and array->bytes conversion.
func convertPrimitiveType(data any, schema avro.Schema) (any, error) {
	switch schema.Type() {
	case avro.Fixed:
		fixedSchema, ok := schema.(*avro.FixedSchema)
		if !ok {
			return data, nil
		}
		size := fixedSchema.Size()
		if arr, ok := data.([]any); ok {
			if len(arr) != size {
				return nil, fmt.Errorf("%w: expected %d elements, got %d", ErrCannotConvertToByte, size, len(arr))
			}
			fixedBytes := reflect.New(reflect.ArrayOf(size, reflect.TypeFor[uint8]())).Elem()
			for i, v := range arr {
				convertedByte, err := convertNumericValueToByte(v)
				if err != nil {
					return nil, fmt.Errorf("%w at index %d: %T", ErrCannotConvertToByte, i, v)
				}
				fixedBytes.Index(i).SetUint(uint64(convertedByte))
			}
			return fixedBytes.Interface(), nil
		}
		return data, nil
	case avro.Bytes:
		// Convert array of numbers to []byte for bytes fields
		if arr, ok := data.([]any); ok {
			bytes := make([]byte, len(arr))
			for i, v := range arr {
				convertedByte, err := convertNumericValueToByte(v)
				if err != nil {
					return nil, fmt.Errorf("%w at index %d: %T", ErrCannotConvertToByte, i, v)
				}
				bytes[i] = convertedByte
			}
			return bytes, nil
		}
		if bytes, ok := data.([]byte); ok {
			return bytes, nil
		}
		return data, nil
	case avro.Int:
		if f, ok := data.(float64); ok {
			if f != float64(int32(f)) {
				return nil, fmt.Errorf("%w: %f", ErrCannotConvertToInt32, f)
			}
			return int32(f), nil
		}
		return data, nil
	case avro.Long:
		if f, ok := data.(float64); ok {
			if f != float64(int64(f)) {
				return nil, fmt.Errorf("%w: %f", ErrCannotConvertToInt64, f)
			}
			return int64(f), nil
		}
		return data, nil
	case avro.Record, avro.Error, avro.Ref, avro.Enum, avro.Array, avro.Map,
		avro.Union, avro.String, avro.Float, avro.Double,
		avro.Boolean, avro.Null:
		fallthrough
	default:
		return data, nil
	}
}

// getPrimitiveTypeName returns the Avro primitive type name for a given schema type,
// or empty string if it's not a primitive type.
func getPrimitiveTypeName(schemaType avro.Type) string {
	switch schemaType {
	case avro.Null:
		return "null"
	case avro.Boolean:
		return "boolean"
	case avro.Int:
		return "int"
	case avro.Long:
		return "long"
	case avro.Float:
		return "float"
	case avro.Double:
		return "double"
	case avro.Bytes:
		return "bytes"
	case avro.String:
		return "string"
	case avro.Record, avro.Error, avro.Ref, avro.Enum, avro.Array, avro.Map, avro.Union, avro.Fixed:
		return ""
	default:
		return ""
	}
}

// getPrimitiveUnionDiscriminator returns the discriminator key expected by hamba/avro
// for a primitive union branch. For logical primitive types, this is typically
// "<primitive>.<logicalType>" (for example: "int.date").
func getPrimitiveUnionDiscriminator(schema avro.Schema) string {
	if schema == nil {
		return ""
	}

	if refSchema, ok := schema.(*avro.RefSchema); ok {
		schema = refSchema.Schema()
	}

	primitive := getPrimitiveTypeName(schema.Type())
	if primitive == "" {
		return ""
	}

	schemaString := schema.String()
	if !strings.HasPrefix(schemaString, "{") {
		return primitive
	}

	var schemaMap map[string]any
	if err := json.Unmarshal([]byte(schemaString), &schemaMap); err != nil {
		return primitive
	}

	logicalType, ok := schemaMap["logicalType"].(string)
	if !ok || logicalType == "" {
		return primitive
	}

	return primitive + "." + logicalType
}

// isValueCompatibleWithSchema checks whether value has a Go type compatible with the Avro schema.
// It is used by union matching to avoid accepting a branch when conversion didn't actually produce
// a value that can be encoded for that branch.
func isValueCompatibleWithSchema(value any, schema avro.Schema) bool {
	if schema == nil {
		return false
	}

	if refSchema, ok := schema.(*avro.RefSchema); ok {
		schema = refSchema.Schema()
	}

	switch schema.Type() {
	case avro.Null:
		return value == nil
	case avro.Boolean:
		_, ok := value.(bool)
		return ok
	case avro.Int:
		switch value.(type) {
		case int32, int16, int8, int:
			return true
		default:
			return false
		}
	case avro.Long:
		switch value.(type) {
		case int64, int32, int16, int8, int:
			return true
		default:
			return false
		}
	case avro.Float:
		switch value.(type) {
		case float32, float64:
			return true
		default:
			return false
		}
	case avro.Double:
		switch value.(type) {
		case float64, float32:
			return true
		default:
			return false
		}
	case avro.Bytes:
		_, ok := value.([]byte)
		return ok
	case avro.String, avro.Enum:
		_, ok := value.(string)
		return ok
	case avro.Array:
		_, ok := value.([]any)
		return ok
	case avro.Map, avro.Record:
		_, ok := value.(map[string]any)
		return ok
	case avro.Union:
		return true
	case avro.Fixed, avro.Error, avro.Ref:
		return true
	default:
		return true
	}
}

// convertUnionField converts a union field value, wrapping named schemas appropriately.
func convertUnionField(fieldValue any, unionSchema *avro.UnionSchema) (any, error) {
	if fieldValue == nil {
		//nolint: nilnil // nil is a valid union value
		return nil, nil
	}

	types := unionSchema.Types()

	// Handle map values (could be wrapped union or record)
	if fieldValueMap, ok := fieldValue.(map[string]any); ok {
		// Check if it's already wrapped: {"typeName": value}
		if len(fieldValueMap) == 1 {
			for key, wrappedValue := range fieldValueMap {
				// First, try to match as a primitive type name (e.g., "int", "string")
				// Handle logical types like "int.date" by stripping the suffix
				primitiveKey := key
				for i := range key {
					if key[i] == '.' {
						primitiveKey = key[:i]
						break
					}
				}

				// Try to find matching primitive type
				for _, unionType := range types {
					if unionType.Type() == avro.Null {
						continue
					}
					actualType := unionType
					if refSchema, ok := unionType.(*avro.RefSchema); ok {
						actualType = refSchema.Schema()
					}

					// Check if this is a primitive type matching the key
					if primitiveName := getPrimitiveTypeName(actualType.Type()); primitiveName != "" && primitiveName == primitiveKey {
						// Found matching primitive type, unwrap and convert
						converted, err := convertFloat64ToIntForIntegerFields(wrappedValue, actualType)
						if err != nil {
							return nil, err
						}
						if !isValueCompatibleWithSchema(converted, actualType) {
							continue
						}
						discriminator := getPrimitiveUnionDiscriminator(actualType)
						if discriminator != "" && discriminator != primitiveName {
							return map[string]any{discriminator: converted}, nil
						}
						// Return unwrapped value for non-logical primitives.
						return converted, nil
					}

					// Try to find matching named schema
					if namedSchema, ok := actualType.(avro.NamedSchema); ok && namedSchema.FullName() == key {
						// Already wrapped, convert nested value
						converted, err := convertFloat64ToIntForIntegerFields(wrappedValue, actualType)
						if err != nil {
							return nil, err
						}
						return map[string]any{key: converted}, nil
					}
				}
			}
		}

		// Not wrapped, try to match as record
		for _, unionType := range types {
			if unionType.Type() == avro.Null {
				continue
			}
			actualType := unionType
			if refSchema, ok := unionType.(*avro.RefSchema); ok {
				actualType = refSchema.Schema()
			}
			if actualType.Type() == avro.Record {
				converted, err := convertFloat64ToIntForIntegerFields(fieldValueMap, actualType)
				if err == nil {
					if namedSchema, ok := actualType.(avro.NamedSchema); ok {
						return map[string]any{namedSchema.FullName(): converted}, nil
					}
					return converted, nil
				}
			}
		}
	}

	// Handle non-map values (primitives, enums)
	for _, unionType := range types {
		if unionType.Type() == avro.Null {
			continue
		}
		actualType := unionType
		if refSchema, ok := unionType.(*avro.RefSchema); ok {
			actualType = refSchema.Schema()
		}

		// Primitive types should stay unwrapped.
		if primitiveName := getPrimitiveTypeName(actualType.Type()); primitiveName != "" {
			converted, err := convertFloat64ToIntForIntegerFields(fieldValue, actualType)
			if err == nil && isValueCompatibleWithSchema(converted, actualType) {
				discriminator := getPrimitiveUnionDiscriminator(actualType)
				if discriminator != "" && discriminator != primitiveName {
					return map[string]any{discriminator: converted}, nil
				}
				return converted, nil
			}
			continue
		}

		// Named schemas (enums, fixed, records) need wrapping.
		if namedSchema, ok := actualType.(avro.NamedSchema); ok {
			if actualType.Type() == avro.Enum {
				// Enums are strings, wrap directly
				return map[string]any{namedSchema.FullName(): fieldValue}, nil
			}
			// Other named types, try converting first
			converted, err := convertFloat64ToIntForIntegerFields(fieldValue, actualType)
			if err != nil {
				continue
			}
			return map[string]any{namedSchema.FullName(): converted}, nil
		}
	}

	// Couldn't match, return as-is
	return fieldValue, nil
}

// convertFloat64ToIntForIntegerFields converts float64 values to int32/int64 for int/long schema fields.
// This is necessary because JSON unmarshaling converts all numbers to float64,
// but Avro int fields require int32 values and long fields require int64 values.
func convertFloat64ToIntForIntegerFields(data any, schema avro.Schema) (any, error) {
	if schema == nil {
		return data, nil
	}

	// Handle schema references
	if refSchema, ok := schema.(*avro.RefSchema); ok {
		schema = refSchema.Schema()
	}

	switch schema.Type() {
	case avro.Bytes, avro.Int, avro.Long, avro.Fixed:
		return convertPrimitiveType(data, schema)
	case avro.Record:
		return convertRecordFields(data, schema, func(fieldValue any, fieldType avro.Schema) (any, error) {
			if unionSchema, ok := fieldType.(*avro.UnionSchema); ok {
				return convertUnionField(fieldValue, unionSchema)
			}
			return convertFloat64ToIntForIntegerFields(fieldValue, fieldType)
		})
	case avro.Array:
		arraySchema, ok := schema.(*avro.ArraySchema)
		if !ok {
			return data, nil
		}

		dataArray, ok := data.([]any)
		if !ok {
			return data, nil
		}

		convertedArray := make([]any, len(dataArray))
		for i, item := range dataArray {
			convertedItem, err := convertFloat64ToIntForIntegerFields(item, arraySchema.Items())
			if err != nil {
				return nil, fmt.Errorf("array index %d: %w", i, err)
			}
			convertedArray[i] = convertedItem
		}

		return convertedArray, nil
	case avro.Map:
		mapSchema, ok := schema.(*avro.MapSchema)
		if !ok {
			return data, nil
		}

		dataMap, ok := data.(map[string]any)
		if !ok {
			return data, nil
		}

		convertedMap := make(map[string]any)
		for k, v := range dataMap {
			convertedValue, err := convertFloat64ToIntForIntegerFields(v, mapSchema.Values())
			if err != nil {
				return nil, fmt.Errorf("map key %s: %w", k, err)
			}
			convertedMap[k] = convertedValue
		}

		return convertedMap, nil
	case avro.Union:
		unionSchema, ok := schema.(*avro.UnionSchema)
		if !ok {
			return data, nil
		}
		return convertUnionField(data, unionSchema)
	case avro.Error, avro.Ref, avro.Enum, avro.String,
		avro.Float, avro.Double, avro.Boolean, avro.Null:
		fallthrough
	default:
		return data, nil
	}
}

// convertRecordFields processes record fields using the provided field converter function.
func convertRecordFields(data any, schema avro.Schema, convertField func(any, avro.Schema) (any, error)) (any, error) {
	recordSchema, ok := schema.(*avro.RecordSchema)
	if !ok {
		return data, nil
	}

	dataMap, ok := data.(map[string]any)
	if !ok {
		return data, nil
	}

	resultMap := make(map[string]any)
	for _, field := range recordSchema.Fields() {
		fieldName := field.Name()
		fieldValue, exists := dataMap[fieldName]
		if !exists {
			continue
		}

		fieldType := field.Type()
		convertedValue, err := convertField(fieldValue, fieldType)
		if err != nil {
			return nil, fmt.Errorf("field %s: %w", fieldName, err)
		}
		resultMap[fieldName] = convertedValue
	}

	// Copy any remaining fields that aren't in the schema
	for k, v := range dataMap {
		if _, exists := resultMap[k]; !exists {
			resultMap[k] = v
		}
	}

	return resultMap, nil
}

// Serialize serializes a JSON object into Avro binary.
func (*AvroSerde) Serialize(data any, schema *Schema) ([]byte, *Xk6KafkaError) {
	jsonBytes, err := toJSONBytes(data)
	if err != nil {
		return nil, err
	}

	avroSchema := schema.Codec()
	if avroSchema == nil {
		return nil, NewXk6KafkaError(failedToEncode, "Failed to parse Avro schema", nil)
	}

	// Parse JSON data into a map for marshaling
	var jsonData any
	jsonErr := json.Unmarshal(jsonBytes, &jsonData)
	if jsonErr != nil {
		return nil, NewXk6KafkaError(failedToEncode, "Failed to parse JSON data", jsonErr)
	}

	// Convert float64 to int32/int64 for int/long fields before marshaling
	convertedData, convertErr := convertFloat64ToIntForIntegerFields(jsonData, avroSchema)
	if convertErr != nil {
		return nil, NewXk6KafkaError(failedToEncode,
			fmt.Sprintf("Failed to convert float64 to int32/int64 for integer fields: %v", convertErr),
			convertErr)
	}

	// Marshal to binary using hamba/avro
	bytesData, originalErr := avro.Marshal(avroSchema, convertedData)
	if originalErr != nil {
		return nil, NewXk6KafkaError(failedToEncodeToBinary,
			"Failed to encode data into binary",
			originalErr)
	}

	return bytesData, nil
}

// unwrapUnionValues recursively unwraps union values that are wrapped in the
// {"typeName": value} format returned by hamba/avro for named types in unions.
func unwrapUnionValues(data any, schema avro.Schema) (any, error) {
	if data == nil {
		//nolint: nilnil // nil is a valid value
		return nil, nil
	}

	switch schema.Type() {
	case avro.Record:
		return convertRecordFields(data, schema, func(fieldValue any, fieldType avro.Schema) (any, error) {
			if unionSchema, ok := fieldType.(*avro.UnionSchema); ok {
				return unwrapUnionValue(fieldValue, unionSchema)
			}
			return unwrapUnionValues(fieldValue, fieldType)
		})
	case avro.Array:
		arraySchema, ok := schema.(*avro.ArraySchema)
		if !ok {
			return data, nil
		}

		dataArray, ok := data.([]any)
		if !ok {
			return data, nil
		}

		unwrappedArray := make([]any, len(dataArray))
		for i, item := range dataArray {
			unwrappedItem, err := unwrapUnionValues(item, arraySchema.Items())
			if err != nil {
				return nil, fmt.Errorf("array index %d: %w", i, err)
			}
			unwrappedArray[i] = unwrappedItem
		}

		return unwrappedArray, nil
	case avro.Map:
		mapSchema, ok := schema.(*avro.MapSchema)
		if !ok {
			return data, nil
		}

		dataMap, ok := data.(map[string]any)
		if !ok {
			return data, nil
		}

		unwrappedMap := make(map[string]any)
		for k, v := range dataMap {
			unwrappedValue, err := unwrapUnionValues(v, mapSchema.Values())
			if err != nil {
				return nil, fmt.Errorf("map key %s: %w", k, err)
			}
			unwrappedMap[k] = unwrappedValue
		}

		return unwrappedMap, nil
	case avro.Error, avro.Ref, avro.Enum, avro.Union, avro.Fixed,
		avro.String, avro.Bytes, avro.Int, avro.Long, avro.Float,
		avro.Double, avro.Boolean, avro.Null:
		fallthrough
	default:
		return data, nil
	}
}

// unwrapUnionValue unwraps a single union value if it's wrapped in {"typeName": value} format.
func unwrapUnionValue(value any, unionSchema *avro.UnionSchema) (any, error) {
	if value == nil {
		//nolint: nilnil // nil is a valid union value
		return nil, nil
	}

	// Check if value is wrapped as {"typeName": value}
	if valueMap, ok := value.(map[string]any); ok && len(valueMap) == 1 {
		for key, wrappedValue := range valueMap {
			// Check if key matches any union type's full name
			for _, unionType := range unionSchema.Types() {
				if unionType.Type() == avro.Null {
					continue
				}
				actualType := unionType
				if refSchema, ok := unionType.(*avro.RefSchema); ok {
					actualType = refSchema.Schema()
				}

				if namedSchema, ok := actualType.(avro.NamedSchema); ok && namedSchema.FullName() == key {
					// Found matching type - unwrap and recursively process
					return unwrapUnionValues(wrappedValue, actualType)
				}
			}
		}
	}

	// Not wrapped - try to recursively unwrap nested structures
	// Find the first matching union type that can successfully unwrap the value
	for _, unionType := range unionSchema.Types() {
		if unionType.Type() == avro.Null {
			continue
		}
		actualType := unionType
		if refSchema, ok := unionType.(*avro.RefSchema); ok {
			actualType = refSchema.Schema()
		}

		if unwrapped, err := unwrapUnionValues(value, actualType); err == nil {
			return unwrapped, nil
		}
	}

	// If we can't determine the type, return as-is
	return value, nil
}

// Deserialize deserializes a Avro binary into a JSON object.
func (*AvroSerde) Deserialize(data []byte, schema *Schema) (any, *Xk6KafkaError) {
	avroSchema := schema.Codec()
	if avroSchema == nil {
		return nil, NewXk6KafkaError(failedToDecodeFromBinary, "Failed to parse Avro schema", nil)
	}

	var decodedData any
	err := avro.Unmarshal(avroSchema, data, &decodedData)
	if err != nil {
		return nil, NewXk6KafkaError(
			failedToDecodeFromBinary, "Failed to decode data", err)
	}

	// Unwrap union values that are wrapped in {"typeName": value} format
	unwrappedData, unwrapErr := unwrapUnionValues(decodedData, avroSchema)
	if unwrapErr != nil {
		// Return original data if unwrapping fails
		unwrappedData = decodedData
	}

	if data, ok := unwrappedData.(map[string]any); ok {
		return data, nil
	}
	return unwrappedData, nil
}
