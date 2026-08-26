package kafka

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/grafana/sobek"
	"github.com/hamba/avro/v2"
	"go.k6.io/k6/v2/js/modules"
)

// schemaRegistryTimeout bounds every Schema Registry HTTP call so a slow or
// unreachable registry cannot stall a VU (or init) indefinitely.
const schemaRegistryTimeout = 60 * time.Second

// errProtobufUnsupported is returned for SCHEMA_TYPE_PROTOBUF. Protobuf serdes
// arrived in the community v2 surface (which is out of scope here); this
// v1-compatible extension supports Avro and JSON only. The constant is kept so
// scripts referencing it fail with a clear message rather than a ReferenceError.
var errProtobufUnsupported = errors.New(
	"SchemaRegistry: Protobuf serdes is not supported in v1 (Avro and JSON only)")

// BasicAuth holds Schema Registry basic auth credentials.
type BasicAuth struct {
	Username string `js:"username"`
	Password string `js:"password"` //nolint:gosec // not a real secret
}

// Schema represents a schema fetched from or registered with Schema Registry.
type Schema struct {
	ID         int    `json:"id" js:"id"`
	Subject    string `json:"subject" js:"subject"`
	Version    int    `json:"version" js:"version"`
	Schema     string `json:"schema" js:"schema"`
	SchemaType string `json:"schemaType" js:"schemaType"`
}

// Container bundles data with schema info for serialize/deserialize.
type Container struct {
	Data       any     `js:"data"`
	SchemaType string  `js:"schemaType"`
	Schema     *Schema `js:"schema"`
}

// SubjectNameConfig holds params for computing subject names.
type SubjectNameConfig struct {
	Topic               string `js:"topic"`
	Element             string `js:"element"`
	SubjectNameStrategy string `js:"subjectNameStrategy"`
	Schema              string `js:"schema"`
}

// SchemaRegistryConfig holds Schema Registry connection settings.
type SchemaRegistryConfig struct {
	URL           string     `js:"url"`
	BasicAuth     *BasicAuth `js:"basicAuth"`
	TLS           *TLSConfig `js:"tls"`
	EnableCaching bool       `js:"enableCaching"`
}

// SchemaRegistry is a client for Schema Registry and serdes operations.
type SchemaRegistry struct {
	vu     modules.VU
	config *SchemaRegistryConfig
	client *http.Client

	// enableCaching gates the registry-response cache (schemaCache) only.
	// Parsed-Avro reuse (avroCache) is always on. Both maps are guarded by mu,
	// which is defensive: access is only from the VU's JS goroutine.
	enableCaching bool
	mu            sync.RWMutex
	schemaCache   map[string]*Schema
	avroCache     map[string]avro.Schema
}

// reqContext returns the VU context when one is available, so registry calls
// cancel when the test stops. It falls back to Background in the init context
// (or when constructed without a VU, e.g. unit tests). The client's Timeout
// bounds the call either way.
func (sr *SchemaRegistry) reqContext() context.Context {
	return vuContext(sr.vu)
}

// vuContext resolves the usable context for a (possibly nil) VU.
func vuContext(vu modules.VU) context.Context {
	if vu != nil {
		if ctx := vu.Context(); ctx != nil {
			return ctx
		}
	}
	return context.Background()
}

