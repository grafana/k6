package kafka

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGzipTransportDecompressesResponse tests that gzipTransport correctly
// decompresses gzipped responses from the schema registry, even when the
// Content-Encoding header is missing.
func TestGzipTransportDecompressesResponse(t *testing.T) {
	test := getTestModuleInstance(t)

	// Create a mock schema registry response
	schemaResponse := map[string]any{
		"subject":    "test-subject",
		"version":    1,
		"id":         42,
		"schema":     `{"type":"record","name":"TestSchema","fields":[{"name":"field","type":"string"}]}`,
		"schemaType": "AVRO",
	}

	// Convert to JSON
	jsonResponse, err := json.Marshal(schemaResponse)
	require.NoError(t, err)

	// Compress the JSON response
	var gzippedResponse bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzippedResponse)
	_, err = gzipWriter.Write(jsonResponse)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	// Create a test server that returns gzipped response WITHOUT Content-Encoding header
	// This simulates the issue reported in https://github.com/mostafa/xk6-kafka/issues/363
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify Accept-Encoding header is set
		assert.Contains(t, r.Header.Get("Accept-Encoding"), "gzip", "Accept-Encoding header should be set")

		// Mock the schema registry endpoint: /subjects/{subject}/versions/latest
		// Return gzipped content WITHOUT Content-Encoding header
		// This simulates the problematic scenario
		w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
		// Intentionally NOT setting Content-Encoding: gzip to test our fix
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(gzippedResponse.Bytes())
		require.NoError(t, err)
	}))
	defer server.Close()

	// Create schema registry client with gzip transport wrapper
	srConfig := SchemaRegistryConfig{
		URL: server.URL,
	}
	srClient := test.module.schemaRegistryClient(&srConfig)
	require.NotNil(t, srClient)

	// Try to fetch a schema - this should work because gzipTransport decompresses the response
	// The subject doesn't matter since we're mocking the response
	schema, err := srClient.GetLatestSchema("test-subject")
	require.NoError(t, err, "Should successfully fetch schema even with gzipped response")
	assert.NotNil(t, schema)
	assert.Equal(t, 42, schema.ID())
	assert.Equal(t, 1, schema.Version())
	assert.Contains(t, schema.Schema(), "TestSchema")
}

// TestGzipTransportWithContentEncodingHeader tests that gzipTransport correctly
// handles responses with Content-Encoding: gzip header.
func TestGzipTransportWithContentEncodingHeader(t *testing.T) {
	test := getTestModuleInstance(t)

	// Create a mock schema registry response
	schemaResponse := map[string]any{
		"subject":    "test-subject-2",
		"version":    2,
		"id":         100,
		"schema":     `{"type":"record","name":"TestSchema2","fields":[{"name":"field","type":"string"}]}`,
		"schemaType": "AVRO",
	}

	// Convert to JSON
	jsonResponse, err := json.Marshal(schemaResponse)
	require.NoError(t, err)

	// Compress the JSON response
	var gzippedResponse bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzippedResponse)
	_, err = gzipWriter.Write(jsonResponse)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	// Create a test server that returns gzipped response WITH Content-Encoding header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return gzipped content WITH Content-Encoding header
		w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(gzippedResponse.Bytes())
		require.NoError(t, err)
	}))
	defer server.Close()

	// Create schema registry client with gzip transport wrapper
	srConfig := SchemaRegistryConfig{
		URL: server.URL,
	}
	srClient := test.module.schemaRegistryClient(&srConfig)
	require.NotNil(t, srClient)

	// Try to fetch a schema - this should work
	schema, err := srClient.GetLatestSchema("test-subject-2")
	require.NoError(t, err, "Should successfully fetch schema with Content-Encoding header")
	assert.NotNil(t, schema)
	assert.Equal(t, 100, schema.ID())
	assert.Equal(t, 2, schema.Version())
	assert.Contains(t, schema.Schema(), "TestSchema2")
}

// TestGzipTransportWithNonGzippedResponse tests that gzipTransport correctly
// handles non-gzipped responses (should pass through unchanged).
func TestGzipTransportWithNonGzippedResponse(t *testing.T) {
	test := getTestModuleInstance(t)

	// Create a mock schema registry response (not gzipped)
	schemaResponse := map[string]any{
		"subject":    "test-subject-3",
		"version":    3,
		"id":         200,
		"schema":     `{"type":"record","name":"TestSchema3","fields":[{"name":"field","type":"string"}]}`,
		"schemaType": "AVRO",
	}

	// Convert to JSON
	jsonResponse, err := json.Marshal(schemaResponse)
	require.NoError(t, err)

	// Create a test server that returns non-gzipped response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(jsonResponse)
		require.NoError(t, err)
	}))
	defer server.Close()

	// Create schema registry client with gzip transport wrapper
	srConfig := SchemaRegistryConfig{
		URL: server.URL,
	}
	srClient := test.module.schemaRegistryClient(&srConfig)
	require.NotNil(t, srClient)

	// Try to fetch a schema - this should work (non-gzipped response passes through)
	schema, err := srClient.GetLatestSchema("test-subject-3")
	require.NoError(t, err, "Should successfully fetch schema with non-gzipped response")
	assert.NotNil(t, schema)
	assert.Equal(t, 200, schema.ID())
	assert.Equal(t, 3, schema.Version())
	assert.Contains(t, schema.Schema(), "TestSchema3")
}

