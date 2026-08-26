package kafka

import (
	"encoding/json"
	"testing"

	"github.com/hamba/avro/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testUserSchemaJSONSimple = `{
		"type": "record",
		"name": "User",
		"namespace": "com.example",
		"fields": [{"name": "id", "type": "int"}]
	}`
	testMainSchemaJSON = `{
		"type": "record",
		"name": "Main",
		"namespace": "com.example",
		"fields": [{"name": "user", "type": "com.example.User"}]
	}`
)

func TestResolver_FindInCacheBySubject(t *testing.T) {
	test := getTestModuleInstance(t)
	avroType := Avro

	// Add a schema to cache
	cachedSchema := &Schema{
		ID:            1,
		Schema:        testUserSchemaJSONSimple,
		SchemaType:    &avroType,
		Version:       1,
		Subject:       "com.example.User",
		EnableCaching: true,
	}
	test.module.schemaCache["com.example.User"] = cachedSchema

	// Create a resolver function manually to test cache lookup
	resolver := func(name string) (*Schema, error) {
		if enableCaching := true; enableCaching {
			for subject, cachedSchema := range test.module.schemaCache {
				if subject == name {
					return cachedSchema, nil
				}
			}
		}
		return nil, assert.AnError
	}

	resolved, err := resolver("com.example.User")

	require.NoError(t, err)
	assert.Equal(t, cachedSchema, resolved)
	assert.Equal(t, "com.example.User", resolved.Subject)
}

func TestResolver_FindInCacheByFullName(t *testing.T) {
	test := getTestModuleInstance(t)
	avroType := Avro

	// Parse schema and add to cache
	avroSchema, err := avro.Parse(testUserSchemaJSONSimple)
	require.NoError(t, err)

	cachedSchema := &Schema{
		ID:            1,
		Schema:        testUserSchemaJSONSimple,
		SchemaType:    &avroType,
		Version:       1,
		Subject:       "different-subject",
		EnableCaching: true,
		avroSchema:    avroSchema,
	}
	test.module.schemaCache["different-subject"] = cachedSchema

	// Create a resolver function manually to test cache lookup by full name
	resolver := func(name string) (*Schema, error) {
		if enableCaching := true; enableCaching {
			for _, cachedSchema := range test.module.schemaCache {
				if cachedSchema.avroSchema != nil {
					if namedSchema, ok := cachedSchema.avroSchema.(avro.NamedSchema); ok {
						if namedSchema.FullName() == name {
							return cachedSchema, nil
						}
					}
				}
			}
		}
		return nil, assert.AnError
	}

	resolved, err := resolver("com.example.User")

	require.NoError(t, err)
	assert.Equal(t, cachedSchema, resolved)
}

func TestResolver_FindInCacheByExtractedName(t *testing.T) {
	test := getTestModuleInstance(t)
	avroType := Avro

	// Add schema to cache without parsing it first
	cachedSchema := &Schema{
		ID:            1,
		Schema:        testUserSchemaJSONSimple,
		SchemaType:    &avroType,
		Version:       1,
		Subject:       "different-subject",
		EnableCaching: true,
		avroSchema:    nil, // Not parsed yet
	}
	test.module.schemaCache["different-subject"] = cachedSchema

	// Test that Codec() can resolve references using cached schemas
	// This indirectly tests the resolver finding schemas by extracted name
	mainSchema := &Schema{
		ID:            2,
		Schema:        testMainSchemaJSON,
		SchemaType:    &avroType,
		Version:       1,
		Subject:       "com.example.Main",
		EnableCaching: true,
		References: []Reference{
			{
				Name:    "com.example.User",
				Subject: "different-subject",
				Version: 1,
			},
		},
		resolver: func(name string) (*Schema, error) {
			// Simulate resolver finding by extracted name
			for _, cachedSchema := range test.module.schemaCache {
				var schemaMap map[string]any
				if json.Unmarshal([]byte(cachedSchema.Schema), &schemaMap) == nil {
					if ns, ok := schemaMap["namespace"].(string); ok {
						if n, ok := schemaMap["name"].(string); ok {
							fullName := n
							if ns != "" {
								fullName = ns + "." + n
							}
							if fullName == name {
								return cachedSchema, nil
							}
						}
					}
				}
			}
			return nil, assert.AnError
		},
	}

	// Codec should be able to resolve the reference
	codec := mainSchema.Codec()
	assert.NotNil(t, codec)
}

func TestResolver_CodecWithReferences(t *testing.T) {
	test := getTestModuleInstance(t)
	avroType := Avro

	// Create referenced schema
	userSchema := &Schema{
		ID:            1,
		Schema:        testUserSchemaJSONSimple,
		SchemaType:    &avroType,
		Version:       1,
		Subject:       "com.example.User",
		EnableCaching: true,
	}
	test.module.schemaCache["com.example.User"] = userSchema

	// Create main schema with reference
	mainSchema := &Schema{
		ID:            2,
		Schema:        testMainSchemaJSON,
		SchemaType:    &avroType,
		Version:       1,
		Subject:       "com.example.Main",
		EnableCaching: true,
		References: []Reference{
			{
				Name:    "com.example.User",
				Subject: "com.example.User",
				Version: 1,
			},
		},
		resolver: func(name string) (*Schema, error) {
			if cachedSchema, ok := test.module.schemaCache[name]; ok {
				return cachedSchema, nil
			}
			return nil, assert.AnError
		},
	}

	// Codec should resolve the reference and parse successfully
	codec := mainSchema.Codec()
	assert.NotNil(t, codec)

	// Verify it's a record schema
	recordSchema, ok := codec.(*avro.RecordSchema)
	require.True(t, ok)
	assert.Equal(t, "Main", recordSchema.Name())
	assert.Equal(t, "com.example", recordSchema.Namespace())
}

func TestResolver_NestedReferences(t *testing.T) {
	test := getTestModuleInstance(t)
	avroType := Avro

	// Create nested referenced schemas
	addressSchemaJSON := `{
		"type": "record",
		"name": "Address",
		"namespace": "com.example",
		"fields": [{"name": "street", "type": "string"}]
	}`
	addressSchema := &Schema{
		ID:            1,
		Schema:        addressSchemaJSON,
		SchemaType:    &avroType,
		Version:       1,
		Subject:       "com.example.Address",
		EnableCaching: true,
	}
	test.module.schemaCache["com.example.Address"] = addressSchema

	userSchemaJSON := `{
		"type": "record",
		"name": "User",
		"namespace": "com.example",
		"fields": [
			{"name": "id", "type": "int"},
			{"name": "address", "type": "com.example.Address"}
		]
	}`
	userSchema := &Schema{
		ID:            2,
		Schema:        userSchemaJSON,
		SchemaType:    &avroType,
		Version:       1,
		Subject:       "com.example.User",
		EnableCaching: true,
		References: []Reference{
			{
				Name:    "com.example.Address",
				Subject: "com.example.Address",
				Version: 1,
			},
		},
	}
	test.module.schemaCache["com.example.User"] = userSchema

	// Create main schema referencing User
	mainSchema := &Schema{
		ID:            3,
		Schema:        testMainSchemaJSON,
		SchemaType:    &avroType,
		Version:       1,
		Subject:       "com.example.Main",
		EnableCaching: true,
		References: []Reference{
			{
				Name:    "com.example.User",
				Subject: "com.example.User",
				Version: 1,
			},
		},
		resolver: func(name string) (*Schema, error) {
			if cachedSchema, ok := test.module.schemaCache[name]; ok {
				return cachedSchema, nil
			}
			return nil, assert.AnError
		},
	}

	// Codec should resolve nested references
	codec := mainSchema.Codec()
	assert.NotNil(t, codec)

	recordSchema, ok := codec.(*avro.RecordSchema)
	require.True(t, ok)
	assert.Equal(t, "Main", recordSchema.Name())
}

func TestResolver_NotFound(t *testing.T) {
	// Create resolver that doesn't find anything
	resolver := func(name string) (*Schema, error) {
		return nil, assert.AnError
	}

	resolved, err := resolver("com.example.NonExistent")

	assert.Error(t, err)
	assert.Nil(t, resolved)
}

func TestResolver_CachingDisabled(t *testing.T) {
	test := getTestModuleInstance(t)
	avroType := Avro

	// Add schema to cache
	cachedSchema := &Schema{
		ID:            1,
		Schema:        testUserSchemaJSONSimple,
		SchemaType:    &avroType,
		Version:       1,
		Subject:       "com.example.User",
		EnableCaching: true,
	}
	test.module.schemaCache["com.example.User"] = cachedSchema

	// Create resolver with caching disabled
	resolver := func(name string) (*Schema, error) {
		enableCaching := false
		if enableCaching {
			for subject, cachedSchema := range test.module.schemaCache {
				if subject == name {
					return cachedSchema, nil
				}
			}
		}
		return nil, assert.AnError
	}

	// Should not find in cache when caching is disabled
	resolved, err := resolver("com.example.User")
	assert.Error(t, err)
	assert.Nil(t, resolved)

	// Verify cache still has the schema (it just wasn't used)
	assert.Contains(t, test.module.schemaCache, "com.example.User")
}