// NewSchemaRegistry creates a new SchemaRegistry client.
func NewSchemaRegistry(vu modules.VU, config *SchemaRegistryConfig) (*SchemaRegistry, error) {
	if config == nil {
		// Standalone mode: no registry, so no registry-response caching, but
		// parsed-Avro reuse still applies to inline schemas.
		return &SchemaRegistry{
			vu:        vu,
			config:    nil,
			avroCache: map[string]avro.Schema{},
		}, nil
	}

	if config.URL == "" {
		return nil, fmt.Errorf("SchemaRegistry: url is required")
	}

	// Build HTTP client with TLS config. Timeout bounds every call so an
	// unreachable registry fails instead of hanging. The registry client gates
	// TLS on the URL scheme (https), not EnableTLS, so any TLS material provided
	// (minVersion, client cert/key, server CA, insecure-skip) is honored.
	httpClient := &http.Client{Timeout: schemaRegistryTimeout}
	if config.TLS != nil {
		tlsConfig, err := tlsConfigFrom(config.TLS)
		if err != nil {
			return nil, fmt.Errorf("SchemaRegistry: %w", err)
		}
		httpClient.Transport = &http.Transport{TLSClientConfig: tlsConfig}
	}

	// Validate connectivity with /config endpoint (requires auth)
	req, err := http.NewRequestWithContext(vuContext(vu), http.MethodGet, config.URL+"/config", nil)
	if err != nil {
		return nil, fmt.Errorf("SchemaRegistry: failed to create request: %w", err)
	}
	if config.BasicAuth != nil {
		req.SetBasicAuth(config.BasicAuth.Username, config.BasicAuth.Password)
	}

	resp, err := httpClient.Do(req) //nolint:gosec // registry URL is configured, not user input
	if err != nil {
		return nil, fmt.Errorf("SchemaRegistry: failed to reach registry at %s: %w", config.URL, err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("SchemaRegistry: registry returned %d (expected 2xx)", resp.StatusCode)
	}

	sr := &SchemaRegistry{
		vu:            vu,
		config:        config,
		client:        httpClient,
		enableCaching: config.EnableCaching,
		schemaCache:   map[string]*Schema{},
		avroCache:     map[string]avro.Schema{},
	}
	return sr, nil
}

// schemaCacheKey keys the response cache by subject and requested version;
// a nil version means "latest".
func schemaCacheKey(subject string, version *int) string {
	if version == nil {
		return subject + "\x00"
	}
	return subject + "\x00" + strconv.Itoa(*version)
}

// parsedAvro parses an Avro schema string, reusing a previously parsed result.
// Reuse is always on (behavior-neutral); a parse failure is not cached.
func (sr *SchemaRegistry) parsedAvro(schemaStr string) (avro.Schema, error) {
	if sr.avroCache != nil {
		sr.mu.RLock()
		cached, ok := sr.avroCache[schemaStr]
		sr.mu.RUnlock()
		if ok {
			return cached, nil
		}
	}
	parsed, err := avro.Parse(schemaStr)
	if err != nil {
		return nil, err
	}
	if sr.avroCache != nil {
		sr.mu.Lock()
		sr.avroCache[schemaStr] = parsed
		sr.mu.Unlock()
	}
	return parsed, nil
}

// GetSchema fetches a schema from the registry.
func (sr *SchemaRegistry) GetSchema(schemaParam *Schema) (*Schema, error) {
	if sr.config == nil {
		return nil, fmt.Errorf("SchemaRegistry: GetSchema requires registry configuration (standalone mode not supported)")
	}
	if schemaParam == nil || schemaParam.Subject == "" {
		return nil, fmt.Errorf("SchemaRegistry: GetSchema requires schema with subject")
	}

	var version *int
	if schemaParam.Version > 0 {
		version = &schemaParam.Version
	}
	return sr.getSchema(schemaParam.Subject, version)
}

// getSchema fetches a schema from the registry (internal helper).
func (sr *SchemaRegistry) getSchema(subject string, version *int) (*Schema, error) {
	if sr.config == nil {
		return nil, fmt.Errorf("SchemaRegistry: getSchema requires registry configuration (standalone mode not supported)")
	}

	key := schemaCacheKey(subject, version)
	if sr.enableCaching {
		sr.mu.RLock()
		cached, ok := sr.schemaCache[key]
		sr.mu.RUnlock()
		if ok {
			return copySchema(cached), nil
		}
	}

	path := fmt.Sprintf("/subjects/%s/versions/latest", subject)
	if version != nil {
		path = fmt.Sprintf("/subjects/%s/versions/%d", subject, *version)
	}

	req, err := http.NewRequestWithContext(sr.reqContext(), http.MethodGet, sr.config.URL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("SchemaRegistry: failed to create request: %w", err)
	}

	if sr.config.BasicAuth != nil {
		req.SetBasicAuth(sr.config.BasicAuth.Username, sr.config.BasicAuth.Password)
	}

	resp, err := sr.client.Do(req) //nolint:gosec // registry URL is configured, not user input
	if err != nil {
		return nil, fmt.Errorf("SchemaRegistry: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SchemaRegistry: GET %s returned %d", path, resp.StatusCode)
	}

	var schema Schema
	if err := json.NewDecoder(resp.Body).Decode(&schema); err != nil {
		return nil, fmt.Errorf("SchemaRegistry: failed to decode response: %w", err)
	}

	if sr.enableCaching {
		sr.mu.Lock()
		sr.schemaCache[key] = &schema
		sr.mu.Unlock()
		// Return a copy so the first caller cannot mutate the cached entry.
		return copySchema(&schema), nil
	}
	return &schema, nil
}

// copySchema returns an independent shallow copy of a Schema. Schema has only
// scalar fields, so a shallow copy fully isolates the cached entry from callers.
func copySchema(s *Schema) *Schema {
	c := *s
	return &c
}

// CreateSchema registers a schema in the registry.
func (sr *SchemaRegistry) CreateSchema(schemaParam *Schema) (*Schema, error) {
	if sr.config == nil {
		return nil, fmt.Errorf("SchemaRegistry: CreateSchema requires registry configuration (standalone mode not supported)")
	}
	if schemaParam == nil || schemaParam.Subject == "" || schemaParam.Schema == "" || schemaParam.SchemaType == "" {
		return nil, fmt.Errorf("SchemaRegistry: CreateSchema requires schema with subject, schema, and schemaType")
	}

	return sr.createSchema(schemaParam.Subject, schemaParam.Schema, schemaParam.SchemaType)
}

// createSchema registers a schema in the registry (internal helper).
func (sr *SchemaRegistry) createSchema(subject string, schemaStr string, schemaType string) (*Schema, error) {
	if sr.config == nil {
		return nil, fmt.Errorf("SchemaRegistry: createSchema requires registry configuration (standalone mode not supported)")
	}

	reqBody := map[string]any{
		"schema":     schemaStr,
		"schemaType": schemaType,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("SchemaRegistry: failed to marshal request: %w", err)
	}

	url := sr.config.URL + "/subjects/" + subject + "/versions"
	req, err := http.NewRequestWithContext(sr.reqContext(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("SchemaRegistry: failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")
	if sr.config.BasicAuth != nil {
		req.SetBasicAuth(sr.config.BasicAuth.Username, sr.config.BasicAuth.Password)
	}

	resp, err := sr.client.Do(req) //nolint:gosec // registry URL is configured, not user input
	if err != nil {
		return nil, fmt.Errorf("SchemaRegistry: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		statusMsg := fmt.Sprintf("POST /subjects/%s/versions returned %d: %s", subject, resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("SchemaRegistry: %s", statusMsg)
	}

	// Decode registry response for ID and version
	var registryResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&registryResp); err != nil {
		return nil, fmt.Errorf("SchemaRegistry: failed to decode response: %w", err)
	}

	id, ok := registryResp["id"].(float64)
	if !ok {
		return nil, fmt.Errorf("SchemaRegistry: response missing schema id")
	}

	// Confluent's POST /subjects/{subject}/versions returns only the schema id;
	// version is usually absent. Report it only when the registry includes it,
	// leaving 0 ("unknown") otherwise rather than fabricating a value.
	version := 0
	if v, ok := registryResp["version"].(float64); ok {
		version = int(v)
	}

	// Return complete Schema with input fields + registry-provided ID/version
	return &Schema{
		ID:         int(id),
		Subject:    subject,
		Version:    version,
		Schema:     schemaStr,
		SchemaType: schemaType,
	}, nil
}

// GetSubjectName computes the registry subject name from topic, element, and naming strategy.
func (sr *SchemaRegistry) GetSubjectName(config *SubjectNameConfig) (string, error) {
	if config == nil {
		return "", fmt.Errorf("SchemaRegistry: GetSubjectName requires a config")
	}
	if config.Topic == "" {
		return "", fmt.Errorf("SchemaRegistry: GetSubjectName requires topic")
	}
	if config.Element == "" {
		return "", fmt.Errorf("SchemaRegistry: GetSubjectName requires element")
	}
	return sr.getSubjectName(config.Topic, config.Element, config.SubjectNameStrategy, config.Schema)
}

// getSubjectName returns the subject name for a topic and element using TopicNameStrategy.
func (sr *SchemaRegistry) getSubjectName(topic, element, strategy, schema string) (string, error) {
	switch strategy {
	case "", topicNameStrategy:
		// TopicNameStrategy (also the default): {topic}-{element}. Ignores schema.
		return topic + "-" + strings.ToLower(element), nil
	case recordNameStrategy:
		name, err := sr.recordFullName(schema)
		if err != nil {
			return "", err
		}
		return name, nil
	case topicRecordNameStrategy:
		name, err := sr.recordFullName(schema)
		if err != nil {
			return "", err
		}
		return topic + "-" + name, nil
	default:
		return "", fmt.Errorf("SchemaRegistry: unknown subjectNameStrategy %q", strategy)
	}
}

// recordFullName returns the fully-qualified name of an Avro named schema
// (record/enum/fixed) parsed from schemaStr, for the record-name strategies.
// It errors on an empty, unparseable, or non-named schema rather than guess;
// JSON Schema and Protobuf record naming are not supported in v1.
func (sr *SchemaRegistry) recordFullName(schemaStr string) (string, error) {
	if schemaStr == "" {
		return "", fmt.Errorf("SchemaRegistry: record-name strategy requires a schema")
	}
	parsed, err := sr.parsedAvro(schemaStr)
	if err != nil {
		return "", fmt.Errorf("SchemaRegistry: record-name strategy requires a parseable Avro schema: %w", err)
	}
	named, ok := parsed.(avro.NamedSchema)
	if !ok {
		return "", fmt.Errorf(
			"SchemaRegistry: record-name strategy requires a named Avro schema (record/enum/fixed), got %q", parsed.Type())
	}
	return named.FullName(), nil
}

// encodeWireFormat encodes a 5-byte Confluent magic envelope. The schema ID is
// a big-endian uint32, so it must be in range; an out-of-range ID is an error
// rather than a silently truncated cast.
func encodeWireFormat(schemaID int) ([]byte, error) {
	if schemaID < 0 || schemaID > math.MaxUint32 {
		return nil, fmt.Errorf("SchemaRegistry: schema ID %d out of range for wire format (0..%d)", schemaID, math.MaxUint32)
	}
	buf := make([]byte, 5)
	buf[0] = 0x00
	binary.BigEndian.PutUint32(buf[1:], uint32(schemaID))
	return buf, nil
}

// withWireFormat prepends the Confluent envelope when the schema carries a
// registry-assigned ID; otherwise it returns the payload unchanged.
func withWireFormat(schema *Schema, payload []byte) ([]byte, error) {
	if schema == nil || schema.ID == 0 {
		return payload, nil
	}
	envelope, err := encodeWireFormat(schema.ID)
	if err != nil {
		return nil, err
	}
	return append(envelope, payload...), nil
}

// decodeWireFormat decodes a 5-byte Confluent magic envelope and returns (schemaID, remainingBytes).
func decodeWireFormat(data []byte) (int, []byte, error) {
	if len(data) < 5 {
		return 0, nil, fmt.Errorf("SchemaRegistry: wire format data too short (need 5 bytes, got %d)", len(data))
	}

	if data[0] != 0x00 {
		return 0, nil, fmt.Errorf("SchemaRegistry: invalid magic byte: expected 0x00, got 0x%02x", data[0])
	}

	schemaID := int(binary.BigEndian.Uint32(data[1:5]))
	return schemaID, data[5:], nil
}

// coerceBytes normalizes byte-like values crossing the JS bridge. Depending on
// how sobek exports arrays and typed arrays, callers may receive []byte,
// *[]byte, ArrayBuffer, or generic numeric arrays.
func coerceBytes(v any) ([]byte, bool) {
	switch x := v.(type) {
	case nil:
		return nil, true
	case []byte:
		return x, true
	case *[]byte:
		if x == nil {
			return nil, true
		}
		return *x, true
	case sobek.ArrayBuffer:
		return x.Bytes(), true
	case *sobek.ArrayBuffer:
		if x == nil {
			return nil, true
		}
		return x.Bytes(), true
	}

	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return nil, true
	}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, true
		}
		return coerceBytes(rv.Elem().Interface())
	}
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return nil, false
	}

	out := make([]byte, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		b, ok := coerceByte(rv.Index(i))
		if !ok {
			return nil, false
		}
		out[i] = b
	}
	return out, true
}

