package kafka

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/grafana/sobek"
	"github.com/stretchr/testify/require"
)

// cachingTestRegistry is an httptest registry that counts schema GET requests.
// /config returns 200; GET /subjects/.../versions/... returns a fixed v1 schema;
// POST /subjects/.../versions returns an id.
func cachingTestRegistry(t *testing.T) (*httptest.Server, *int32) {
	t.Helper()
	var gets int32
	const schemaJSON = `{"id":1,"version":1,"subject":"s-value","schema":"\"string\"","schemaType":"STRING"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/config":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/versions/"):
			atomic.AddInt32(&gets, 1)
			_, _ = w.Write([]byte(schemaJSON))
		case r.Method == http.MethodPost:
			_, _ = w.Write([]byte(`{"id":2}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &gets
}

func TestGetSchemaCacheHit(t *testing.T) {
	t.Parallel()
	srv, gets := cachingTestRegistry(t)
	sr, err := NewSchemaRegistry(nil, &SchemaRegistryConfig{URL: srv.URL, EnableCaching: true})
	require.NoError(t, err)

	s1, err := sr.GetSchema(&Schema{Subject: "s-value"})
	require.NoError(t, err)
	s2, err := sr.GetSchema(&Schema{Subject: "s-value"})
	require.NoError(t, err)

	require.Equal(t, int32(1), atomic.LoadInt32(gets), "second getSchema served from cache")
	require.Equal(t, s1.ID, s2.ID)
	require.NotSame(t, s1, s2, "cache hits return independent copies")
}

func TestGetSchemaCachingDisabled(t *testing.T) {
	t.Parallel()
	srv, gets := cachingTestRegistry(t)
	sr, err := NewSchemaRegistry(nil, &SchemaRegistryConfig{URL: srv.URL}) // EnableCaching false
	require.NoError(t, err)

	_, err = sr.GetSchema(&Schema{Subject: "s-value"})
	require.NoError(t, err)
	_, err = sr.GetSchema(&Schema{Subject: "s-value"})
	require.NoError(t, err)
	require.Equal(t, int32(2), atomic.LoadInt32(gets), "no caching: both calls hit the registry")
}

func TestGetSchemaVersionsCachedSeparately(t *testing.T) {
	t.Parallel()
	srv, gets := cachingTestRegistry(t)
	sr, err := NewSchemaRegistry(nil, &SchemaRegistryConfig{URL: srv.URL, EnableCaching: true})
	require.NoError(t, err)

	_, _ = sr.GetSchema(&Schema{Subject: "s-value"})             // latest
	_, _ = sr.GetSchema(&Schema{Subject: "s-value", Version: 1}) // explicit v1
	require.Equal(t, int32(2), atomic.LoadInt32(gets), "latest and explicit version are distinct keys")
	// Repeats are cache hits.
	_, _ = sr.GetSchema(&Schema{Subject: "s-value"})
	_, _ = sr.GetSchema(&Schema{Subject: "s-value", Version: 1})
	require.Equal(t, int32(2), atomic.LoadInt32(gets))
}

func TestCreateSchemaDoesNotSeedCache(t *testing.T) {
	t.Parallel()
	srv, gets := cachingTestRegistry(t)
	sr, err := NewSchemaRegistry(nil, &SchemaRegistryConfig{URL: srv.URL, EnableCaching: true})
	require.NoError(t, err)

	_, err = sr.CreateSchema(&Schema{Subject: "s-value", Schema: `"string"`, SchemaType: "STRING"})
	require.NoError(t, err)
	_, err = sr.GetSchema(&Schema{Subject: "s-value"})
	require.NoError(t, err)
	require.Equal(t, int32(1), atomic.LoadInt32(gets), "createSchema did not seed; getSchema still fetched")
}

