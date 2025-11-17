package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// Response structure matching GSM API format
type secretResponse struct {
	Plaintext string `json:"plaintext"`
	Name      string `json:"name"`
}

// Mock secret store - in real GSM, these would be encrypted secrets
var mockSecrets = map[string]string{
	"api-key":        "super-secret-api-key-12345",
	"database-pass":  "db-password-xyz789",
	"jwt-secret":     "jwt-signing-key-abc123",
	"stripe-key":     "sk_test_mock_stripe_key",
	"github-token":   "ghp_mock_github_token_12345",
	"test-secret":    "this-is-a-test-secret",
	"my-secret":      "my-secret-value",
	"another-secret": "another-secret-value",
}

func main() {
	mux := http.NewServeMux()

	// Handle secret decryption requests
	mux.HandleFunc("/secrets/", handleSecretRequest)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})

	addr := ":8888"
	log.Printf("Starting mock GSM server on %s", addr)
	log.Printf("Available secrets: %v", getSecretKeys())
	log.Printf("Expected Authorization header: Bearer YOUR_GSM_TOKEN_HERE")
	log.Println()
	log.Println("Example curl command:")
	log.Println(`  curl -H "Authorization: Bearer YOUR_GSM_TOKEN_HERE" http://localhost:8888/secrets/api-key/decrypt`)

	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func handleSecretRequest(w http.ResponseWriter, r *http.Request) {
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
		_, _ = w.Write([]byte(`{"error":"missing authorization header"}`))
		return
	}

	// Validate Bearer token
	expectedToken := "Bearer YOUR_GSM_TOKEN_HERE"
	if authHeader != expectedToken {
		log.Printf("  ❌ Invalid token: %s", authHeader)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid authorization token"}`))
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
		_, _ = w.Write([]byte(`{"error":"invalid path format, expected /secrets/{key}/decrypt"}`))
		return
	}

	secretKey := path

	// Look up secret
	secretValue, exists := mockSecrets[secretKey]
	if !exists {
		log.Printf("  ❌ Secret not found: %s", secretKey)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"error":"secret %q not found"}`, secretKey)))
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

func getSecretKeys() []string {
	keys := make([]string, 0, len(mockSecrets))
	for k := range mockSecrets {
		keys = append(keys, k)
	}
	return keys
}