func coerceByte(v reflect.Value) (byte, bool) {
	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr) {
		if v.IsNil() {
			return 0, false
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		n := v.Uint()
		if n > math.MaxUint8 {
			return 0, false
		}
		return byte(n), true
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		n := v.Int()
		if n < 0 || n > math.MaxUint8 {
			return 0, false
		}
		return byte(n), true
	case reflect.Float32, reflect.Float64:
		n := v.Float()
		if n < 0 || n > math.MaxUint8 || math.Trunc(n) != n {
			return 0, false
		}
		return byte(n), true
	default:
		return 0, false
	}
}

func normalizeAvroRecord(s *avro.RecordSchema, data any) (any, error) {
	record, ok := data.(map[string]any)
	if !ok {
		return data, nil
	}
	out := make(map[string]any, len(record))
	maps.Copy(out, record)
	for _, field := range s.Fields() {
		value, ok := record[field.Name()]
		if !ok {
			continue
		}
		normalized, err := normalizeAvroValue(field.Type(), value)
		if err != nil {
			return nil, err
		}
		out[field.Name()] = normalized
	}
	return out, nil
}

func normalizeAvroValue(schema avro.Schema, data any) (any, error) {
	switch s := schema.(type) {
	case *avro.RecordSchema:
		return normalizeAvroRecord(s, data)
	case *avro.ArraySchema:
		rv := reflect.ValueOf(data)
		if !rv.IsValid() || (rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array) {
			return data, nil
		}
		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			normalized, err := normalizeAvroValue(s.Items(), rv.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			out[i] = normalized
		}
		return out, nil
	case *avro.MapSchema:
		record, ok := data.(map[string]any)
		if !ok {
			return data, nil
		}
		out := make(map[string]any, len(record))
		for k, v := range record {
			normalized, err := normalizeAvroValue(s.Values(), v)
			if err != nil {
				return nil, err
			}
			out[k] = normalized
		}
		return out, nil
	case *avro.UnionSchema:
		if data == nil {
			return data, nil
		}
		for _, branch := range s.Types() {
			if branch.Type() == avro.Null {
				continue
			}
			return normalizeAvroValue(branch, data)
		}
		return data, nil
	case *avro.RefSchema:
		return normalizeAvroValue(s.Schema(), data)
	default:
		return normalizeAvroPrimitive(schema.Type(), data), nil
	}
}

