package kafka

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var avroSchemaForSRTests = `{"type":"record","name":"Schema","fields":[{"name":"field","type":"string"}]}`

func newMockSchemaRegistryClientObject(t *testing.T, test *kafkaTest, name string) *sobek.Object {
	t.Helper()

	client := test.module.schemaRegistryClientClass(sobek.ConstructorCall{
		Arguments: []sobek.Value{
			test.module.vu.Runtime().ToValue(
				map[string]any{
					"url": "mock://" + name,
				},
			),
		},
	})
	require.NotNil(t, client)

	return client
}

func schemaRegistryMethod(
	t *testing.T,
	client *sobek.Object,
	name string,
) func(sobek.FunctionCall) sobek.Value {
	t.Helper()

	method, ok := client.Get(name).Export().(func(sobek.FunctionCall) sobek.Value)
	require.True(t, ok)

	return method
}

// TestDecodeWireFormat tests the decoding of a wire-formatted message.
func TestDecodeWireFormat(t *testing.T) {
	test := getTestModuleInstance(t)

	encoded := []byte{0, 1, 2, 3, 4, 5}
	decoded := []byte{5}

	result := test.module.decodeWireFormat(encoded)
	assert.Equal(t, decoded, result)
}

// TestDecodeWireFormatFails tests the decoding of a wire-formatted message and
// fails because the message is too short.
func TestDecodeWireFormatFails(t *testing.T) {
	test := getTestModuleInstance(t)

	encoded := []byte{0, 1, 2, 3} // too short

	defer func() {
		err := recover()
		errObj, ok := err.(*sobek.Object)
		assert.True(t, ok)
		assert.Equal(t,
			errObj.ToString().String(),
			GoErrorPrefix+"Invalid message: message too short to contain schema id.")
	}()

	test.module.decodeWireFormat(encoded)
}

// TestEncodeWireFormat tests the encoding of a message and adding wire-format to it.
func TestEncodeWireFormat(t *testing.T) {
	test := getTestModuleInstance(t)

	data := []byte{6}
	schemaID := 5
	encoded := []byte{0, 0, 0, 0, 5, 6}

	result := test.module.encodeWireFormat(data, schemaID)
	assert.Equal(t, encoded, result)
}

// TestSchemaRegistryClient tests the creation of a SchemaRegistryClient instance
// with the given configuration.
func TestSchemaRegistryClient(t *testing.T) {
	test := getTestModuleInstance(t)

	srConfig := SchemaRegistryConfig{
		URL: "http://localhost:8081",
		BasicAuth: BasicAuth{
			Username: "username",
			Password: "password",
		},
	}
	srClient := test.module.schemaRegistryClient(&srConfig)
	assert.NotNil(t, srClient)
}

// TestSchemaRegistryClientWithTLSConfig tests the creation of a SchemaRegistryClient instance
// with the given configuration along with TLS configuration.
func TestSchemaRegistryClientWithTLSConfig(t *testing.T) {
	test := getTestModuleInstance(t)

	srConfig := SchemaRegistryConfig{
		URL: "http://localhost:8081",
		BasicAuth: BasicAuth{
			Username: "username",
			Password: "password",
		},
		TLS: TLSConfig{
			ClientCertPem: "testdata/fixtures/client.cer",
			ClientKeyPem:  "testdata/fixtures/client.pem",
			ServerCaPem:   "testdata/fixtures/caroot.cer",
		},
	}
	srClient := test.module.schemaRegistryClient(&srConfig)
	assert.NotNil(t, srClient)
}

// TestNewSchemaRegistryTransportWithTLS verifies that the transport returned when a
// TLSClientConfig is provided has TLSNextProto cleared. Cloning http.DefaultTransport
// copies TLSNextProto (the HTTP/2 protocol map); without clearing it, setting
// TLSClientConfig on the clone corrupts the HTTP/2 connection pool and causes EOF
// and gzip decoding failures on HTTPS schema registry endpoints (regression in v2).
func TestNewSchemaRegistryTransportWithTLS(t *testing.T) {
	tlsConfig := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // test only
	transport := newSchemaRegistryTransport(tlsConfig)

	httpTransport, ok := transport.(*http.Transport)
	require.True(t, ok, "expected *http.Transport")
	assert.Empty(t, httpTransport.TLSNextProto,
		"TLSNextProto must be empty to prevent HTTP/2 negotiation "+
			"which bypasses gzipTransport when TLSClientConfig is set")
	assert.Equal(t, tlsConfig, httpTransport.TLSClientConfig)
}

