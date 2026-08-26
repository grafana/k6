package kafka

import (
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	cschemaregistry "github.com/confluentinc/confluent-kafka-go/v2/schemaregistry"
	"github.com/grafana/sobek"
	"github.com/hamba/avro/v2"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/sirupsen/logrus"
	"go.k6.io/k6/js/common"
)

type Element string

const (
	Key                Element = "key"
	Value              Element = "value"
	MagicPrefixSize    int     = 5
	ConcurrentRequests int     = 16
)

type BasicAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type SchemaRegistryConfig struct {
	EnableCaching bool      `json:"enableCaching"`
	URL           string    `json:"url"`
	BasicAuth     BasicAuth `json:"basicAuth"`
	TLS           TLSConfig `json:"tls"`
}

const (
	TopicNameStrategy       string = "TopicNameStrategy"
	RecordNameStrategy      string = "RecordNameStrategy"
	TopicRecordNameStrategy string = "TopicRecordNameStrategy"
)

// Schema is a wrapper around the schema registry schema.
// The Codec() and JsonSchema() methods will return the respective codecs (duck-typing).
type Schema struct {
	EnableCaching bool              `json:"enableCaching"`
	ID            int               `json:"id"`
	Schema        string            `json:"schema"`
	SchemaType    *SchemaType       `json:"schemaType"`
	Version       int               `json:"version"`
	References    []Reference       `json:"references"`
	Subject       string            `json:"subject"`
	MessageName   string            `json:"messageName"`
	Dependencies  map[string]string `json:"dependencies"`
	avroSchema    avro.Schema
	jsonSchema    *jsonschema.Schema

	// resolver is a function that can resolve referenced schemas by name
	resolver func(name string) (*Schema, error)
}

type SubjectNameConfig struct {
	Schema              string  `json:"schema"`
	Topic               string  `json:"topic"`
	Element             Element `json:"element"`
	SubjectNameStrategy string  `json:"subjectNameStrategy"`
	MessageName         string  `json:"messageName"`
}

type WireFormat struct {
	SchemaID int    `json:"schemaId"`
	Data     []byte `json:"data"`
}

type schemaRegistryState struct {
	client SchemaRegistryClient
	cache  map[string]*Schema
}

