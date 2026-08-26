## 1. Setup and Dependencies

- [x] 1.1 Add `github.com/hamba/avro` dependency to go.mod (Avro codec)
- [x] 1.2 Create `pkg/kafka/schema_registry.go` with SchemaRegistry struct
- [x] 1.3 Implement SchemaRegistry constructor (config validation, HTTP client init, standalone mode)

## 2. Registry HTTP Client

- [x] 2.1 Implement `getSchema(subject, version)` HTTP GET to `/subjects/{subject}/versions/{version}`
- [x] 2.2 Implement `createSchema(subject, schema, schemaType)` HTTP POST to `/subjects/{subject}/versions`
- [x] 2.3 Add basic auth header support (BasicAuth from config)
- [x] 2.4 Add error handling and logging for registry calls

## 3. Wire Format Utilities

- [x] 3.1 Implement `encodeWireFormat(schemaID)` → 5-byte magic envelope (0x00 + 4-byte big-endian ID)
- [x] 3.2 Implement `decodeWireFormat(data)` → (schemaID, remainingBytes) tuple, validate magic byte
- [x] 3.3 Add tests for wire format round-trip (encode/decode ID values 0, 1, MAX_INT32)

## 4. STRING and BYTES SerDes

- [x] 4.1 Implement `serialize()` for SCHEMA_TYPE_STRING (UTF-8 encode)
- [x] 4.2 Implement `deserialize()` for SCHEMA_TYPE_STRING (UTF-8 decode, error on invalid UTF-8)
- [x] 4.3 Implement `serialize()` for SCHEMA_TYPE_BYTES (pass-through)
- [x] 4.4 Implement `deserialize()` for SCHEMA_TYPE_BYTES (pass-through)
- [x] 4.5 Unit tests: STRING round-trip (empty, ASCII, special chars), BYTES round-trip (empty, binary)

## 5. Avro SerDes

- [x] 5.1 Implement `serialize()` for SCHEMA_TYPE_AVRO (hamba/avro encode + optional wire format)
- [x] 5.2 Implement `deserialize()` for SCHEMA_TYPE_AVRO (detect wire format by schema.id, strip and verify, hamba/avro decode)
- [x] 5.3 Unit tests: Avro round-trip (simple record, union, null), error cases (type mismatch, corrupted bytes)
- [x] 5.4 Unit tests: wire format Avro (registry-backed round-trip, schema ID verification)

## 6. JSON SerDes

- [x] 6.1 Implement `serialize()` for SCHEMA_TYPE_JSON (validate required fields, JSON encode + optional wire format)
- [x] 6.2 Implement `deserialize()` for SCHEMA_TYPE_JSON (detect wire format by schema.id, strip and verify, JSON parse + basic validation)
- [x] 6.3 Unit tests: JSON round-trip (simple object, nested, types), required field validation
- [x] 6.4 Unit tests: error cases (missing required field, type mismatch, malformed JSON, corrupted bytes)
- [x] 6.5 Unit tests: wire format JSON (registry-backed round-trip, schema ID verification)

## 7. Public API Integration

- [x] 7.1 Implement `getSubjectName(topic, element, strategy, schema)` → subject name (TopicNameStrategy)
- [x] 7.2 Wire SchemaRegistry into module exports (index.js / register.go, export class)
- [x] 7.3 Export all constants: SCHEMA_TYPE_AVRO, SCHEMA_TYPE_JSON, SCHEMA_TYPE_STRING, SCHEMA_TYPE_BYTES, KEY, VALUE, TOPIC_NAME_STRATEGY
- [x] 7.4 Verify index.d.ts types match implementation (SchemaRegistry, Container, Schema, etc.)

## 8. Integration Tests

- [x] 8.1 Extend `compose.yaml`: add Confluent Schema Registry service (image: confluentinc/cp-schema-registry, port 8081, KAFKA_BROKERS=kafka:9092)
- [x] 8.2 Update `Makefile` `broker-up` target: wire `SCHEMA_REGISTRY_URL=http://localhost:8081` env var
- [x] 8.3 Update `test/integration/lib/common.js`: add `getSchemaRegistry()` helper (reads SCHEMA_REGISTRY_URL env var)
- [x] 8.4 Create `test/integration/schema-registry-avro.js` (register schema, round-trip produce/consume with Avro)
- [x] 8.5 Create `test/integration/schema-registry-json.js` (register schema, round-trip produce/consume with JSON)
- [x] 8.6 Create `test/integration/schema-registry-standalone.js` (inline schemas, no registry calls)
- [x] 8.7 Create `test/integration/schema-registry-string-bytes.js` (STRING/BYTES round-trip, no registry)
- [x] 8.8 Run `make integration` and confirm all tests pass

## 9. Documentation and Review

- [x] 9.1 Add migration notes to README.md (how to port community v1 schema-registry scripts)
- [x] 9.2 Document known limitations (no caching v1, no complex refs, no Protobuf)
- [ ] 9.3 Code review checklist: wire format correctness, error handling, test coverage
- [ ] 9.4 Commit and push for PR review