// TestNewSchemaRegistryTransportWithoutTLS verifies that http.DefaultTransport is
// returned as-is when no TLSClientConfig is provided.
func TestNewSchemaRegistryTransportWithoutTLS(t *testing.T) {
	transport := newSchemaRegistryTransport(nil)
	assert.Equal(t, http.DefaultTransport, transport)
}

// TestGetLatestSchemaFails tests getting the latest schema and fails because
// the configuration is invalid.
func TestGetLatestSchemaFails(t *testing.T) {
	test := getTestModuleInstance(t)

	srConfig := SchemaRegistryConfig{
		URL: "http://localhost:8081",
		BasicAuth: BasicAuth{
			Username: "username",
			Password: "password",
		},
	}
	srClient := test.module.schemaRegistryClient(&srConfig)
	assert.Panics(t, func() {
		schema := test.module.getSchema(srClient, &Schema{
			Subject: "no-such-subject",
			Version: 0,
		})
		assert.Equal(t, schema, nil)
	})
}

// TestGetSchemaFails tests getting the first version of the schema and fails because
// the configuration is invalid.
func TestGetSchemaFails(t *testing.T) {
	test := getTestModuleInstance(t)

	srConfig := SchemaRegistryConfig{
		URL: "http://localhost:8081",
		BasicAuth: BasicAuth{
			Username: "username",
			Password: "password",
		},
	}
	srClient := test.module.schemaRegistryClient(&srConfig)
	assert.Panics(t, func() {
		schema := test.module.getSchema(srClient, &Schema{
			Subject: "no-such-subject",
			Version: 0,
		})
		assert.Equal(t, schema, nil)
	})
}

// TestCreateSchemaFails tests creating the schema and fails because the
// configuration is invalid.
func TestCreateSchemaFails(t *testing.T) {
	test := getTestModuleInstance(t)

	srConfig := SchemaRegistryConfig{
		URL: "http://localhost:8081",
		BasicAuth: BasicAuth{
			Username: "username",
			Password: "password",
		},
	}
	srClient := test.module.schemaRegistryClient(&srConfig)
	assert.Panics(t, func() {
		schema := test.module.getSchema(srClient, &Schema{
			Subject: "no-such-subject",
			Version: 0,
		})
		assert.Equal(t, schema, nil)
	})
}

func TestGetSubjectNameFailsIfInvalidSchema(t *testing.T) {
	test := getTestModuleInstance(t)

	assert.Panics(t, func() {
		subjectName := test.module.getSubjectName(&SubjectNameConfig{
			Schema:              `Bad Schema`,
			Topic:               "test-topic",
			SubjectNameStrategy: RecordNameStrategy,
			Element:             Value,
		})
		assert.Equal(t, subjectName, "")
	})
}

func TestGetSubjectNameFailsIfSubjectNameStrategyUnknown(t *testing.T) {
	test := getTestModuleInstance(t)

	assert.Panics(t, func() {
		subjectName := test.module.getSubjectName(&SubjectNameConfig{
			Schema:              avroSchemaForSRTests,
			Topic:               "test-topic",
			SubjectNameStrategy: "Unknown",
			Element:             Value,
		})
		assert.Equal(t, subjectName, "")
	})
}

func TestGetSubjectNameCanUseDefaultSubjectNameStrategy(t *testing.T) {
	test := getTestModuleInstance(t)

	for _, element := range []Element{Key, Value} {
		subjectName := test.module.getSubjectName(&SubjectNameConfig{
			Schema:              avroSchemaForSRTests,
			Topic:               "test-topic",
			SubjectNameStrategy: "",
			Element:             element,
		})
		assert.Equal(t, "test-topic-"+string(element), subjectName)
	}
}