func (k *Kafka) createResolverWithCache(
	client SchemaRegistryClient,
	cache map[string]*Schema,
	enableCaching bool,
) func(name string) (*Schema, error) {
	return func(name string) (*Schema, error) {
		// Try to find the referenced schema in the cache first
		if enableCaching {
			for subject, cachedSchema := range cache {
				// Check if cached schema matches the reference name by subject
				if subject == name {
					return cachedSchema, nil
				}
				// Also check by parsed schema full name
				if cachedSchema.avroSchema != nil {
					if namedSchema, ok := cachedSchema.avroSchema.(avro.NamedSchema); ok {
						if namedSchema.FullName() == name {
							return cachedSchema, nil
						}
					}
				}

				// Also check by extracting name from schema JSON
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
		}

		// Try to fetch by subject name (subject name often matches schema full name in RecordNameStrategy)
		refSchemaInfo, refErr := client.GetLatestSchema(name)
		if refErr == nil {
			refSchema := &Schema{
				EnableCaching: enableCaching,
				ID:            refSchemaInfo.ID(),
				Version:       refSchemaInfo.Version(),
				Schema:        refSchemaInfo.Schema(),
				SchemaType:    refSchemaInfo.SchemaType(),
				References:    refSchemaInfo.References(),
				Subject:       name,
				resolver:      k.createResolverWithCache(client, cache, enableCaching), // Recursive resolver setup
			}
			if refSchema.EnableCaching {
				cache[name] = refSchema
			}
			return refSchema, nil
		}

		// If GetLatestSchema failed, try to search through all cached schemas' references
		// This handles the case where a nested reference is specified in a parent schema's references
		if enableCaching {
			for _, cachedSchema := range cache {
				for _, ref := range cachedSchema.References {
					if ref.Name == name {
						// Fetch the referenced schema from the registry using the reference info
						refSchemaInfo, refErr := client.GetSchemaByVersion(ref.Subject, ref.Version)
						if refErr != nil {
							continue
						}
						refSchema := &Schema{
							EnableCaching: enableCaching,
							ID:            refSchemaInfo.ID(),
							Version:       refSchemaInfo.Version(),
							Schema:        refSchemaInfo.Schema(),
							SchemaType:    refSchemaInfo.SchemaType(),
							References:    refSchemaInfo.References(),
							Subject:       ref.Subject,
							resolver:      k.createResolverWithCache(client, cache, enableCaching),
						}
						if refSchema.EnableCaching {
							cache[ref.Subject] = refSchema
						}
						return refSchema, nil
					}
				}
			}
		}

		return nil, fmt.Errorf("%w: %s (GetLatestSchema error: %w)", ErrReferenceNotFound, name, refErr)
	}
}

// Codec ensures access to parsed Avro Schema.
// Will try to initialize a new one if it hasn't been initialized before.
// Will return nil if it can't initialize a schema from the schema string.
//
//nolint:maintidx
func (s *Schema) Codec() avro.Schema {
	if s.avroSchema != nil {
		return s.avroSchema
	}

	var (
		schema avro.Schema
		err    error
		cache  *avro.SchemaCache
	)

	// Extract namespace from schema JSON
	extractNamespace := func(schemaStr string) string {
		var schemaMap map[string]any
		if json.Unmarshal([]byte(schemaStr), &schemaMap) == nil {
			if ns, ok := schemaMap["namespace"]; ok {
				if ns, ok := ns.(string); ok {
					return ns
				}
			}
		}
		return ""
	}

	if len(s.References) > 0 && s.resolver != nil {
		// Build a schema cache with referenced schemas and all their nested references
		cache = &avro.SchemaCache{}
		var resolveErrors []error

		// Helper function to recursively resolve all nested references FIRST, then parse schemas
		var resolveAllReferences func(
			refSchema *Schema,
			refName string,
			visited map[string]bool,
		) error

		resolveAllReferences = func(
			refSchema *Schema,
			refName string,
			visited map[string]bool,
		) error {
			if refSchema == nil {
				return nil
			}

			// Mark as visited to avoid infinite recursion
			if visited == nil {
				visited = make(map[string]bool)
			}

			// Get the schema's full name for visited tracking
			var schemaFullName string
			if refSchema.avroSchema != nil {
				if namedSchema, ok := refSchema.avroSchema.(avro.NamedSchema); ok {
					schemaFullName = namedSchema.FullName()
				}
			}

			// If not parsed yet, try to extract from schema JSON
			if schemaFullName == "" {
				var schemaMap map[string]any
				if json.Unmarshal([]byte(refSchema.Schema), &schemaMap) == nil {
					if ns, ok := schemaMap["namespace"].(string); ok {
						if n, ok := schemaMap["name"].(string); ok {
							schemaFullName = n
							if ns != "" {
								schemaFullName = ns + "." + n
							}
						}
					}
				}
			}

			if schemaFullName != "" && visited[schemaFullName] {
				return nil // Already processed
			}

			if schemaFullName != "" {
				visited[schemaFullName] = true
			}

			// FIRST: Recursively resolve all nested references BEFORE parsing
			for _, nestedRef := range refSchema.References {
				if !visited[nestedRef.Name] {
					nestedRefSchema, nestedErr := s.resolver(nestedRef.Name)
					if nestedErr != nil {
						// Log but don't fail - might be optional
						continue
					}
					if nestedRefSchema != nil {
						if err := resolveAllReferences(nestedRefSchema, nestedRef.Name, visited); err != nil {
							// Log nested reference errors but continue
							resolveErrors = append(resolveErrors, fmt.Errorf("nested reference %s: %w", nestedRef.Name, err))
						}
					}
				}
			}

			// NOW parse the schema (all its dependencies should be resolved)
			// Parse directly with ParseWithCache using the shared cache so dependencies are available
			var (
				refAvroSchema avro.Schema
				parseErr      error
			)

			refNamespace := extractNamespace(refSchema.Schema)

			// Parse with the shared cache (which should have all dependencies)
			refAvroSchema, parseErr = avro.ParseWithCache(refSchema.Schema, refNamespace, cache)
			if parseErr != nil {
				return fmt.Errorf("failed to parse referenced schema %s: %w", refSchema.Subject, parseErr)
			}
			if refAvroSchema == nil {
				return fmt.Errorf("%w: %s", ErrFailedParseReferencedSchema, refSchema.Subject)
			}

			// Add to cache with multiple keys for different lookup scenarios
			if namedSchema, ok := refAvroSchema.(avro.NamedSchema); ok {
				fullName := namedSchema.FullName()
				namespace := namedSchema.Namespace()
				name := namedSchema.Name()

				// Add with full name (primary key)
				cache.Add(fullName, refAvroSchema)

				// Add with the reference name (as specified in the references array)
				if refName != "" && refName != fullName {
					cache.Add(refName, refAvroSchema)
				}

				// Add with namespace.name combination
				if namespace != "" && name != "" {
					namespaceName := namespace + "." + name
					if namespaceName != fullName && namespaceName != refName {
						cache.Add(namespaceName, refAvroSchema)
					}
					// Add with just the name - critical for namespace-relative lookups
					cache.Add(name, refAvroSchema)
				}
			} else {
				// For non-named schemas, just add with the reference name
				cache.Add(refSchema.Subject, refAvroSchema)
			}

			return nil
		}

		// Helper function to add a schema to cache (now that all references are resolved)
		addSchemaToCache := func(refSchema *Schema, refName string, visited map[string]bool) error {
			return resolveAllReferences(refSchema, refName, visited)
		}

		// Resolve all direct references
		for _, ref := range s.References {
			refSchema, resolveErr := s.resolver(ref.Name)
			if resolveErr != nil {
				resolveErrors = append(
					resolveErrors,
					fmt.Errorf("failed to resolve reference %s: %w", ref.Name, resolveErr),
				)
				continue
			}
			if err := addSchemaToCache(refSchema, ref.Name, nil); err != nil {
				resolveErrors = append(resolveErrors, err)
			}
		}

		// If we had errors resolving references, set error and skip parsing
		if len(resolveErrors) > 0 {
			err = fmt.Errorf("%w: %d reference(s): %v", ErrFailedResolveReferences, len(resolveErrors), resolveErrors)
		}
	}

	// Parse the schema (with or without cache depending on whether references were resolved)
	if err == nil {
		namespace := extractNamespace(s.Schema)
		if cache != nil {
			schema, err = avro.ParseWithCache(s.Schema, namespace, cache)
		} else {
			schema, err = avro.Parse(s.Schema)
		}
	}

	if err == nil {
		s.avroSchema = schema
	} else {
		logger.WithFields(logrus.Fields{
			"subject": s.Subject,
			"error":   err.Error(),
		}).Error("Failed to parse Avro schema")
	}

	return s.avroSchema
}

// JSONSchema ensures access to JsonSchema.
// Will try to initialize a new one if it hasn't been initialized before.
// Will return nil if it can't initialize a json schema from the schema.
func (s *Schema) JSONSchema() *jsonschema.Schema {
	if s.jsonSchema == nil {
		jsonSchema, err := jsonschema.CompileString("schema.json", s.Schema)
		if err == nil {
			s.jsonSchema = jsonSchema
		}
	}
	return s.jsonSchema
}

func (k *Kafka) schemaRegistryClientClass(call sobek.ConstructorCall) *sobek.Object {
	runtime := k.vu.Runtime()
	var configuration SchemaRegistryConfig
	var schemaRegistryClient SchemaRegistryClient
	registryState := &schemaRegistryState{cache: make(map[string]*Schema)}

	if len(call.Arguments) == 1 {
		decodeArgument(runtime, call.Argument(0), &configuration, "schema registry config")

		schemaRegistryClient = k.schemaRegistryClient(&configuration)
		registryState.client = schemaRegistryClient
	}

	schemaRegistryClientObject := runtime.NewObject()
	// This is the schema registry client object itself
	if err := schemaRegistryClientObject.Set("This", schemaRegistryClient); err != nil {
		common.Throw(runtime, err)
	}

	err := schemaRegistryClientObject.Set("getSchema", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			common.Throw(runtime, ErrNotEnoughArguments)
		}

		if registryState.client == nil {
			common.Throw(runtime, ErrNoSchemaRegistryClient)
		}

		var schema *Schema
		decodeArgument(runtime, call.Argument(0), &schema, "schema metadata")

		return runtime.ToValue(k.getSchemaWithCache(registryState.client, registryState.cache, schema))
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	err = schemaRegistryClientObject.Set("createSchema", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			common.Throw(runtime, ErrNotEnoughArguments)
		}

		if registryState.client == nil {
			common.Throw(runtime, ErrNoSchemaRegistryClient)
		}

		var schema *Schema
		decodeArgument(runtime, call.Argument(0), &schema, "schema metadata")

		return runtime.ToValue(k.createSchemaWithCache(registryState.client, registryState.cache, schema))
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	var subjectNameConfig *SubjectNameConfig
	err = schemaRegistryClientObject.Set("getSubjectName", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			common.Throw(runtime, ErrNotEnoughArguments)
		}

		decodeArgument(runtime, call.Argument(0), &subjectNameConfig, "subject name config")

		return runtime.ToValue(k.getSubjectName(subjectNameConfig))
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	err = schemaRegistryClientObject.Set("serialize", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			common.Throw(runtime, ErrNotEnoughArguments)
		}

		var metadata *Container
		decodeArgument(runtime, call.Argument(0), &metadata, "serialize metadata")

		return runtime.ToValue(k.serializeWithRegistry(metadata, registryState))
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	err = schemaRegistryClientObject.Set("deserialize", func(call sobek.FunctionCall) sobek.Value {
		if len(call.Arguments) == 0 {
			common.Throw(runtime, ErrNotEnoughArguments)
		}

		var metadata *Container
		decodeArgument(runtime, call.Argument(0), &metadata, "deserialize metadata")

		return runtime.ToValue(k.deserializeWithRegistry(metadata, registryState))
	})
	if err != nil {
		common.Throw(runtime, err)
	}

	return schemaRegistryClientObject
}

// gzipTransport wraps an http.RoundTripper to handle gzip decompression
// even when the Content-Encoding header is missing or incorrect.
type gzipTransport struct {
	transport http.RoundTripper
}

func (gt *gzipTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("Accept-Encoding") == "" {
		req.Header.Set("Accept-Encoding", "gzip")
	}

	resp, err := gt.transport.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	if resp.Body != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			_ = resp.Body.Close()
			return resp, err
		}
		_ = resp.Body.Close()

		// Check for gzip magic number: 0x1f 0x8b
		// This works even if Content-Encoding header is missing
		if len(bodyBytes) >= 2 && bodyBytes[0] == 0x1f && bodyBytes[1] == 0x8b {
			// Decompress the gzipped response
			gzipReader, err := gzip.NewReader(bytes.NewReader(bodyBytes))
			if err == nil {
				decompressed, err := io.ReadAll(gzipReader)
				_ = gzipReader.Close()
				if err == nil {
					bodyBytes = decompressed
					// Remove Content-Encoding header if it was set incorrectly
					resp.Header.Del("Content-Encoding")
					resp.ContentLength = int64(len(bodyBytes))
				}
			}
		}

		// Replace the response body with the (possibly decompressed) content
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	return resp, nil
}

