# avro-serdes Specification

## Purpose

Avro schema support for SchemaRegistry. Encodes and decodes Avro-formatted messages with
optional Confluent wire format (magic + schema ID prefix). Works standalone (inline Avro
schemas) or registry-backed.

## ADDED Requirements

### Requirement: Serialize Avro data

`schemaRegistry.serialize(container)` SHALL encode data to Avro bytes using the provided
schema. The `container` parameter MUST include `data` (the value to encode), `schemaType`
(MUST be `SCHEMA_TYPE_AVRO`), and `schema` (a Schema object, per index.d.ts). Returns a
`Uint8Array` of encoded bytes.

When `schema.id` is present (registry-backed), the bytes SHALL be prefixed with a Confluent
wire format magic envelope (1 byte 0x00 followed by 4 bytes of big-endian schema ID), then
followed by Avro-encoded data. When `schema.id` is absent (standalone), bytes are pure Avro
with no magic envelope.

SHALL throw if data does not match the schema structure.

#### Scenario: Serialize a simple record (standalone)

- **WHEN** serialize is called with data matching a simple Avro record schema (schema.id absent)
- **THEN** a Uint8Array of pure Avro-encoded bytes is returned (no magic envelope)

#### Scenario: Serialize with schema ID (registry-backed)

- **WHEN** serialize is called with data and a schema object containing an `id` field
- **THEN** a Uint8Array with Confluent magic envelope (5 bytes: 0x00 + 4-byte big-endian ID) plus Avro data is returned

#### Scenario: Serialize an Avro union

- **WHEN** serialize is called with union data (e.g., null, string, or number for a union type)
- **THEN** the correct union type is encoded and bytes are returned (with or without envelope)

#### Scenario: Serialize fails on type mismatch

- **WHEN** serialize is called with data that does not match the schema
- **THEN** an error is thrown

### Requirement: Deserialize Avro data

`schemaRegistry.deserialize(container)` SHALL decode Avro bytes back to a JavaScript object
using the provided schema. The `container` parameter MUST include `data` (Uint8Array of bytes),
`schemaType` (SCHEMA_TYPE_AVRO), and `schema` (a Schema object, per index.d.ts). Returns the
decoded object.

Deserialization behavior depends on `schema.id`:
- When `schema.id` is present (registry-backed): deserialize SHALL strip the first 5 bytes
  (magic byte 0x00 + 4-byte big-endian schema ID), verify the ID matches `schema.id`, and
  decode the remaining bytes as Avro.
- When `schema.id` is absent (standalone): deserialize SHALL decode the entire `data` as pure Avro.

SHALL throw if bytes are malformed Avro or schema ID verification fails.

#### Scenario: Deserialize pure Avro (standalone)

- **WHEN** deserialize is called with schema.id absent and valid Avro bytes
- **THEN** a JavaScript object matching the schema is returned, and all input bytes are decoded

#### Scenario: Deserialize with Confluent magic envelope (registry-backed)

- **WHEN** deserialize is called with schema.id present and bytes starting with magic byte (0x00) followed by matching schema ID
- **THEN** the 5-byte magic envelope is stripped, schema ID is verified, and remaining bytes are decoded as Avro

#### Scenario: Deserialize an Avro union

- **WHEN** deserialize is called with union-encoded bytes
- **THEN** the union value is decoded and the correct type is returned

#### Scenario: Deserialize fails on malformed bytes

- **WHEN** deserialize is called with invalid or corrupted Avro bytes
- **THEN** an error is thrown

#### Scenario: Deserialize fails on schema ID mismatch

- **WHEN** deserialize is called with schema.id present and the first 4 bytes (after magic) do not match schema.id
- **THEN** an error is thrown

### Requirement: Avro round-trip

Encoding then decoding the same data with the same schema (standalone or registry-backed) SHALL
return an equivalent value (same keys, types, and values).

#### Scenario: Record round-trip (standalone)

- **WHEN** a record is serialized (no magic envelope) and deserialized with the same schema (schema.id absent)
- **THEN** the result matches the original data

#### Scenario: Record round-trip (registry-backed)

- **WHEN** a record is serialized with a schema containing an id (magic envelope) and deserialized with the same schema
- **THEN** the result matches the original data and magic envelope is correctly handled

#### Scenario: Union round-trip

- **WHEN** a union value is serialized and deserialized with the same schema
- **THEN** the result is equivalent (including null and nested unions)
