// Package main implements a mock HTTP secrets server for testing k6's URL secret source.
//
// The server mimics a secret management API like Vault or Google Secret Manager,
// providing HTTP endpoints to retrieve secrets.
//
// Usage:
//
//	go run examples/secrets/mock_server.go
//
// The server listens on localhost:8888 by default and requires Bearer token
// authentication.
//
// Example k6 command with inline configuration:
//
//	./k6 run --secret-source='url=urlTemplate=http://localhost:8888/secrets/{key}/decrypt,\
//	  headers.Authorization=Bearer YOUR_API_TOKEN_HERE,responsePath=plaintext' \
//	  examples/secrets/url-test.js
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	serverAddr = ":8888"
	// expectedToken is the test bearer token for authentication in examples
	expectedToken = "Bearer YOUR_API_TOKEN_HERE" //nolint:gosec // Test token for examples
	readTimeout   = 10 * time.Second
	writeTimeout  = 10 * time.Second
)

// Response structure for secret API
type secretResponse struct {
	Plaintext string `json:"plaintext"`
	Name      string `json:"name"`
}

func main() {
	// Mock secret store
	mockSecrets := map[string]string{
		"api-key":        "super-secret-api-key-12345",
		"database-pass":  "db-password-xyz789",
		"jwt-secret":     "jwt-signing-key-abc123",
		"stripe-key":     "sk_test_mock_stripe_key",
		"github-token":   "ghp_mock_github_token_12345",
		"test-secret":    "this-is-a-test-secret",
		"my-secret":      "my-secret-value",
		"my-secret-key":  "my-secret-key-value",
		"another-secret": "another-secret-value",
	}

	mux := http.NewServeMux()

	// Handle secret decryption requests
	mux.HandleFunc("/secrets/", createSecretHandler(mockSecrets))

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"healthy"}`)
	})

	server := &http.Server{
		Addr:         serverAddr,
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	log.Printf("Starting mock secrets API server on %s", serverAddr)
	log.Printf("Available secrets: %v", getSecretKeys(mockSecrets))
	log.Printf("Expected Authorization header: %s", expectedToken)
	log.Println()
	log.Println("Example curl command:")
	log.Println(`  curl -H "Authorization: Bearer YOUR_API_TOKEN_HERE" http://localhost:8888/secrets/api-key/decrypt`)
	log.Println()
	log.Println("Example k6 command with inline config:")
	log.Println(`  ./k6 run \`)
	log.Println(`    --secret-source='url=urlTemplate=http://localhost:8888/secrets/{key}/decrypt,\`)
	log.Println(`      headers.Authorization=Bearer YOUR_API_TOKEN_HERE,responsePath=plaintext' \`)
	log.Println(`    examples/secrets/url-test.js`)
	log.Println()

	// Get the actual port after server starts
	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", serverAddr)
	if err != nil {
		log.Fatalf("Server failed to listen: %v", err)
	}

	log.Printf("Server listening at %v\n", ln.Addr())

	if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

func createSecretHandler(mockSecrets map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Log incoming request
		log.Printf("%s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

		// Check HTTP method
		if r.Method != http.MethodGet {
			log.Printf("  ❌ Invalid method: %s", r.Method)
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		// Check Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Println("  ❌ Missing Authorization header")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"error":"missing authorization header"}`)
			return
		}

		// Validate Bearer token
		if authHeader != expectedToken {
			log.Printf("  ❌ Invalid token: %s", authHeader)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"error":"invalid authorization token"}`)
			return
		}

		// Extract secret key from URL path
		// Expected format: /secrets/{key}/decrypt
		path := strings.TrimPrefix(r.URL.Path, "/secrets/")
		path = strings.TrimSuffix(path, "/decrypt")

		if path == "" || path == r.URL.Path {
			log.Println("  ❌ Invalid path format")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprint(w, `{"error":"invalid path format, expected /secrets/{key}/decrypt"}`)
			return
		}

		secretKey := path

		// Look up secret
		secretValue, exists := mockSecrets[secretKey]
		if !exists {
			log.Printf("  ❌ Secret not found: %s", secretKey)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = fmt.Fprintf(w, `{"error":"secret %q not found"}`, secretKey)
			return
		}

		// Build response
		response := secretResponse{
			Plaintext: secretValue,
			Name:      fmt.Sprintf("projects/test-project/secrets/%s", secretKey),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("  ❌ Failed to encode response: %v", err)
			return
		}

		log.Printf("  ✅ Returned secret: %s (length: %d)", secretKey, len(secretValue))
	}
}

func getSecretKeys(secrets map[string]string) []string {
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	return keys
}
