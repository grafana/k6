// Example: Using URL-based secret source
//
// For generic API:
// k6 run --secret-source=url=config=examples/secrets/url-config.json examples/secrets/url-source.test.js
//
// For Grafana Secrets Manager (GSM):
// k6 run --secret-source=url=config=examples/secrets/url-gsm-config.json examples/secrets/url-source.test.js

import secrets from "k6/secrets";

export default async () => {
	// Get secret from URL source
	const mySecret = await secrets.get("my-secret-key");
	console.log("Retrieved secret (value redacted)");

	// Use the secret in your test
	// Example: http.get("https://api.example.com", {
	//   headers: { "X-API-Key": mySecret }
	// });
};