func TestGetSubjectNameCanUseTopicNameStrategy(t *testing.T) {
	test := getTestModuleInstance(t)

	for _, element := range []Element{Key, Value} {
		subjectName := test.module.getSubjectName(&SubjectNameConfig{
			Schema:              avroSchemaForSRTests,
			Topic:               "test-topic",
			SubjectNameStrategy: TopicNameStrategy,
			Element:             element,
		})
		assert.Equal(t, "test-topic-"+string(element), subjectName)
	}
}

func TestGetSubjectNameCanUseTopicRecordNameStrategyWithNamespace(t *testing.T) {
	test := getTestModuleInstance(t)

	avroSchema := `{
		"type":"record",
		"namespace":"com.example.person",
		"name":"Schema",
		"fields":[{"name":"field","type":"string"}]}`
	subjectName := test.module.getSubjectName(&SubjectNameConfig{
		Schema:              avroSchema,
		Topic:               "test-topic",
		SubjectNameStrategy: TopicRecordNameStrategy,
		Element:             Value,
	})
	assert.Equal(t, "test-topic-com.example.person.Schema", subjectName)
}

func TestGetSubjectNameCanUseTopicRecordNameStrategyWithoutNamespace(t *testing.T) {
	test := getTestModuleInstance(t)

	subjectName := test.module.getSubjectName(&SubjectNameConfig{
		Schema:              avroSchemaForSRTests,
		Topic:               "test-topic",
		SubjectNameStrategy: TopicRecordNameStrategy,
		Element:             Value,
	})
	assert.Equal(t, "test-topic-Schema", subjectName)
}

func TestGetSubjectNameCanUseRecordNameStrategyWithoutNamespace(t *testing.T) {
	test := getTestModuleInstance(t)

	subject := test.module.getSubjectName(&SubjectNameConfig{
		Schema:              avroSchemaForSRTests,
		Topic:               "test-topic",
		SubjectNameStrategy: RecordNameStrategy,
		Element:             Value,
	})
	assert.Equal(t, "Schema", subject)
}

func TestGetSubjectNameCanUseRecordNameStrategyWithNamespace(t *testing.T) {
	test := getTestModuleInstance(t)

	avroSchema := `{
		"type":"record",
		"namespace":"com.example.person",
		"name":"Schema",
		"fields":[{"name":"field","type":"string"}]}`
	subjectName := test.module.getSubjectName(&SubjectNameConfig{
		Schema:              avroSchema,
		Topic:               "test-topic",
		SubjectNameStrategy: RecordNameStrategy,
		Element:             Value,
	})
	assert.Equal(t, "com.example.person.Schema", subjectName)
}

func TestGetSubjectNameUsesMessageNameForProtobufStrategies(t *testing.T) {
	test := getTestModuleInstance(t)

	subject := test.module.getSubjectName(&SubjectNameConfig{
		Schema:              `syntax = "proto3"; message User { string name = 1; }`,
		Topic:               "test-topic",
		SubjectNameStrategy: RecordNameStrategy,
		Element:             Value,
		MessageName:         "com.example.User",
	})
	assert.Equal(t, "com.example.User", subject)

	subject = test.module.getSubjectName(&SubjectNameConfig{
		Schema:              `syntax = "proto3"; message User { string name = 1; }`,
		Topic:               "test-topic",
		SubjectNameStrategy: TopicRecordNameStrategy,
		Element:             Value,
		MessageName:         "com.example.User",
	})
	assert.Equal(t, "test-topic-com.example.User", subject)
}

