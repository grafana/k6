// Example: Using URL-based secret source with local mock server
//
// SETUP:
// ======
// 1. Start the mock server in a separate terminal:
//    go run examples/secrets/mock_server.go
//
// 2. Run this test with one of the following configurations:
//
// USAGE OPTIONS:
// ==============
//
// Option 1: File-based configuration (using url-config.json)
// -----------------------------------------------------------
// k6 run --secret-source=url=config=examples/secrets/url-config.json \
//   examples/secrets/url-source.test.js
//
// Option 2: File-based with local mock server settings (using url-local.json)
// ----------------------------------------------------------------------------
// k6 run --secret-source=url=config=examples/secrets/url-local.json \
//   examples/secrets/url-source.test.js
//
// Option 3: Inline configuration (no config file needed)
// -------------------------------------------------------
// k6 run \
//   --secret-source='url=urlTemplate=http://localhost:8888/secrets/{key}/decrypt,\
//     headers.Authorization=Bearer YOUR_API_TOKEN_HERE,\
//     responsePath=plaintext' \
//   examples/secrets/url-source.test.js
//
// Option 4: Inline with custom timeout and retry settings
// --------------------------------------------------------
// k6 run \
//   --secret-source='url=urlTemplate=http://localhost:8888/secrets/{key}/decrypt,\
//     headers.Authorization=Bearer YOUR_API_TOKEN_HERE,\
//     responsePath=plaintext,\
//     timeout=5s,\
//     maxRetries=2' \
//   examples/secrets/url-source.test.js
//
// Option 5: Mixed - Load config file and override specific settings
// ------------------------------------------------------------------
// k6 run \
//   --secret-source='url=config=examples/secrets/url-config.json,\
//     timeout=60s,\
//     maxRetries=5' \
//   examples/secrets/url-source.test.js
//
// Option 6: Environment variable configuration
// ---------------------------------------------
// Environment variables are ALWAYS considered and follow k6's order of precedence:
// 1. Defaults (lowest priority)
// 2. Environment variables (K6_SECRET_SOURCE_URL_*)
// 3. Config file (if specified with config=path)
// 4. Inline CLI flags (highest priority)
//
// To use ONLY environment variables (no config file or inline flags):
// export K6_SECRET_SOURCE_URL_URL_TEMPLATE="http://localhost:8888/secrets/{key}/decrypt"
// export K6_SECRET_SOURCE_URL_HEADER_AUTHORIZATION="Bearer YOUR_API_TOKEN_HERE"
// export K6_SECRET_SOURCE_URL_RESPONSE_PATH="plaintext"
// k6 run --secret-source=url examples/secrets/url-source.test.js
//
// You can also combine environment variables with other options:
// export K6_SECRET_SOURCE_URL_URL_TEMPLATE="http://localhost:8888/secrets/{key}/decrypt"
// export K6_SECRET_SOURCE_URL_RESPONSE_PATH="plaintext"
// k6 run --secret-source='url=headers.Authorization=Bearer YOUR_API_TOKEN_HERE' \
//   examples/secrets/url-source.test.js
// (The Authorization header from CLI will override the env var if set)

import secrets from "k6/secrets";
import { check } from "k6";

export default async function () {
	console.log("Testing URL-based secret source with mock server...\n");

	// Test 1: Get a secret
	console.log("1. Fetching 'my-secret-key'...");
	const mySecret = await secrets.get("my-secret-key");
	console.log(`   Retrieved: ${mySecret}`);

	check(mySecret, {
		"my-secret-key retrieved": (val) => val === "my-secret-key-value",
		"secret not empty": (val) => val.length > 0,
	});

	// Test 2: Get another secret
	console.log("\n2. Fetching 'api-key'...");
	const apiKey = await secrets.get("api-key");
	console.log(`   Retrieved: ${apiKey}`);

	check(apiKey, {
		"api-key retrieved": (val) => val === "super-secret-api-key-12345",
		"api-key not empty": (val) => val.length > 0,
	});

	// Test 3: Get database password
	console.log("\n3. Fetching 'database-pass'...");
	const dbPass = await secrets.get("database-pass");
	console.log(`   Retrieved: ${dbPass}`);

	check(dbPass, {
		"database-pass retrieved": (val) => val === "db-password-xyz789",
	});

	console.log("\n✓ All secrets retrieved successfully!");

	// Example: Use secrets in HTTP requests
	// const response = http.get("https://api.example.com/data", {
	//   headers: {
	//     "Authorization": `Bearer ${apiKey}`,
	//     "X-Database-Auth": dbPass
	//   }
	// });
}
