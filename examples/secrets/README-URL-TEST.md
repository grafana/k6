# URL Secret Source Testing

This directory contains a mock Google Secret Manager (GSM) server and test files for the URL secret source implementation.

## Quick Start

### 1. Start the Mock GSM Server

In one terminal, run:

```bash
cd examples/secrets
go run mock_gsm_server.go
```

You should see:
```
Starting mock GSM server on :8080
Available secrets: [api-key database-pass jwt-secret stripe-key github-token test-secret my-secret another-secret]
Expected Authorization header: Bearer YOUR_GSM_TOKEN_HERE

Example curl command:
  curl -H "Authorization: Bearer YOUR_GSM_TOKEN_HERE" http://localhost:8080/secrets/api-key/decrypt
```

### 2. Test with curl

In another terminal, test the mock server directly:

```bash
# Successful request
curl -H "Authorization: Bearer YOUR_GSM_TOKEN_HERE" \
  http://localhost:8080/secrets/api-key/decrypt

# Expected response:
# {"plaintext":"super-secret-api-key-12345","name":"projects/test-project/secrets/api-key"}

# Test without auth (should fail)
curl http://localhost:8080/secrets/api-key/decrypt

# Test with wrong token (should fail)
curl -H "Authorization: Bearer WRONG_TOKEN" \
  http://localhost:8080/secrets/api-key/decrypt

# Test non-existent secret (should fail)
curl -H "Authorization: Bearer YOUR_GSM_TOKEN_HERE" \
  http://localhost:8080/secrets/nonexistent/decrypt
```

### 3. Test with k6 (when implementation is complete)

Build k6 with the URL secret source:

```bash
cd /Users/vicenteortegatorres/projects/k6
make build
```

Run the test script:

```bash
./k6 run --secrets url=config=examples/secrets/url-gsm-local.json \
  examples/secrets/url-test.js
```

## Mock Server Features

The mock GSM server includes:

- ✅ **Authentication**: Validates Bearer token in Authorization header
- ✅ **Path Parsing**: Handles `/secrets/{key}/decrypt` format
- ✅ **JSON Response**: Returns secrets in GSM-compatible format with `plaintext` field
- ✅ **Error Handling**: Proper HTTP status codes for various error scenarios
- ✅ **Logging**: Detailed request/response logging for debugging
- ✅ **Health Check**: `/health` endpoint for monitoring

## Available Mock Secrets

| Secret Key       | Secret Value (example)           |
|------------------|----------------------------------|
| api-key          | super-secret-api-key-12345       |
| database-pass    | db-password-xyz789               |
| jwt-secret       | jwt-signing-key-abc123           |
| stripe-key       | sk_test_mock_stripe_key          |
| github-token     | ghp_mock_github_token_12345      |
| test-secret      | this-is-a-test-secret            |
| my-secret        | my-secret-value                  |
| another-secret   | another-secret-value             |

## Configuration Files

- **url-gsm-local.json**: Points to localhost mock server
- **url-gsm-config.json**: Production-style config for actual GSM proxy
- **url-config.json**: Generic URL secret source config

## Testing Different Scenarios

### Success Cases
```bash
# Basic secret retrieval
curl -H "Authorization: Bearer YOUR_GSM_TOKEN_HERE" \
  http://localhost:8080/secrets/api-key/decrypt

# Different secrets
curl -H "Authorization: Bearer YOUR_GSM_TOKEN_HERE" \
  http://localhost:8080/secrets/jwt-secret/decrypt
```

### Error Cases
```bash
# 401: Missing authorization
curl http://localhost:8080/secrets/api-key/decrypt

# 401: Invalid token
curl -H "Authorization: Bearer INVALID" \
  http://localhost:8080/secrets/api-key/decrypt

# 404: Secret not found
curl -H "Authorization: Bearer YOUR_GSM_TOKEN_HERE" \
  http://localhost:8080/secrets/nonexistent/decrypt

# 405: Wrong HTTP method
curl -X POST -H "Authorization: Bearer YOUR_GSM_TOKEN_HERE" \
  http://localhost:8080/secrets/api-key/decrypt
```

## Extending the Mock Server

To add more mock secrets, edit `mock_gsm_server.go`:

```go
var mockSecrets = map[string]string{
	"api-key":        "super-secret-api-key-12345",
	"your-new-key":   "your-new-secret-value",  // Add here
}
```

Then restart the server.

## Using with k6 Tests

Example k6 script using URL secrets:

```javascript
import { getSecret } from 'k6/x/secrets';
import http from 'k6/http';

export default function () {
  const apiKey = getSecret('api-key');

  http.get('https://api.example.com/data', {
    headers: {
      'Authorization': `Bearer ${apiKey}`,
    },
  });
}
```

Run with:
```bash
./k6 run --secrets url=config=examples/secrets/url-gsm-local.json script.js
```