// TestSchemaRegistryClientClass tests the schema registry client class.
func TestSchemaRegistryClientClass(t *testing.T) {
	test := getTestModuleInstance(t)
	avroSchema := `{"type":"record","name":"Schema","namespace":"com.example.person",` +
		`"fields":[{"name":"field","type":"string"}]}`

	test.moveToVUCode()
	assert.NotPanics(t, func() {
		client := newMockSchemaRegistryClientObject(t, test, "schema-registry-client-class")

		createSchema := schemaRegistryMethod(t, client, "createSchema")
		newSchema := createSchema(sobek.FunctionCall{
			Arguments: []sobek.Value{
				test.module.vu.Runtime().ToValue(
					map[string]any{
						"subject":    "test-subject",
						"schema":     avroSchema,
						"schemaType": Avro,
					},
				),
			},
		}).Export().(*Schema)
		assert.Equal(t, "test-subject", newSchema.Subject)
		assert.Equal(t, 1, newSchema.Version)

		getSchema := schemaRegistryMethod(t, client, "getSchema")
		currentSchema := getSchema(sobek.FunctionCall{
			Arguments: []sobek.Value{
				test.module.vu.Runtime().ToValue(
					map[string]any{
						"subject": "test-subject",
						"version": 0,
					},
				),
			},
		}).Export().(*Schema)
		assert.Equal(t, "test-subject", currentSchema.Subject)
		assert.Equal(t, 1, currentSchema.Version)
		assert.Equal(t, avroSchema, currentSchema.Schema)

		getSubjectName := schemaRegistryMethod(t, client, "getSubjectName")
		subjectName := getSubjectName(sobek.FunctionCall{
			Arguments: []sobek.Value{
				test.module.vu.Runtime().ToValue(
					map[string]any{
						"schema":              avroSchema,
						"topic":               "test-topic",
						"subjectNameStrategy": TopicRecordNameStrategy,
						"element":             Value,
					},
				),
			},
		}).Export().(string)
		assert.Equal(t, "test-topic-com.example.person.Schema", subjectName)

		serialize := schemaRegistryMethod(t, client, "serialize")
		serialized := serialize(sobek.FunctionCall{
			Arguments: []sobek.Value{
				test.module.vu.Runtime().ToValue(
					map[string]any{
						"data":       map[string]any{"field": "value"},
						"schema":     currentSchema,
						"schemaType": Avro,
					},
				),
			},
		}).Export().([]byte)
		assert.NotNil(t, serialized)

		deserialize := schemaRegistryMethod(t, client, "deserialize")
		deserialized := deserialize(sobek.FunctionCall{
			Arguments: []sobek.Value{
				test.module.vu.Runtime().ToValue(
					map[string]any{
						"data":       serialized,
						"schema":     currentSchema,
						"schemaType": Avro,
					},
				),
			},
		}).Export().(map[string]any)
		assert.Equal(t, "value", deserialized["field"])
	})
}

func TestSchemaRegistryClientClassSupportsMultipleSchemaVersions(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	client := newMockSchemaRegistryClientObject(t, test, "schema-registry-multiple-versions")
	createSchema := schemaRegistryMethod(t, client, "createSchema")
	getSchema := schemaRegistryMethod(t, client, "getSchema")
	serialize := schemaRegistryMethod(t, client, "serialize")
	deserialize := schemaRegistryMethod(t, client, "deserialize")

	jsonSchemaV1 := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		},
		"required": ["name"]
	}`
	jsonSchemaV2 := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "number"}
		},
		"required": ["name", "age"]
	}`

	firstVersion := createSchema(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.module.vu.Runtime().ToValue(
				map[string]any{
					"subject":    "test-json-subject",
					"schema":     jsonSchemaV1,
					"schemaType": Json,
				},
			),
		},
	}).Export().(*Schema)
	secondVersion := createSchema(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.module.vu.Runtime().ToValue(
				map[string]any{
					"subject":    "test-json-subject",
					"schema":     jsonSchemaV2,
					"schemaType": Json,
				},
			),
		},
	}).Export().(*Schema)

	assert.Equal(t, 1, firstVersion.Version)
	assert.Equal(t, 2, secondVersion.Version)

	latestSchema := getSchema(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.module.vu.Runtime().ToValue(
				map[string]any{
					"subject": "test-json-subject",
					"version": 0,
				},
			),
		},
	}).Export().(*Schema)
	firstSchema := getSchema(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.module.vu.Runtime().ToValue(
				map[string]any{
					"subject": "test-json-subject",
					"version": 1,
				},
			),
		},
	}).Export().(*Schema)

	assert.Equal(t, 2, latestSchema.Version)
	assert.Equal(t, jsonSchemaV2, latestSchema.Schema)
	assert.Equal(t, 1, firstSchema.Version)
	assert.Equal(t, jsonSchemaV1, firstSchema.Schema)

	serialized := serialize(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.module.vu.Runtime().ToValue(
				map[string]any{
					"data": map[string]any{
						"name": "Mostafa",
						"age":  33.0,
					},
					"schema":     latestSchema,
					"schemaType": Json,
				},
			),
		},
	}).Export().([]byte)

	deserialized := deserialize(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.module.vu.Runtime().ToValue(
				map[string]any{
					"data":       serialized,
					"schema":     latestSchema,
					"schemaType": Json,
				},
			),
		},
	}).Export().(map[string]any)

	assert.Equal(t, "Mostafa", deserialized["name"])
	assert.Equal(t, 33.0, deserialized["age"])
}