func TestCachedLatestStableAfterCreate(t *testing.T) {
	t.Parallel()
	srv, gets := cachingTestRegistry(t)
	sr, err := NewSchemaRegistry(nil, &SchemaRegistryConfig{URL: srv.URL, EnableCaching: true})
	require.NoError(t, err)

	first, err := sr.GetSchema(&Schema{Subject: "s-value"}) // caches latest (v1)
	require.NoError(t, err)
	_, err = sr.CreateSchema(&Schema{Subject: "s-value", Schema: `"string"`, SchemaType: "STRING"})
	require.NoError(t, err)
	again, err := sr.GetSchema(&Schema{Subject: "s-value"}) // served from cache
	require.NoError(t, err)

	require.Equal(t, int32(1), atomic.LoadInt32(gets), "createSchema did not refresh the cached latest")
	require.Equal(t, first.Version, again.Version)
}

func TestGetSchemaHitIsIndependentCopy(t *testing.T) {
	t.Parallel()
	srv, _ := cachingTestRegistry(t)
	sr, err := NewSchemaRegistry(nil, &SchemaRegistryConfig{URL: srv.URL, EnableCaching: true})
	require.NoError(t, err)

	s1, err := sr.GetSchema(&Schema{Subject: "s-value"})
	require.NoError(t, err)
	s1.Version = 999 // mutate the returned schema
	s2, err := sr.GetSchema(&Schema{Subject: "s-value"})
	require.NoError(t, err)
	require.NotEqual(t, 999, s2.Version, "mutating a returned schema must not corrupt the cache")
}

func TestParsedAvroReuse(t *testing.T) {
	t.Parallel()
	// Go through the real constructor: standalone construction must initialize
	// the always-on parsed-Avro cache (a regression there would fail here).
	sr, err := NewSchemaRegistry(nil, nil)
	require.NoError(t, err)
	const s = `{"type":"record","name":"R","fields":[{"name":"a","type":"int"}]}`

	p1, err := sr.parsedAvro(s)
	require.NoError(t, err)
	p2, err := sr.parsedAvro(s)
	require.NoError(t, err)
	require.True(t, p1 == p2, "same schema string returns the same parsed value")

	// A parse failure is not cached: it still errors on repeat.
	_, err = sr.parsedAvro(`{not valid`)
	require.Error(t, err)
	_, err = sr.parsedAvro(`{not valid`)
	require.Error(t, err)
}

func TestStandaloneSerializeReusesParsedAvro(t *testing.T) {
	t.Parallel()
	// End-to-end: a standalone client serializing repeatedly with the same
	// inline Avro schema parses it once (always-on reuse, no enableCaching).
	sr, err := NewSchemaRegistry(nil, nil)
	require.NoError(t, err)
	schema := &Schema{
		Schema:     `{"type":"record","name":"R","fields":[{"name":"a","type":"int"}]}`,
		SchemaType: "AVRO",
	}
	_, err = sr.serialize(map[string]any{"a": 1}, "AVRO", schema)
	require.NoError(t, err)
	_, err = sr.serialize(map[string]any{"a": 2}, "AVRO", schema)
	require.NoError(t, err)
	require.Len(t, sr.avroCache, 1, "schema parsed once and reused across serialize calls")
}

const (
	avroRecordNS   = `{"type":"record","name":"User","namespace":"com.example","fields":[{"name":"id","type":"int"}]}`
	avroRecordBare = `{"type":"record","name":"User","fields":[{"name":"id","type":"int"}]}`
	avroEnumNS     = `{"type":"enum","name":"Color","namespace":"com.example","symbols":["RED","GREEN"]}`
)

func TestGetSubjectNameStrategies(t *testing.T) {
	t.Parallel()
	sr, err := NewSchemaRegistry(nil, nil)
	require.NoError(t, err)

	cases := []struct{ name, strategy, element, schema, want string }{
		{"topic-value", topicNameStrategy, elementValue, "", "t-value"},
		{"topic-key", topicNameStrategy, elementKey, "", "t-key"},
		{"empty-defaults-to-topic", "", elementValue, "", "t-value"},
		{"topic-ignores-schema", topicNameStrategy, elementValue, avroRecordNS, "t-value"},
		{"record-namespaced", recordNameStrategy, elementValue, avroRecordNS, "com.example.User"},
		{"record-no-namespace", recordNameStrategy, elementValue, avroRecordBare, "User"},
		{"record-enum", recordNameStrategy, elementValue, avroEnumNS, "com.example.Color"},
		{"topic-record", topicRecordNameStrategy, elementValue, avroRecordNS, "t-com.example.User"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := sr.GetSubjectName(&SubjectNameConfig{
				Topic: "t", Element: c.element, SubjectNameStrategy: c.strategy, Schema: c.schema,
			})
			require.NoError(t, err)
			require.Equal(t, c.want, got)
		})
	}
}

