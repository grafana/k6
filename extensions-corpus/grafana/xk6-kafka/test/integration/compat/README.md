# Compatibility fixtures

Modernized ports of the community [`mostafa/xk6-kafka`](https://github.com/mostafa/xk6-kafka)
v1 example scripts, kept close to the originals so their `check()` assertions
verify behavior parity with this extension. They run as part of `make integration`.

Uniform adaptations across all fixtures:

- **Broker / registry from env** (`getBroker()` / `getSchemaRegistry()` from
  `../lib/common.js`) instead of hardcoded `localhost`.
- **Produce loops trimmed** (the originals used 100 iterations) — still ≥ the
  consume count each script asserts.
- **Wrapped in `runTest()`** and using the shared `thresholds` so an exception
  fails the run — a bare `checks: rate==1.0` threshold passes vacuously on zero
  checks, which would otherwise let a crashing script report green.

## Fixtures

| Fixture | Based on | Kind |
|---|---|---|
| `string.js` | `test_string.js` | Faithful parity — runs unchanged (env wiring only) |
| `bytes.js` | `test_bytes.js` | Faithful parity |
| `topics.js` | `test_topics.js` | Faithful parity (admin) |
| `avro.js` | `test_avro_with_schema_registry.js` | Faithful parity (registry; uses `TOPIC_NAME_STRATEGY` + `RECORD_NAME_STRATEGY`) |
| `json.js` | `test_json.js` | **MIGRATED** — see below |

## Known migration: JSON serdes require a schema

The community `test_json.js` does **schemaless** JSON serdes (`serialize`/
`deserialize` with only `data` + `schemaType`). This extension's JSON serdes
**require a schema object** (per `openspec/specs/json-serdes`: standalone = a
schema whose `id` is absent, used for required/type validation and pure-JSON
encoding), matching `index.d.ts` `Container.schema`. STRING and BYTES serdes
require no schema, but JSON does.

So `json.js` supplies a standalone JSON schema — it demonstrates **how a v1 JSON
script migrates** to this extension, not that the v1 JSON script runs unchanged.
This is a real familiarity divergence from the community extension; a migration
guide should call it out. (Whether to support schemaless JSON serdes for closer
parity is a possible future change; STRING/BYTES already are schemaless.)

## Not ported here

- `test_sasl_auth.js` → covered by `../sasl.js`.
- `test_consumer_group.js` → covered by `../consumer-group-commit.js`.
- `test_protobuf_*` → Protobuf is a v2-only community feature, out of scope.
- `test_tls_with_jks.js` → needs a TLS broker (deferred with the #67 TLS follow-up).
- `test_azure_event_hub.js` / `test_gcp_kafka.js` → managed-cloud specific.
- Complex/union/multi-topic Avro, JSON-Schema-via-registry, custom balancer,
  timeout → optional edge cases, may be added on demand.