// schemaRegistryClient creates a schemaRegistryClient instance
// with the given configuration. It will also configure auth and TLS credentials if exists.
func (k *Kafka) schemaRegistryClient(config *SchemaRegistryConfig) SchemaRegistryClient {
	runtime := k.vu.Runtime()
	if config == nil {
		throwConfigError(runtime, newMissingConfigError("schema registry config"))
		return nil
	}
	if config.URL == "" {
		throwConfigError(runtime, newInvalidConfigError("schema registry config", errURLMustNotBeEmpty))
		return nil
	}

	tlsConfig, err := GetTLSConfig(config.TLS)
	if err != nil && err.Code != noTLSConfig {
		common.Throw(runtime, err)
		return nil
	}

	httpClient := &http.Client{
		Transport: &gzipTransport{transport: newSchemaRegistryTransport(tlsConfig)},
	}

	clientConfig := cschemaregistry.NewConfig(config.URL)
	clientConfig.HTTPClient = httpClient
	if config.BasicAuth.Username != "" && config.BasicAuth.Password != "" {
		clientConfig.BasicAuthUserInfo = fmt.Sprintf(
			"%s:%s",
			config.BasicAuth.Username,
			config.BasicAuth.Password,
		)
		clientConfig.BasicAuthCredentialsSource = "USER_INFO"
	}

	srClient, clientErr := cschemaregistry.NewClient(clientConfig)
	if clientErr != nil {
		common.Throw(runtime, NewXk6KafkaError(
			failedConfigureSchemaRegistryClient,
			"Failed to configure the schema registry client",
			clientErr,
		))
		return nil
	}

	return newConfluentSchemaRegistryAdapter(srClient, config.EnableCaching)
}