func TestGetSubjectNameErrors(t *testing.T) {
	t.Parallel()
	sr, err := NewSchemaRegistry(nil, nil)
	require.NoError(t, err)

	recordStrategies := []string{recordNameStrategy, topicRecordNameStrategy}

	// Empty schema errors for both record strategies.
	for _, s := range recordStrategies {
		_, err := sr.GetSubjectName(&SubjectNameConfig{Topic: "t", Element: elementValue, SubjectNameStrategy: s})
		require.Error(t, err, "empty schema, strategy %s", s)
	}
	// Non-Avro / unnamed schema errors for both record strategies.
	for _, s := range recordStrategies {
		for _, bad := range []string{`{"type":"object","title":"User"}`, `"string"`} {
			_, err := sr.GetSubjectName(&SubjectNameConfig{
				Topic: "t", Element: elementValue, SubjectNameStrategy: s, Schema: bad,
			})
			require.Error(t, err, "strategy %s, schema %s", s, bad)
		}
	}
	// Unknown strategy errors.
	_, err = sr.GetSubjectName(&SubjectNameConfig{
		Topic: "t", Element: elementValue, SubjectNameStrategy: "BogusStrategy", Schema: avroRecordNS,
	})
	require.Error(t, err)
}

func TestSchemaRegistryTLS(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	caPEM := pemStr("CERTIFICATE", srv.Certificate().Raw)

	// Trusted via serverCaPem: connectivity check succeeds.
	_, err := NewSchemaRegistry(nil, &SchemaRegistryConfig{URL: srv.URL, TLS: &TLSConfig{ServerCaPem: caPEM}})
	require.NoError(t, err)

	// Untrusted (no CA, verify on): fails.
	_, err = NewSchemaRegistry(nil, &SchemaRegistryConfig{URL: srv.URL, TLS: &TLSConfig{}})
	require.Error(t, err)

	// insecureSkipTlsVerify: connects despite the untrusted cert.
	_, err = NewSchemaRegistry(nil, &SchemaRegistryConfig{URL: srv.URL, TLS: &TLSConfig{InsecureSkipTLSVerify: true}})
	require.NoError(t, err)
}

// Test wire format encoding/decoding (task 3.3)
func TestWireFormatRoundTrip(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		schemaID int
	}{
		{"zero", 0},
		{"one", 1},
		{"max_int32", math.MaxInt32},
		{"typical", 12345},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Encode
			encoded, err := encodeWireFormat(tt.schemaID)
			require.NoError(t, err)
			require.Equal(t, 5, len(encoded))
			require.Equal(t, byte(0x00), encoded[0])

			// Decode
			decoded, remaining, err := decodeWireFormat(encoded)
			require.NoError(t, err)
			require.Equal(t, tt.schemaID, decoded)
			require.Equal(t, 0, len(remaining))
		})
	}
}

func TestBytesToUint8Array(t *testing.T) {
	t.Parallel()
	rt := sobek.New()
	v, err := bytesToUint8Array(rt, []byte{1, 2, 3})
	require.NoError(t, err)
	require.NoError(t, rt.Set("v", v))
	res, err := rt.RunString(`v instanceof Uint8Array && v.length === 3 && v[0] === 1 && v[2] === 3`)
	require.NoError(t, err)
	require.True(t, res.ToBoolean(), "expected a Uint8Array with the original bytes")
}

func TestReqContextNilVU(t *testing.T) {
	t.Parallel()
	// Standalone mode / unit tests construct without a VU; reqContext must
	// still return a usable (non-nil) context instead of panicking.
	sr := &SchemaRegistry{config: nil}
	require.NotNil(t, sr.reqContext())
	require.NotNil(t, vuContext(nil))
}

