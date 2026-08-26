# kafka-module Specification

## Purpose
TBD - created by archiving change add-extension-scaffold. Update Purpose after archive.
## Requirements
### Requirement: Module registration

The extension SHALL register a k6 module under the import path `k6/x/kafka` so
that k6 scripts can import its members. The module's members SHALL be exposed as
the module's default export.

#### Scenario: Importing the module

- **WHEN** a k6 script runs `import { Writer } from "k6/x/kafka"` on a `k6`
  binary built with this extension
- **THEN** the import resolves without error and `Writer` is defined

#### Scenario: Members available via the default export

- **WHEN** a k6 script runs `import kafka from "k6/x/kafka"`
- **THEN** the default export is an object exposing the module members (e.g.
  `kafka.Writer`, `kafka.Reader`, and `kafka.CODEC_SNAPPY` are defined)

### Requirement: Exported constants match the contract

The module SHALL export every grouped value declared in `index.d.ts` as an
individual top-level constant, with names and values matching the contract
exactly. This includes the compression codecs, SASL mechanisms, TLS versions,
balancers, group balancers, schema types, element types, subject name
strategies, isolation levels, start offsets (including the `FIRST_OFFSET` and
`LAST_OFFSET` backward-compatibility aliases), and `TIME` units. The constants
SHALL be exported as flat values, not grouped inside enum objects.

#### Scenario: A string constant has the contract value

- **WHEN** a script reads `CODEC_SNAPPY` imported from `k6/x/kafka`
- **THEN** its value is the string `"snappy"`

#### Scenario: An element-type constant uses the lower-case value

- **WHEN** a script reads `KEY` and `VALUE` imported from `k6/x/kafka`
- **THEN** their values are the strings `"key"` and `"value"`

#### Scenario: A time unit is a nanosecond count

- **WHEN** a script reads `SECOND` imported from `k6/x/kafka`
- **THEN** its value is the number `1000000000`

#### Scenario: No enum namespace objects are exported

- **WHEN** a script reads `COMPRESSION_CODECS` from `k6/x/kafka`
- **THEN** it is `undefined` (the grouped values exist only as flat constants)

### Requirement: Public symbols are present

The module SHALL export the public top-level symbols declared in `index.d.ts`:
the constructors `Writer`, `Reader`, `Connection`, and `SchemaRegistry`, and the
function `LoadJKS`. Each constructor SHALL be invocable with `new` and construct
an instance without error when given a valid configuration.

This requirement covers **only symbol presence and construction**. The instance
methods declared in `index.d.ts` (e.g. `Writer.produce`, `Reader.consume`,
`Connection.createTopic`, the `SchemaRegistry` serdes methods) are explicitly
**out of scope for this change** and are delivered by later capability changes
(producer, consumer, admin, schema-registry). Likewise, the behavior of
`LoadJKS` (actually reading a keystore) is deferred to the auth change; here only
its presence as a function is required. Completing this requirement does not
imply the module's method surface is complete.

#### Scenario: Constructing a Writer

- **WHEN** a script runs `new Writer({ brokers: ["localhost:9092"], topic: "t" })`
- **THEN** an instance is returned without throwing

#### Scenario: Constructing a Reader

- **WHEN** a script runs `new Reader({ brokers: ["localhost:9092"], topic: "t" })`
- **THEN** an instance is returned without throwing

#### Scenario: Constructing a Connection

- **WHEN** a script runs `new Connection({ address: "localhost:9092" })`
- **THEN** an instance is returned without throwing

#### Scenario: SchemaRegistry constructs with no arguments

- **WHEN** a script runs `new SchemaRegistry()`
- **THEN** an instance is returned without throwing

#### Scenario: LoadJKS is present as a function

- **WHEN** a script reads `LoadJKS` imported from `k6/x/kafka`
- **THEN** it is a function (its keystore-loading behavior is verified by the auth change)