func newSchemaRegistryTransport(tlsConfig *tls.Config) http.RoundTripper {
	if tlsConfig == nil {
		return http.DefaultTransport
	}

	baseTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Transport{TLSClientConfig: tlsConfig}
	}

	transport := baseTransport.Clone()
	transport.TLSClientConfig = tlsConfig
	// Cloning http.DefaultTransport copies TLSNextProto (the HTTP/2 protocol map).
	// Setting TLSClientConfig on the clone without clearing TLSNextProto leaves the
	// HTTP/2 connection pool in a broken state, causing EOF and gzip decoding failures
	// on HTTPS schema registry endpoints. Clearing TLSNextProto forces HTTP/1.1, which
	// is the same behaviour as the v1 fix (PR #363) that used a fresh http.Transport.
	transport.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
	return transport
}

// getSchema returns the schema for the given subject and schema ID and version.
func (k *Kafka) getSchema(client SchemaRegistryClient, schema *Schema) *Schema {
	return k.getSchemaWithCache(client, k.schemaCache, schema)
}

func (k *Kafka) getSchemaWithCache(
	client SchemaRegistryClient,
	cache map[string]*Schema,
	schema *Schema,
) *Schema {
	if client == nil {
		throwConfigError(k.vu.Runtime(), newMissingConfigError("schema registry client"))
		return nil
	}
	if schema == nil {
		throwConfigError(k.vu.Runtime(), newMissingConfigError("schema metadata"))
		return nil
	}
	if schema.Subject == "" {
		throwConfigError(k.vu.Runtime(), newInvalidConfigError("schema metadata", errSubjectMustNotBeEmpty))
		return nil
	}

	// If EnableCache is set, check if the schema is in the cache.
	if schema.EnableCaching {
		if schema, ok := cache[schema.Subject]; ok {
			return schema
		}
	}

	runtime := k.vu.Runtime()
	// The client always caches the schema.
	var schemaInfo *RegisteredSchema
	var err error
	// Default version of the schema is the latest version.
	if schema.Version == 0 {
		schemaInfo, err = client.GetLatestSchema(schema.Subject)
	} else {
		schemaInfo, err = client.GetSchemaByVersion(
			schema.Subject, schema.Version)
	}

	if err == nil {
		wrappedSchema := &Schema{
			EnableCaching: schema.EnableCaching,
			ID:            schemaInfo.ID(),
			Version:       schemaInfo.Version(),
			Schema:        schemaInfo.Schema(),
			SchemaType:    schemaInfo.SchemaType(),
			References:    schemaInfo.References(),
			Subject:       schema.Subject,
			resolver:      k.createResolverWithCache(client, cache, schema.EnableCaching),
		}
		// If the Cache is set, cache the schema.
		if wrappedSchema.EnableCaching {
			cache[wrappedSchema.Subject] = wrappedSchema
		}
		return wrappedSchema
	} else {
		err := NewXk6KafkaError(schemaNotFound, "Failed to get schema from schema registry", err)
		common.Throw(runtime, err)
		return nil
	}
}

