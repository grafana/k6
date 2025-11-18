// Example: Using URL-based secret source
//
// File-based configuration:
// k6 run --secret-source=url=config=examples/secrets/url-config.json examples/secrets/url-source.test.js
//
// With local mock server and custom retry configuration:
// k6 run --secret-source=url=config=examples/secrets/url-local.json examples/secrets/url-source.test.js
//
// Inline configuration (no config file needed):
// k6 run --secret-source='url=urlTemplate=http://localhost:8080/secrets/{key}' examples/secrets/url-source.test.js
//
// Inline with headers:
// k6 run --secret-source='url=urlTemplate=https://api.example.com/{key},headers.Authorization=Bearer token123' examples/secrets/url-source.test.js
//
// Mixed file and inline (inline overrides file):
// k6 run --secret-source='url=config=examples/secrets/url-config.json,timeout=60s,maxRetries=5' examples/secrets/url-source.test.js

import secrets from "k6/secrets";

export default async () => {
	// Get secret from URL source
	const mySecret = await secrets.get("my-secret-key");
	console.log(`Retrieved secret ${mySecret}`);

	// Use the secret in your test
	// Example: http.get("https://api.example.com", {
	//   headers: { "X-API-Key": mySecret }
	// });
};
