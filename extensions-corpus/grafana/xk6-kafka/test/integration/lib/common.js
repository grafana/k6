// Shared helpers for the integration tests.
//
// This file lives under lib/ so the `test/integration/*.js` glob does not pick
// it up as a test script (it has no default function).
import { check } from "k6";
import { Counter } from "k6/metrics";

// errors counts assertion failures across all scripts. The threshold below
// turns any failure into a non-zero exit, so `make it` and CI fail.
export const errors = new Counter("test_errors");

// thresholds is re-used by every script via `export const options = { thresholds }`.
// - checks must all pass (rate==1.0)
// - no assertion may fail (test_errors count==0)
export const thresholds = {
  checks: ["rate==1.0"],
  test_errors: ["count==0"],
};

// getBroker returns KAFKA_BROKER or throws. The integration tests require a real
// broker — run `make broker-up` (or `make integration`) or set KAFKA_BROKER.
export function getBroker() {
  const broker = __ENV.KAFKA_BROKER;
  if (!broker) {
    throw new Error(
      "KAFKA_BROKER is not set; run `make broker-up` (or `make integration`), " +
      "or set KAFKA_BROKER to a reachable broker",
    );
  }
  return broker;
}

// getSchemaRegistry returns SCHEMA_REGISTRY_URL or null. Useful for tests that
// optionally use Schema Registry (e.g., registry-backed vs standalone serdes).
// Tests that require the registry should check the result and skip if null.
export function getSchemaRegistry() {
  const url = __ENV.SCHEMA_REGISTRY_URL;
  return url || null;
}

// getSaslBroker returns KAFKA_SASL_BROKER (a SASL_PLAINTEXT listener) or null.
// The SASL auth test skips when it is unset (e.g. an external broker with no
// SASL listener); `make integration` and CI set it to the compose SASL port.
export function getSaslBroker() {
  return __ENV.KAFKA_SASL_BROKER || null;
}

// SASL credentials for the SASL_PLAINTEXT listener, matching the JAAS user in
// compose.yaml. Overridable via env for a different broker.
export const saslUser = __ENV.KAFKA_SASL_USER || "testuser";
export const saslPass = __ENV.KAFKA_SASL_PASS || "testpass";

// verify runs a named check and records a failure against the error budget.
// Returns the boolean result so callers can branch when needed.
export function verify(name, condition) {
  const ok = check(null, { [name]: () => condition });
  if (!ok) {
    errors.add(1);
  }
  return ok;
}

// runTest runs a test body and converts any thrown error into a recorded
// failure. Without this, an uncaught exception (e.g. a failed produce) does not
// trip the thresholds, so the run would pass silently — a false green.
export function runTest(fn) {
  try {
    fn();
  } catch (e) {
    verify(`no unexpected error (${e})`, false);
  }
}

// toStr decodes a consumed key/value (bytes) to a string, tolerating array,
// Uint8Array, or ArrayBuffer representations.
export function toStr(v) {
  if (v instanceof ArrayBuffer) {
    v = new Uint8Array(v);
  }
  return String.fromCharCode.apply(null, v);
}

// uniqueTopic builds a per-run topic name with the given prefix.
export function uniqueTopic(prefix) {
  return `${prefix}_${Date.now()}`;
}