func normalizeAvroPrimitive(typ avro.Type, data any) any {
	switch typ {
	case avro.Int:
		if n, ok := coerceNumber(data); ok && n >= math.MinInt32 && n <= math.MaxInt32 && math.Trunc(n) == n {
			return int32(n)
		}
	case avro.Long:
		if n, ok := coerceNumber(data); ok && n >= math.MinInt64 && n <= math.MaxInt64 && math.Trunc(n) == n {
			return int64(n)
		}
	case avro.Float:
		if n, ok := coerceNumber(data); ok {
			return float32(n)
		}
	case avro.Double:
		if n, ok := coerceNumber(data); ok {
			return n
		}
	case avro.Bytes:
		if b, ok := coerceBytes(data); ok {
			return b
		}
	case avro.String, avro.Boolean, avro.Null:
		// Primitives returned as-is
	case avro.Record, avro.Enum, avro.Array, avro.Map, avro.Union, avro.Fixed, avro.Error, avro.Ref:
		// Complex types handled by caller
	}
	return data
}

func coerceNumber(v any) (float64, bool) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return 0, false
	}
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return 0, false
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return rv.Float(), true
	default:
		return 0, false
	}
}

// Serialize encodes data using the schema and schema type from Container and
// returns a JS Uint8Array (per index.d.ts), suitable for writer.produce.
func (sr *SchemaRegistry) Serialize(container *Container) (sobek.Value, error) {
	if container == nil {
		return nil, fmt.Errorf("SchemaRegistry: Serialize requires a container")
	}
	if sr.vu == nil {
		return nil, fmt.Errorf("SchemaRegistry: Serialize requires a runtime (call it from a VU)")
	}
	b, err := sr.serialize(container.Data, container.SchemaType, container.Schema)
	if err != nil {
		return nil, err
	}
	return bytesToUint8Array(sr.vu.Runtime(), b)
}