func TestSchemaRegistryClientClassSupportsReferencedSchemas(t *testing.T) {
	test := getTestModuleInstance(t)
	test.moveToVUCode()

	client := newMockSchemaRegistryClientObject(t, test, "schema-registry-references")
	createSchema := schemaRegistryMethod(t, client, "createSchema")
	getSchema := schemaRegistryMethod(t, client, "getSchema")
	serialize := schemaRegistryMethod(t, client, "serialize")
	deserialize := schemaRegistryMethod(t, client, "deserialize")

	addressSchema := `{
		"type": "record",
		"name": "Address",
		"namespace": "com.example",
		"fields": [{"name": "street", "type": "string"}]
	}`
	userSchema := `{
		"type": "record",
		"name": "User",
		"namespace": "com.example",
		"fields": [
			{"name": "name", "type": "string"},
			{"name": "address", "type": "com.example.Address"}
		]
	}`

	address := createSchema(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.module.vu.Runtime().ToValue(
				map[string]any{
					"subject":    "com.example.Address",
					"schema":     addressSchema,
					"schemaType": Avro,
				},
			),
		},
	}).Export().(*Schema)

	user := createSchema(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.module.vu.Runtime().ToValue(
				map[string]any{
					"subject":    "com.example.User",
					"schema":     userSchema,
					"schemaType": Avro,
					"references": []map[string]any{
						{
							"name":    "com.example.Address",
							"subject": "com.example.Address",
							"version": address.Version,
						},
					},
				},
			),
		},
	}).Export().(*Schema)

	assert.Equal(t, 1, address.Version)
	assert.Equal(t, 1, user.Version)
	require.Len(t, user.References, 1)
	assert.Equal(t, "com.example.Address", user.References[0].Name)
	assert.Equal(t, 1, user.References[0].Version)

	currentUser := getSchema(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.module.vu.Runtime().ToValue(
				map[string]any{
					"subject": "com.example.User",
					"version": 0,
				},
			),
		},
	}).Export().(*Schema)
	require.Len(t, currentUser.References, 1)
	assert.Equal(t, 1, currentUser.Version)

	serialized := serialize(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.module.vu.Runtime().ToValue(
				map[string]any{
					"data": map[string]any{
						"name": "Mostafa",
						"address": map[string]any{
							"street": "Main Street",
						},
					},
					"schema":     currentUser,
					"schemaType": Avro,
				},
			),
		},
	}).Export().([]byte)

	deserialized := deserialize(sobek.FunctionCall{
		Arguments: []sobek.Value{
			test.module.vu.Runtime().ToValue(
				map[string]any{
					"data":       serialized,
					"schema":     currentUser,
					"schemaType": Avro,
				},
			),
		},
	}).Export().(map[string]any)

	assert.Equal(t, "Mostafa", deserialized["name"])
	addressValue, ok := deserialized["address"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Main Street", addressValue["street"])
}