func TestEncodeWireFormatOutOfRange(t *testing.T) {
	t.Parallel()
	for _, id := range []int{-1, math.MaxUint32 + 1} {
		_, err := encodeWireFormat(id)
		require.Error(t, err, "schema ID %d should be rejected", id)
	}
}

func TestProtobufUnsupported(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}
	_, serErr := sr.serialize(map[string]any{"x": 1}, "PROTOBUF", &Schema{})
	require.ErrorIs(t, serErr, errProtobufUnsupported)
	_, desErr := sr.deserialize([]byte{1, 2, 3}, "PROTOBUF", &Schema{})
	require.ErrorIs(t, desErr, errProtobufUnsupported)
}

func TestDecodeWireFormatErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		data   []byte
		errMsg string
	}{
		{"too_short", []byte{0x00, 0x00, 0x00}, "too short"},
		{"invalid_magic", []byte{0x01, 0x00, 0x00, 0x00, 0x00}, "invalid magic byte"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := decodeWireFormat(tt.data)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

// Test STRING serdes (task 4.5)
func TestStringSerdes(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}

	tests := []struct {
		name  string
		value string
	}{
		{"empty", ""},
		{"ascii", "hello world"},
		{"special_chars", "hello\nworld\t!"},
		{"unicode", "こんにちは世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Serialize
			encoded, err := sr.serialize(tt.value, "STRING", nil)
			require.NoError(t, err)

			// Deserialize
			decoded, err := sr.deserialize(encoded, "STRING", nil)
			require.NoError(t, err)
			require.Equal(t, tt.value, decoded)
		})
	}
}

func TestStringDeserializeInvalidUTF8(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}
	invalidUTF8 := []byte{0xFF, 0xFE}

	_, err := sr.deserialize(invalidUTF8, "STRING", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid UTF-8")
}

// Test BYTES serdes (task 4.5)
func TestBytesSerdes(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}

	tests := []struct {
		name  string
		value []byte
	}{
		{"empty", []byte{}},
		{"binary", []byte{0x00, 0x01, 0x02, 0xFF}},
		{"text_as_bytes", []byte("hello")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Serialize
			encoded, err := sr.serialize(tt.value, "BYTES", nil)
			require.NoError(t, err)
			require.Equal(t, tt.value, encoded)

			// Deserialize
			decoded, err := sr.deserialize(encoded, "BYTES", nil)
			require.NoError(t, err)
			require.Equal(t, tt.value, decoded)
		})
	}
}

// Test Avro serdes (tasks 5.3-5.4)
func TestAvroSerdes(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}

	// Simple record schema
	schema := &Schema{
		ID:         0,
		Schema:     `{"type":"record","name":"Test","fields":[{"name":"name","type":"string"},{"name":"age","type":"int"}]}`,
		SchemaType: "AVRO",
	}

	data := map[string]any{
		"name": "Alice",
		"age":  30,
	}

	// Serialize
	encoded, err := sr.serialize(data, "AVRO", schema)
	require.NoError(t, err)
	require.Greater(t, len(encoded), 0)

	// Deserialize
	decoded, err := sr.deserialize(encoded, "AVRO", schema)
	require.NoError(t, err)

	// Verify round-trip (Avro preserves types)
	decodedMap := decoded.(map[string]any)
	require.Equal(t, "Alice", decodedMap["name"])
	// Age can be int or float64 depending on Avro version
	switch v := decodedMap["age"].(type) {
	case int:
		require.Equal(t, 30, v)
	case float64:
		require.Equal(t, float64(30), v)
	default:
		t.Fatalf("unexpected type for age: %T", v)
	}
}

func TestAvroWireFormatRoundTrip(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}

	schema := &Schema{
		ID:         12345,
		Schema:     `{"type":"record","name":"Test","fields":[{"name":"id","type":"int"}]}`,
		SchemaType: "AVRO",
	}

	data := map[string]any{"id": 42}

	// Serialize (with wire format envelope)
	encoded, err := sr.serialize(data, "AVRO", schema)
	require.NoError(t, err)

	// Check wire format envelope present
	require.GreaterOrEqual(t, len(encoded), 5)
	require.Equal(t, byte(0x00), encoded[0])

	// Deserialize (strips envelope, verifies schema ID)
	decoded, err := sr.deserialize(encoded, "AVRO", schema)
	require.NoError(t, err)
	decodedMap := decoded.(map[string]any)
	// Avro may preserve int or convert to float64 depending on version
	switch v := decodedMap["id"].(type) {
	case int:
		require.Equal(t, 42, v)
	case float64:
		require.Equal(t, float64(42), v)
	default:
		t.Fatalf("unexpected type for id: %T", v)
	}
}

