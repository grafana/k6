package kafka

import (
	"go.k6.io/k6/js/common"
)

type Container struct {
	Data           any        `json:"data"`
	Schema         *Schema    `json:"schema"`
	SchemaType     SchemaType `json:"schemaType"`
	ProtobufFormat string     `json:"protobufFormat"`
}

// serialize checks whether the incoming data has a schema or not.
// If the data has a schema, it encodes the data into Avro, JSONSchema or Protocol Buffer.
// Then it adds the wire format prefix and returns the binary to be used in key or value.
// If no schema is passed, it treats the data as a byte array, a string or a JSON object without
// a JSONSchema. Then, it returns the data as a byte array.
// nolint: funlen
func (k *Kafka) serialize(container *Container) []byte {
	return k.serializeWithRegistry(container, nil)
}

func (k *Kafka) serializeWithRegistry(container *Container, registry *schemaRegistryState) []byte {
	if container == nil {
		throwConfigError(k.vu.Runtime(), newMissingConfigError("serialize metadata"))
		return nil
	}

	if container.Schema == nil {
		if container.SchemaType == Protobuf {
			common.Throw(k.vu.Runtime(), newMissingConfigError("schema metadata"))
			return nil
		}

		// we are dealing with a byte array, a string or a JSON object without a JSONSchema
		serde, err := GetSerdes(container.SchemaType)
		if err != nil {
			common.Throw(k.vu.Runtime(), err)
			return nil
		}

		data, err := serde.Serialize(container.Data, nil)
		if err != nil {
			common.Throw(k.vu.Runtime(), err)
			return nil
		}
		return data
	}
	// we are dealing with binary data to be encoded with Avro, JSONSchema or Protocol Buffer

	// If the schema was unmarshaled from JSON, it won't have the resolver function.
	// Try to get the schema from cache if caching is enabled,
	// as the cached version will have the resolver.
	if container.Schema != nil {
		cache := k.schemaCache
		client := k.currentSchemaRegistry
		if registry != nil {
			cache = registry.cache
			client = registry.client
		}

		if container.Schema.EnableCaching {
			if cachedSchema, ok := cache[container.Schema.Subject]; ok {
				container.Schema = cachedSchema
			}
		}

		// If schema doesn't have a resolver but has references,
		// create one using the stored schema registry client
		if container.Schema.resolver == nil && len(container.Schema.References) > 0 {
			if client != nil {
				container.Schema.resolver = k.createResolverWithCache(
					client, cache, container.Schema.EnableCaching)
			}
		}
	}

	switch container.SchemaType {
	case Avro, Json:
		serde, err := GetSerdes(container.SchemaType)
		if err != nil {
			common.Throw(k.vu.Runtime(), err)
			return nil
		}

		bytesData, err := serde.Serialize(container.Data, container.Schema)
		if err != nil {
			common.Throw(k.vu.Runtime(), err)
			return nil
		}

		return k.encodeWireFormat(bytesData, container.Schema.ID)
	case Bytes, String:
		common.Throw(k.vu.Runtime(), ErrUnsupportedOperation)
		return nil
	case Protobuf:
		return k.serializeProtobuf(container)
	default:
		common.Throw(k.vu.Runtime(), ErrUnsupportedOperation)
		return nil
	}
}

// deserialize checks whether the incoming data has a schema or not.
// If the data has a schema, it removes the wire format prefix and decodes the data into JSON
// using Avro, JSONSchema or Protocol Buffer schemas. It returns the decoded data as JSON object.
// If no schema is passed, it treats the data as a byte array, a string or a JSON object without
// a JSONSchema. Then, it returns the data based on how it can decode it.
// nolint: funlen
func (k *Kafka) deserialize(container *Container) any {
	return k.deserializeWithRegistry(container, nil)
}

func (k *Kafka) deserializeWithRegistry(container *Container, registry *schemaRegistryState) any {
	if container == nil {
		throwConfigError(k.vu.Runtime(), newMissingConfigError("deserialize metadata"))
		return nil
	}

	if container.Schema == nil {
		if container.SchemaType == Protobuf {
			common.Throw(k.vu.Runtime(), newMissingConfigError("schema metadata"))
			return nil
		}

		// we are dealing with a byte array, a string or a JSON object without a JSONSchema
		serde, err := GetSerdes(container.SchemaType)
		if err != nil {
			common.Throw(k.vu.Runtime(), err)
			return nil
		}

		switch data := container.Data.(type) {
		case []byte:
			switch container.SchemaType {
			case String:
				return string(data)
			case Bytes:
				return data
			case Avro, Json:
				if isJSON(data) {
					js, err := toMap(data)
					if err != nil {
						common.Throw(k.vu.Runtime(), err)
						return nil
					}
					return js
				}
				return data
			case Protobuf:
				return data
			default:
				return data
			}
		case string:
			if isBase64Encoded(data) {
				decodedData, err := base64ToBytes(data)
				if err != nil {
					common.Throw(k.vu.Runtime(), err)
					return nil
				}
				result, err := serde.Deserialize(decodedData, nil)
				if err != nil {
					common.Throw(k.vu.Runtime(), err)
					return nil
				}
				return result
			}
			return []byte(data)
		default:
			return container.Data
		}
	} else {
		// we are dealing with binary data to be encoded with Avro, JSONSchema or Protocol Buffer
		runtime := k.vu.Runtime()

		var jsonBytes []byte

		switch data := container.Data.(type) {
		case []byte:
			jsonBytes = data
		case string:
			// Decode the data into JSON bytes from base64-encoded data
			if isBase64Encoded(data) {
				decodedData, err := base64ToBytes(data)
				if err != nil {
					common.Throw(k.vu.Runtime(), err)
					return nil
				}
				jsonBytes = decodedData
			}
		}

		// Remove wire format prefix
		jsonBytes = k.decodeWireFormat(jsonBytes)

		// If the schema was unmarshaled from JSON, it won't have the resolver function.
		// Try to get the schema from cache if caching is enabled,
		// as the cached version will have the resolver.
		if container.Schema != nil {
			cache := k.schemaCache
			client := k.currentSchemaRegistry
			if registry != nil {
				cache = registry.cache
				client = registry.client
			}

			if container.Schema.EnableCaching {
				if cachedSchema, ok := cache[container.Schema.Subject]; ok {
					// Use the cached schema which has the resolver set
					container.Schema = cachedSchema
				}
			}

			// If schema doesn't have a resolver but has references,
			// create one using the stored schema registry client
			if container.Schema.resolver == nil && len(container.Schema.References) > 0 {
				if client != nil {
					container.Schema.resolver = k.createResolverWithCache(
						client, cache, container.Schema.EnableCaching)
				}
			}
		}

		switch container.SchemaType {
		case Avro, Json:
			serde, err := GetSerdes(container.SchemaType)
			if err != nil {
				common.Throw(k.vu.Runtime(), err)
				return nil
			}

			deserialized, err := serde.Deserialize(jsonBytes, container.Schema)
			if err != nil {
				common.Throw(k.vu.Runtime(), err)
				return nil
			}

			if jsonObj, ok := deserialized.(map[string]any); ok {
				return jsonObj
			}
			common.Throw(k.vu.Runtime(), ErrInvalidDataType)
			return nil
		case Bytes, String:
			common.Throw(runtime, ErrUnsupportedOperation)
			return nil
		case Protobuf:
			return k.deserializeProtobuf(container)
		default:
			common.Throw(runtime, ErrUnsupportedOperation)
			return nil
		}
	}
}