// bytesToUint8Array wraps raw bytes in a JS Uint8Array via the runtime. sobek
// exports a Go []byte as a plain Array, so an explicit Uint8Array is built to
// match the contract's return type.
func bytesToUint8Array(rt *sobek.Runtime, b []byte) (sobek.Value, error) {
	u8, err := rt.New(rt.Get("Uint8Array"), rt.ToValue(rt.NewArrayBuffer(b)))
	if err != nil {
		return nil, fmt.Errorf("SchemaRegistry: building Uint8Array: %w", err)
	}
	return u8, nil
}

// serialize encodes data to bytes.
func (sr *SchemaRegistry) serialize(data any, schemaType string, schema *Schema) ([]byte, error) {
	switch schemaType {
	case "STRING":
		if str, ok := data.(string); ok {
			return []byte(str), nil
		}
		return nil, fmt.Errorf("SchemaRegistry: STRING serialize expects string, got %T", data)

	case "BYTES":
		if b, ok := coerceBytes(data); ok {
			return b, nil
		}
		return nil, fmt.Errorf("SchemaRegistry: BYTES serialize expects []byte, got %T", data)

	case "AVRO":
		if schema == nil {
			return nil, fmt.Errorf("SchemaRegistry: AVRO serialize requires schema")
		}
		avroSchema, err := sr.parsedAvro(schema.Schema)
		if err != nil {
			return nil, fmt.Errorf("SchemaRegistry: failed to parse Avro schema: %w", err)
		}
		normalized, err := normalizeAvroValue(avroSchema, data)
		if err != nil {
			return nil, fmt.Errorf("SchemaRegistry: failed to normalize Avro data: %w", err)
		}
		encoded, err := avro.Marshal(avroSchema, normalized)
		if err != nil {
			return nil, fmt.Errorf("SchemaRegistry: Avro encode failed: %w", err)
		}
		return withWireFormat(schema, encoded)

	case "JSON":
		if schema == nil {
			return nil, fmt.Errorf("SchemaRegistry: JSON serialize requires schema")
		}
		// Validate data against schema (basic: required fields)
		dataMap, ok := data.(map[string]any)
		if ok {
			if err := validateJSONRequired(dataMap, schema.Schema); err != nil {
				return nil, fmt.Errorf("SchemaRegistry: JSON validation failed: %w", err)
			}
		}
		// Encode to JSON
		encoded, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("SchemaRegistry: JSON encode failed: %w", err)
		}
		return withWireFormat(schema, encoded)

	case schemaTypeProtobuf:
		return nil, errProtobufUnsupported

	default:
		return nil, fmt.Errorf("SchemaRegistry: unsupported schema type: %s", schemaType)
	}
}

