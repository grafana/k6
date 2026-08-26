## Context

SchemaRegistry is a Confluent component for centralized schema management. Community v1 scripts
use it for Avro/JSON message encoding/decoding. xk6-kafka must support both registry-backed
(load/register schemas via HTTP) and standalone (inline schemas) workflows. index.d.ts is the
authoritative API contract.

Confluent wire protocol prefixes registry-backed messages with a 5-byte magic envelope: byte 0x00
followed by a 4-byte big-endian schema ID. Deserializers must detect and strip this envelope.

## Goals / Non-Goals

**Goals:**
- Implement SchemaRegistry class and HTTP client (register, get schema, subject naming)
- Support Avro serdes with union type handling and Confluent wire format
- Support JSON schema serdes with basic validation and Confluent wire format
- Support standalone serdes: STRING (UTF-8), BYTES (pass-through), and inline Avro/JSON
- Pass integration tests: round-trip encode/decode for all types; registry-backed message interop
- Drop-in replacement for community v1 scripts

**Non-Goals:**
- Schema caching (planned v2 feature; enableCaching flag accepted but ignored in v1)
- Protobuf schemas (messageName, RECORD_NAME_STRATEGY, Schema.references handling)
- Complex JSON Schema references ($ref chains); v1 handles flat schemas only
- Full JSON schema draft validation (keywords like regex patterns, complex refs)

## Decisions

**1. Avro codec: `hamba/avro`**
- Pure Go, no CGO dependency
- Supports Avro unions and primitives
- Handles wire format encoding/decoding as separate concern (5 bytes prepended/stripped)

**2. JSON handling: stdlib `encoding/json` + basic validation**
- Stdlib parses JSON; add basic checks for required fields (from schema)
- Avoid heavyweight JSON schema validator; keep dependencies minimal
- Document v1 limitation: full draft validation, complex refs not supported (add in v2)

**3. Wire format (5 bytes) keyed by schema.id presence**
- Serialize: If schema.id present → prepend `[0x00, hi, mid_hi, mid_lo, lo]` (big-endian 32-bit ID) before data
- Deserialize: If schema.id present → strip and verify 5-byte envelope; if absent → decode all bytes as pure data
- No 0x00 byte sniffing (avoids corruption of standalone Avro/JSON that may legitimately start with 0x00)
- Standalone mode (schema.id absent): no envelope, no magic byte

**4. No caching in v1; defer to v2**
- enableCaching flag in SchemaRegistryConfig is accepted but ignored (v1 calls registry every time)
- Design trade-off: simpler implementation, no stale-cache bugs, HTTP layer may cache anyway
- v2 adds in-memory schema cache once behavior is clear

**5. SchemaRegistry struct + HTTP client in `pkg/kafka/schema_registry.go`**
- Single file for SchemaRegistry type, serialize/deserialize, HTTP ops
- Keep interface compact; wire format logic isolated

**6. Config mapping to index.d.ts**
- `url` (not baseUrl), `enableCaching` (accepted, ignored), `basicAuth{username,password}`, `tls`
- SubjectNameConfig: `element` (KEY/VALUE constants), `subjectNameStrategy` (TOPIC_NAME_STRATEGY)

**7. Container.schema must be typed Schema**
- Match index.d.ts declaration: `schema?: Schema` (object with id, subject, version, schemaType)
- Serialize/deserialize accept parsed Schema from registry, not raw strings
- Standalone mode still supported (schema.id omitted means no wire format)

**8. STRING and BYTES: simple pass-through**
- STRING: UTF-8 encode/decode (no schema, no envelope)
- BYTES: pass-through unchanged (no schema, no envelope)

**9. References unsupported in v1**
- Index.d.ts exposes Schema.references for future Protobuf/complex JSON support
- V1 skips handling; schemas must be self-contained
- Document as known limitation (add support in v2)

## Risks / Trade-offs

**Wire format correctness (5 bytes, no checksum)**
- Risk: Subtle bugs in byte order (big-endian) or ID verification
- Mitigation: Integration tests with real Confluent registry; round-trip validation; bytes verified against community v1

**JSON validation incomplete**
- Risk: Invalid JSON documents accepted (no full draft validation)
- Mitigation: Document v1 limitation; cover required fields and type mismatch; add full validator in v2

**Codec performance (Avro unions, JSON parsing)**
- Risk: Union type dispatch and JSON parsing overhead on every encode/decode
- Mitigation: Acceptable for k6 load tests; profiling deferred to later increment

**Schema ID presence as envelope signal**
- Risk: If schema.id is accidentally omitted for registry-backed data, envelope not stripped
- Mitigation: Schema object always includes id (returned by registry); tests validate envelope round-trips

**No caching in v1**
- Risk: Repeated registry calls on every deserialize (latency)
- Mitigation: Acceptable for v1; HTTP caching headers may help; HTTP layer may cache anyway; add client-side caching in v2

## Test Environment

Local and CI integration tests require Confluent Schema Registry alongside Kafka broker:
- `compose.yaml`: adds `cp-schema-registry` service (port 8081, depends on broker)
- `Makefile` `broker-up` target: exports `SCHEMA_REGISTRY_URL=http://localhost:8081`
- `test/integration/lib/common.js`: adds `getSchemaRegistry()` helper (reads env var, constructs client)
- All integration tests use shared env var; tests skip gracefully if registry unreachable

## Test Coverage Plan

**Unit tests** (pkg/kafka/ tests):
- Config validation (missing url, invalid tls, basicAuth handling)
- Wire format encoding (magic byte 0x00, big-endian schema ID)
- Wire format decoding (ID extraction, verification, based on schema.id presence)
- Avro round-trip (simple record, union, null handling)
- JSON round-trip (simple object, nested, types, required field validation)
- STRING round-trip (UTF-8, empty, special characters)
- BYTES round-trip (pass-through, empty)
- Standalone serdes (all types with schema.id absent)
- Error cases (malformed bytes, type mismatch, schema ID mismatch, schema not found, invalid UTF-8)

**Integration tests** (test/integration/):
- Registry-backed Avro round-trip (register schema, serialize with schema.id, deserialize, verify data)
- Registry-backed JSON round-trip (register schema, serialize with schema.id, deserialize, verify data)
- Standalone Avro/JSON (inline schemas with schema.id absent, no registry calls)
- STRING and BYTES round-trips (no schema, no registry)
- Message interop (produce registry-backed message, consume and deserialize, verify bytes and data)
- Wire format validation (bytes contain correct magic + ID prefix for registry-backed, no prefix for standalone)

## Migration Plan

1. Implement SchemaRegistry struct, HTTP client (getSchema, createSchema, no caching)
2. Add standalone STRING/BYTES serdes
3. Add Avro serdes with 5-byte wire format (schema.id driven)
4. Add JSON serdes with 5-byte wire format and basic validation
5. Unit tests (all paths above)
6. Integration tests (round-trips, registry interop, wire format)
7. Merge and move to next increment

## Open Questions

- Should enableCaching throw an error if not supported? → Accept silently (flag exists for future-proofing)
- How to handle schema.enableCaching on individual Schema objects? → Ignore in v1, defer to v2
- What if data starts with 0x00 in standalone Avro? → That's valid Avro; decoded correctly since no envelope assumed