func (k *Kafka) createSchemaWithCache(
	client SchemaRegistryClient,
	cache map[string]*Schema,
	schema *Schema,
) *Schema {
	runtime := k.vu.Runtime()
	if client == nil {
		throwConfigError(runtime, newMissingConfigError("schema registry client"))
		return nil
	}
	if schema == nil {
		throwConfigError(runtime, newMissingConfigError("schema metadata"))
		return nil
	}
	if schema.Subject == "" {
		throwConfigError(runtime, newInvalidConfigError("schema metadata", errSubjectMustNotBeEmpty))
		return nil
	}
	if schema.Schema == "" {
		throwConfigError(runtime, newInvalidConfigError("schema metadata", errSchemaMustNotBeEmpty))
		return nil
	}
	if schema.SchemaType == nil {
		throwConfigError(runtime, newInvalidConfigError("schema metadata", errSchemaTypeMustNotBeEmpty))
		return nil
	}

	schemaInfo, err := client.CreateSchema(
		schema.Subject,
		schema.Schema,
		*schema.SchemaType,
		schema.References...)
	if err != nil {
		err := NewXk6KafkaError(schemaCreationFailed, "Failed to create schema.", err)
		common.Throw(runtime, err)
		return nil
	}

	wrappedSchema := &Schema{
		EnableCaching: schema.EnableCaching,
		ID:            schemaInfo.ID(),
		Version:       schemaInfo.Version(),
		Schema:        schemaInfo.Schema(),
		SchemaType:    schemaInfo.SchemaType(),
		References:    schemaInfo.References(),
		Subject:       schema.Subject,
		resolver:      k.createResolverWithCache(client, cache, schema.EnableCaching),
	}
	if schema.EnableCaching {
		cache[schema.Subject] = wrappedSchema
	}
	return wrappedSchema
}

