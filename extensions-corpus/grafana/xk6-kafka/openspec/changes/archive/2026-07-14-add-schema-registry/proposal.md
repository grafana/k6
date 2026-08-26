## Why

Community v1 scripts rely on SchemaRegistry for message serialization/deserialization (Avro and JSON).
Adding schema-registry support unblocks those compat scripts and covers an epic acceptance criterion
(Schema Registry serdes — Avro and JSON). Critical: messages must encode/decode the Confluent wire
format (5-byte magic + 4-byte schema ID) to interoperate with other Confluent clients.

## What Changes

- New `SchemaRegistry` class: construct with registry endpoint, load/register schemas, encode/decode messages
- Avro serdes: serialize/deserialize Avro-encoded messages with Confluent wire format (registry-backed) or pure Avro (standalone), union support
- JSON serdes: serialize/deserialize JSON-validated messages with Confluent wire format (registry-backed) or pure JSON (standalone)
- STRING serdes: encode/decode UTF-8 text (no schema)
- BYTES serdes: pass-through bytes unchanged (no schema)
- Registry HTTP client: GET/POST schema endpoints (subject, version, schema registration)
- Wire format: 5-byte Confluent prefix (magic 0x00 + 4-byte big-endian schema ID) on registry-backed messages; schema.id presence determines encoding/decoding behavior

## Capabilities

### New Capabilities
- `schema-registry`: SchemaRegistry client, schema load/register operations, subject naming strategies (TopicNameStrategy)
- `avro-serdes`: Avro schema support, serialize/deserialize with Confluent wire format, union handling
- `json-serdes`: JSON schema support, serialize/deserialize with Confluent wire format, basic validation
- `string-bytes-serdes`: STRING and BYTES simple serdes (UTF-8, pass-through; no schema required)

### Modified Capabilities
<!-- None: schema-registry is purely additive -->

## Impact

- **Code**: New `pkg/kafka/schema_registry.go` (client, HTTP ops, all serdes, wire format)
- **index.d.ts**: Already declares SchemaRegistry, config types (url, enableCaching, basicAuth, tls), Schema type, schema types (SCHEMA_TYPE_AVRO, SCHEMA_TYPE_JSON, SCHEMA_TYPE_STRING, SCHEMA_TYPE_BYTES), element types (KEY, VALUE), subject naming strategy (TOPIC_NAME_STRATEGY)
- **Tests**: Unit tests (config, wire format, all serde types, round-trips, error cases), integration tests (registry operations, serdes, message interop)
- **Dependencies**: Avro codec (`hamba/avro`), JSON handling (stdlib + basic validation)
- **Compatibility**: Drop-in for community v1 scripts using schema-registry; Confluent wire protocol compatible
- **Known limitations**: Protobuf schemas, complex JSON Schema references ($ref chains), full JSON schema draft validation, schema caching (all deferred to v2)