// Deserialize decodes bytes to data using schema and schema type from Container.
func (sr *SchemaRegistry) Deserialize(container *Container) (any, error) {
	if container == nil {
		return nil, fmt.Errorf("SchemaRegistry: Deserialize requires a container")
	}
	data, ok := coerceBytes(container.Data)
	if !ok {
		return nil, fmt.Errorf("SchemaRegistry: Deserialize expects byte-like data, got %T", container.Data)
	}
	return sr.deserialize(data, container.SchemaType, container.Schema)
}

// checkAndStripWireFormat validates and strips the wire format envelope if present.
// Returns the remaining data and any error. If no envelope, returns original data.
func (sr *SchemaRegistry) checkAndStripWireFormat(data []byte, schema *Schema) ([]byte, error) {
	if schema == nil || schema.ID == 0 || len(data) < 5 || data[0] != 0x00 {
		return data, nil
	}
	schemaID, remaining, err := decodeWireFormat(data)
	if err != nil {
		return nil, err
	}
	if schemaID != schema.ID {
		return nil, fmt.Errorf("SchemaRegistry: schema ID mismatch: expected %d, got %d", schema.ID, schemaID)
	}
	return remaining, nil
}

// deserialize decodes bytes to data.
func (sr *SchemaRegistry) deserialize(data []byte, schemaType string, schema *Schema) (any, error) {
	switch schemaType {
	case "STRING":
		// Check for wire format envelope
		var err error
		data, err = sr.checkAndStripWireFormat(data, schema)
		if err != nil {
			return nil, err
		}
		// Validate UTF-8
		if !utf8.Valid(data) {
			return nil, fmt.Errorf("SchemaRegistry: STRING deserialize: invalid UTF-8")
		}
		return string(data), nil

	case "BYTES":
		// Check for wire format envelope
		var err error
		data, err = sr.checkAndStripWireFormat(data, schema)
		if err != nil {
			return nil, err
		}
		return data, nil

	case "AVRO":
		if schema == nil {
			return nil, fmt.Errorf("SchemaRegistry: AVRO deserialize requires schema")
		}

		// Check for wire format envelope
		var err error
		data, err = sr.checkAndStripWireFormat(data, schema)
		if err != nil {
			return nil, err
		}

		avroSchema, err := sr.parsedAvro(schema.Schema)
		if err != nil {
			return nil, fmt.Errorf("SchemaRegistry: failed to parse Avro schema: %w", err)
		}

		var result any
		if err := avro.Unmarshal(avroSchema, data, &result); err != nil {
			return nil, fmt.Errorf("SchemaRegistry: Avro decode failed: %w", err)
		}
		return result, nil

	case "JSON":
		if schema == nil {
			return nil, fmt.Errorf("SchemaRegistry: JSON deserialize requires schema")
		}

		// Check for wire format envelope
		var err error
		data, err = sr.checkAndStripWireFormat(data, schema)
		if err != nil {
			return nil, err
		}

		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("SchemaRegistry: JSON decode failed: %w", err)
		}
		// Validate against schema (basic: required fields)
		if err := validateJSONRequired(result, schema.Schema); err != nil {
			return nil, fmt.Errorf("SchemaRegistry: JSON validation failed: %w", err)
		}
		return result, nil

	case schemaTypeProtobuf:
		return nil, errProtobufUnsupported

	default:
		return nil, fmt.Errorf("SchemaRegistry: unsupported schema type: %s", schemaType)
	}
}

// validateJSONRequired performs basic JSON Schema validation: checks required fields.
func validateJSONRequired(data map[string]any, schemaStr string) error {
	var schema map[string]any
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		// Invalid schema document should fail
		return fmt.Errorf("invalid JSON schema: %w", err)
	}

	required, ok := schema["required"].([]any)
	if !ok {
		// No required fields, validation passes
		return nil
	}

	for _, fieldI := range required {
		field, ok := fieldI.(string)
		if !ok {
			continue
		}
		if _, present := data[field]; !present {
			return fmt.Errorf("required field missing: %s", field)
		}
	}
	return nil
}
