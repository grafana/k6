# json-serdes Specification

## Purpose

JSON schema support for SchemaRegistry. Encodes and decodes JSON-formatted messages with
optional Confluent wire format (magic + schema ID prefix). Works standalone (inline JSON
schemas) or registry-backed.

## ADDED Requirements

### Requirement: Serialize JSON data

`schemaRegistry.serialize(container)` SHALL encode data to JSON bytes using the provided
schema. The `container` parameter MUST include `data` (the value to encode), `schemaType`
(MUST be `SCHEMA_TYPE_JSON`), and `schema` (a Schema object, per index.d.ts). Returns a
`Uint8Array` of UTF-8-encoded JSON bytes.

When `schema.id` is present (registry-backed), the bytes SHALL be prefixed with a Confluent
wire format magic envelope (1 byte 0x00 followed by 4 bytes of big-endian schema ID), then
followed by UTF-8 JSON. When `schema.id` is absent (standalone), bytes are pure JSON with no
magic envelope.

Basic validation (required fields, type mismatch) SHALL be applied; full JSON schema draft
validation is not required for v1.

#### Scenario: Serialize a JSON object (standalone)

- **WHEN** serialize is called with data matching a JSON schema (schema.id absent)
- **THEN** a Uint8Array of UTF-8 JSON bytes is returned (no magic envelope)

#### Scenario: Serialize with schema ID (registry-backed)

- **WHEN** serialize is called with data and a schema object containing an `id` field
- **THEN** a Uint8Array with Confluent magic envelope (5 bytes: 0x00 + 4-byte big-endian ID) plus UTF-8 JSON is returned

#### Scenario: Serialize with nested objects

- **WHEN** serialize is called with nested object data
- **THEN** the nested structure is encoded and bytes are returned

#### Scenario: Serialize fails on required field missing

- **WHEN** serialize is called with data missing required fields defined in the schema
- **THEN** an error is thrown

#### Scenario: Serialize fails on type mismatch

- **WHEN** serialize is called with data whose types do not match the schema
- **THEN** an error is thrown

### Requirement: Deserialize JSON data

`schemaRegistry.deserialize(container)` SHALL decode JSON bytes to a JavaScript object
using the provided schema. The `container` parameter MUST include `data` (Uint8Array of
UTF-8 bytes), `schemaType` (SCHEMA_TYPE_JSON), and `schema` (a Schema object, per index.d.ts).
Returns the decoded object.

Deserialization behavior depends on `schema.id`:
- When `schema.id` is present (registry-backed): deserialize SHALL strip the first 5 bytes
  (magic byte 0x00 + 4-byte big-endian schema ID), verify the ID matches `schema.id`, and
  parse the remaining bytes as UTF-8 JSON.
- When `schema.id` is absent (standalone): deserialize SHALL parse the entire `data` as pure UTF-8 JSON.

Basic validation (required fields, type checking) SHALL be applied. Shall throw if bytes are
malformed UTF-8 JSON or schema ID verification fails.

#### Scenario: Deserialize pure JSON (standalone)

- **WHEN** deserialize is called with schema.id absent and valid UTF-8 JSON bytes
- **THEN** a JavaScript object is returned, and all input bytes are parsed

#### Scenario: Deserialize with Confluent magic envelope (registry-backed)

- **WHEN** deserialize is called with schema.id present and bytes starting with magic byte (0x00) followed by matching schema ID
- **THEN** the 5-byte magic envelope is stripped, schema ID is verified, and remaining bytes are parsed as JSON

#### Scenario: Deserialize with nested objects

- **WHEN** deserialize is called with nested JSON bytes
- **THEN** the nested structure is decoded correctly

#### Scenario: Deserialize fails on malformed JSON

- **WHEN** deserialize is called with invalid JSON bytes
- **THEN** an error is thrown

#### Scenario: Deserialize fails on schema ID mismatch

- **WHEN** deserialize is called with schema.id present and the first 4 bytes (after magic) do not match schema.id
- **THEN** an error is thrown

### Requirement: JSON round-trip

Encoding then decoding the same data with the same schema (standalone or registry-backed) SHALL
return an equivalent value (same keys, values, and structure).

#### Scenario: Object round-trip (standalone)

- **WHEN** an object is serialized (no magic envelope) and deserialized with the same schema (schema.id absent)
- **THEN** the result matches the original data

#### Scenario: Object round-trip (registry-backed)

- **WHEN** an object is serialized with a schema containing an id (magic envelope) and deserialized with the same schema
- **THEN** the result matches the original data and magic envelope is correctly handled

#### Scenario: Nested object round-trip

- **WHEN** a nested object is serialized and deserialized
- **THEN** the structure and values are preserved