func TestAvroSchemaIDMismatch(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}

	schema := &Schema{
		ID:         12345,
		Schema:     `{"type":"record","name":"Test","fields":[{"name":"id","type":"int"}]}`,
		SchemaType: "AVRO",
	}

	data := map[string]any{"id": 42}

	// Serialize with schema ID 12345
	encoded, err := sr.serialize(data, "AVRO", schema)
	require.NoError(t, err)

	// Try to deserialize with different schema ID
	wrongSchema := &Schema{
		ID:         99999,
		Schema:     schema.Schema,
		SchemaType: "AVRO",
	}

	_, err = sr.deserialize(encoded, "AVRO", wrongSchema)
	require.Error(t, err)
	require.Contains(t, err.Error(), "schema ID mismatch")
}

// Test JSON serdes (tasks 6.3-6.5)
func TestJSONSerdes(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}

	schema := &Schema{
		ID:         0,
		Schema:     `{"type":"object","properties":{"name":{"type":"string"},"age":{"type":"integer"}},"required":["name"]}`,
		SchemaType: "JSON",
	}

	data := map[string]any{
		"name": "Bob",
		"age":  25,
	}

	// Serialize
	encoded, err := sr.serialize(data, "JSON", schema)
	require.NoError(t, err)

	// Deserialize
	decoded, err := sr.deserialize(encoded, "JSON", schema)
	require.NoError(t, err)
	decodedMap := decoded.(map[string]any)
	require.Equal(t, "Bob", decodedMap["name"])
	require.Equal(t, float64(25), decodedMap["age"])
}

func TestJSONRequiredFieldValidation(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}

	schema := &Schema{
		Schema:     `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`,
		SchemaType: "JSON",
	}

	// Missing required field
	data := map[string]any{
		"age": 25,
	}

	_, err := sr.serialize(data, "JSON", schema)
	require.Error(t, err)
	require.Contains(t, err.Error(), "required field missing")
}

func TestJSONWireFormatRoundTrip(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}

	schema := &Schema{
		ID:         54321,
		Schema:     `{"type":"object","properties":{"msg":{"type":"string"}},"required":["msg"]}`,
		SchemaType: "JSON",
	}

	data := map[string]any{
		"msg": "hello",
	}

	// Serialize (with wire format envelope)
	encoded, err := sr.serialize(data, "JSON", schema)
	require.NoError(t, err)

	// Check wire format envelope
	require.GreaterOrEqual(t, len(encoded), 5)
	require.Equal(t, byte(0x00), encoded[0])

	// Deserialize (strips envelope, verifies schema ID)
	decoded, err := sr.deserialize(encoded, "JSON", schema)
	require.NoError(t, err)
	decodedMap := decoded.(map[string]any)
	require.Equal(t, "hello", decodedMap["msg"])
}

func TestJSONMalformedBytes(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}

	schema := &Schema{
		Schema:     `{}`,
		SchemaType: "JSON",
	}

	_, err := sr.deserialize([]byte(`{invalid json`), "JSON", schema)
	require.Error(t, err)
	require.Contains(t, err.Error(), "JSON decode failed")
}

// Error cases
func TestUnsupportedSchemaType(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}

	_, err := sr.serialize("data", "UNKNOWN", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsupported schema type")
}

func TestStringSerializeWrongType(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}

	_, err := sr.serialize(123, "STRING", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "STRING serialize expects string")
}

func TestBytesSerializeWrongType(t *testing.T) {
	t.Parallel()
	sr := &SchemaRegistry{config: nil}

	_, err := sr.serialize("not bytes", "BYTES", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "BYTES serialize expects []byte")
}