// getSubjectName returns the subject name for the given schema and topic.
func (k *Kafka) getSubjectName(subjectNameConfig *SubjectNameConfig) string {
	if subjectNameConfig == nil {
		throwConfigError(k.vu.Runtime(), newMissingConfigError("subject name config"))
		return ""
	}
	if subjectNameConfig.SubjectNameStrategy == "" ||
		subjectNameConfig.SubjectNameStrategy == TopicNameStrategy {
		return subjectNameConfig.Topic + "-" + string(subjectNameConfig.Element)
	}

	runtime := k.vu.Runtime()
	recordName := ""
	var schemaMap map[string]any
	if strings.TrimSpace(subjectNameConfig.MessageName) != "" {
		recordName = strings.TrimSpace(subjectNameConfig.MessageName)
	}

	if recordName == "" {
		err := json.Unmarshal([]byte(subjectNameConfig.Schema), &schemaMap)
		if err != nil {
			common.Throw(runtime, NewXk6KafkaError(
				failedUnmarshalSchema, "Failed to unmarshal schema", err))
		}
		if namespace, ok := schemaMap["namespace"]; ok {
			if namespace, ok := namespace.(string); ok {
				recordName = namespace + "."
			} else {
				err := NewXk6KafkaError(failedTypeCast, "Failed to cast to string", nil)
				common.Throw(runtime, err)
			}
		}
	}
	if name, ok := schemaMap["name"]; ok {
		if name, ok := name.(string); ok {
			recordName += name
		} else {
			err := NewXk6KafkaError(failedTypeCast, "Failed to cast to string", nil)
			common.Throw(runtime, err)
		}
	}

	if subjectNameConfig.SubjectNameStrategy == RecordNameStrategy {
		return recordName
	}
	if subjectNameConfig.SubjectNameStrategy == TopicRecordNameStrategy {
		return subjectNameConfig.Topic + "-" + recordName
	}

	err := NewXk6KafkaError(failedToEncode, fmt.Sprintf(
		"Unknown subject name strategy: %v", subjectNameConfig.SubjectNameStrategy), nil)
	common.Throw(runtime, err)
	return ""
}

// encodeWireFormat adds the proprietary 5-byte prefix to the Avro, ProtoBuf or
// JSONSchema payload.
// https://docs.confluent.io/platform/current/schema-registry/serdes-develop/index.html#wire-format
func (k *Kafka) encodeWireFormat(data []byte, schemaID int) []byte {
	schemaIDBytes := make([]byte, MagicPrefixSize-1)
	// Validate schemaID is within uint32 range to prevent overflow
	if schemaID < 0 || schemaID > int(^uint32(0)) {
		err := NewXk6KafkaError(
			invalidSchemaID,
			fmt.Sprintf("Invalid schema id %d: must be within uint32 range", schemaID),
			nil,
		)
		logger.WithField("error", err).Error(err)
		common.Throw(k.vu.Runtime(), err)
		return nil
	}
	binary.BigEndian.PutUint32(schemaIDBytes, uint32(schemaID))
	return append(append([]byte{0}, schemaIDBytes...), data...)
}

// decodeWireFormat removes the proprietary 5-byte prefix from the Avro, ProtoBuf
// or JSONSchema payload.
// https://docs.confluent.io/platform/current/schema-registry/serdes-develop/index.html#wire-format
func (k *Kafka) decodeWireFormat(message []byte) []byte {
	runtime := k.vu.Runtime()
	if len(message) < MagicPrefixSize {
		err := NewXk6KafkaError(messageTooShort,
			"Invalid message: message too short to contain schema id.", nil)
		common.Throw(runtime, err)
		return nil
	}
	if message[0] != 0 {
		err := NewXk6KafkaError(messageTooShort, "Invalid message: invalid start byte.", nil)
		common.Throw(runtime, err)
		return nil
	}
	return message[MagicPrefixSize:]
}