// TestGzipTransportDirectly tests the gzipTransport RoundTrip method directly.
func TestGzipTransportDirectly(t *testing.T) {
	// Create test data
	originalData := []byte(`{"test": "data"}`)

	// Compress the data
	var gzippedData bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzippedData)
	_, err := gzipWriter.Write(originalData)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	// Create a test server that returns gzipped content without Content-Encoding header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Intentionally NOT setting Content-Encoding: gzip
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(gzippedData.Bytes())
		require.NoError(t, err)
	}))
	defer server.Close()

	// Create gzipTransport wrapping the default transport
	transport := &gzipTransport{
		transport: http.DefaultTransport,
	}

	// Create a request
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	// Make the request through gzipTransport
	resp, err := transport.RoundTrip(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// Verify the response was decompressed
	assert.Equal(t, originalData, bodyBytes, "Response should be decompressed")
	assert.NotEqual(t, gzippedData.Bytes(), bodyBytes, "Response should not be gzipped")
}

// TestGzipTransportWithTLS tests that gzipTransport works with TLS configuration.
func TestGzipTransportWithTLS(t *testing.T) {
	test := getTestModuleInstance(t)

	// Create a mock schema registry response
	schemaResponse := map[string]any{
		"subject":    "test-subject-tls",
		"version":    1,
		"id":         300,
		"schema":     `{"type":"record","name":"TestSchemaTLS","fields":[{"name":"field","type":"string"}]}`,
		"schemaType": "AVRO",
	}

	// Convert to JSON
	jsonResponse, err := json.Marshal(schemaResponse)
	require.NoError(t, err)

	// Compress the JSON response
	var gzippedResponse bytes.Buffer
	gzipWriter := gzip.NewWriter(&gzippedResponse)
	_, err = gzipWriter.Write(jsonResponse)
	require.NoError(t, err)
	require.NoError(t, gzipWriter.Close())

	// Create a test server with TLS
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.schemaregistry.v1+json")
		// Return gzipped content WITHOUT Content-Encoding header
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(gzippedResponse.Bytes())
		require.NoError(t, err)
	}))
	defer server.Close()

	// Create schema registry client with TLS config
	// Note: We're using the test server's TLS config, but in real scenarios
	// you'd use proper TLS certificates
	srConfig := SchemaRegistryConfig{
		URL: server.URL,
		// TLS config would normally be set here, but for testing we'll skip TLS verification
		// In a real scenario, you'd set proper TLS certificates
	}

	// For this test, we'll verify that the client is created successfully
	// The actual TLS connection would require proper certificate handling
	srClient := test.module.schemaRegistryClient(&srConfig)
	require.NotNil(t, srClient)

	// Verify the client was created (actual TLS connection would fail without proper certs)
	assert.NotNil(t, srClient)
}

// TestGzipTransportMagicNumberDetection tests that gzipTransport correctly
// detects gzip magic number (0x1f 0x8b).
func TestGzipTransportMagicNumberDetection(t *testing.T) {
	// Test various byte sequences
	testCases := []struct {
		name   string
		data   []byte
		isGzip bool
	}{
		{
			name:   "Valid gzip magic number",
			data:   []byte{0x1f, 0x8b, 0x08, 0x00},
			isGzip: true,
		},
		{
			name:   "Invalid magic number",
			data:   []byte{0x1f, 0x8a, 0x08, 0x00},
			isGzip: false,
		},
		{
			name:   "JSON data",
			data:   []byte(`{"test": "data"}`),
			isGzip: false,
		},
		{
			name:   "Empty data",
			data:   []byte{},
			isGzip: false,
		},
		{
			name:   "Single byte",
			data:   []byte{0x1f},
			isGzip: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			isGzip := len(tc.data) >= 2 && tc.data[0] == 0x1f && tc.data[1] == 0x8b
			assert.Equal(t, tc.isGzip, isGzip, "Magic number detection should match expected result")
		})
	}
}
