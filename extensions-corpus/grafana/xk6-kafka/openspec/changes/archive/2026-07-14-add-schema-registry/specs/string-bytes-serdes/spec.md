# string-bytes-serdes Specification

## Purpose

Simple serdes for STRING and BYTES data types. No schema required; operates in standalone mode
only. STRING encodes/decodes UTF-8 text; BYTES passes through raw bytes.

## ADDED Requirements

### Requirement: Serialize STRING data

`schemaRegistry.serialize(container)` SHALL encode text to UTF-8 bytes. The `container` parameter
MUST include `data` (string), `schemaType` (MUST be `SCHEMA_TYPE_STRING`), and MUST omit `schema`
(no schema required). Returns a `Uint8Array` of UTF-8 bytes.

#### Scenario: Serialize a string

- **WHEN** serialize is called with schemaType=SCHEMA_TYPE_STRING, data="hello", and no schema
- **THEN** a Uint8Array of UTF-8 bytes representing "hello" is returned

#### Scenario: Serialize empty string

- **WHEN** serialize is called with schemaType=SCHEMA_TYPE_STRING and data=""
- **THEN** an empty Uint8Array is returned

#### Scenario: Serialize with special characters

- **WHEN** serialize is called with schemaType=SCHEMA_TYPE_STRING and data containing non-ASCII characters
- **THEN** the bytes are correctly UTF-8 encoded

### Requirement: Deserialize STRING data

`schemaRegistry.deserialize(container)` SHALL decode UTF-8 bytes to a string. The `container`
parameter MUST include `data` (Uint8Array of UTF-8 bytes), `schemaType` (SCHEMA_TYPE_STRING),
and MUST omit `schema`. Returns the decoded string.

#### Scenario: Deserialize a string

- **WHEN** deserialize is called with schemaType=SCHEMA_TYPE_STRING and valid UTF-8 bytes
- **THEN** the decoded string is returned

#### Scenario: Deserialize empty bytes

- **WHEN** deserialize is called with schemaType=SCHEMA_TYPE_STRING and an empty Uint8Array
- **THEN** an empty string is returned

#### Scenario: Deserialize fails on invalid UTF-8

- **WHEN** deserialize is called with schemaType=SCHEMA_TYPE_STRING and invalid UTF-8 bytes
- **THEN** an error is thrown

### Requirement: STRING round-trip

Encoding a string and then decoding the bytes SHALL return the original string.

#### Scenario: String round-trip

- **WHEN** a string is serialized and deserialized
- **THEN** the result matches the original string

### Requirement: Serialize BYTES data

`schemaRegistry.serialize(container)` SHALL pass through bytes unchanged. The `container`
parameter MUST include `data` (Uint8Array), `schemaType` (MUST be `SCHEMA_TYPE_BYTES`), and
MUST omit `schema`. Returns the `data` unchanged.

#### Scenario: Serialize bytes

- **WHEN** serialize is called with schemaType=SCHEMA_TYPE_BYTES and data as Uint8Array
- **THEN** the same Uint8Array is returned

#### Scenario: Serialize empty bytes

- **WHEN** serialize is called with schemaType=SCHEMA_TYPE_BYTES and an empty Uint8Array
- **THEN** an empty Uint8Array is returned

### Requirement: Deserialize BYTES data

`schemaRegistry.deserialize(container)` SHALL return bytes unchanged. The `container` parameter
MUST include `data` (Uint8Array), `schemaType` (SCHEMA_TYPE_BYTES), and MUST omit `schema`.
Returns the `data` unchanged.

#### Scenario: Deserialize bytes

- **WHEN** deserialize is called with schemaType=SCHEMA_TYPE_BYTES and data as Uint8Array
- **THEN** the same Uint8Array is returned

#### Scenario: Deserialize empty bytes

- **WHEN** deserialize is called with schemaType=SCHEMA_TYPE_BYTES and an empty Uint8Array
- **THEN** an empty Uint8Array is returned

### Requirement: BYTES round-trip

Encoding and decoding bytes SHALL return the original bytes unchanged.

#### Scenario: Bytes round-trip

- **WHEN** bytes are serialized and deserialized
- **THEN** the result matches the original bytes exactly
