# schema-registry Specification

## Purpose

SchemaRegistry client for managing Kafka message schemas. Supports schema registration, lookup,
and subject naming strategies. Integrates with Confluent Schema Registry HTTP API. Opt-in
client-side caching (via `enableCaching`) avoids repeated registry calls for the same schema;
parsed schema definitions are always reused.
## Requirements
### Requirement: SchemaRegistry construction

`new SchemaRegistry(schemaRegistryConfig)` SHALL construct a SchemaRegistry client. When
`schemaRegistryConfig` is omitted, the client operates in standalone mode (string, bytes,
or inline Avro/JSON schemas; no HTTP calls). When config is provided, it MUST include `url`
(registry endpoint) and MAY include `basicAuth` (object with `username` and `password` fields)
and `tls` (TLS config). The `enableCaching` field, when `true`, enables caching of schemas
resolved from the registry so repeated `getSchema` of the same subject and version skip the
network (see "Registry response caching"); it defaults to `false`, in which case the registry is
contacted on every `getSchema`. (Parsed-schema reuse is always on and is not governed by this
flag — see "Parsed schema reuse".) Construction SHALL throw if `url` is missing, invalid, or
unreachable.

#### Scenario: Construct in standalone mode

- **WHEN** SchemaRegistry is constructed with no config
- **THEN** the client is ready for standalone string/bytes/inline-schema serdes (no registry calls)

#### Scenario: Construct with registry URL

- **WHEN** SchemaRegistry is constructed with `url` pointing to a running registry
- **THEN** the client is ready and can reach the registry

#### Scenario: Construct with basic auth

- **WHEN** SchemaRegistry is constructed with `url` and `basicAuth` containing `username` and `password`
- **THEN** the client uses those credentials for registry HTTP requests

#### Scenario: enableCaching toggles registry caching

- **WHEN** SchemaRegistry is constructed with `enableCaching: true`
- **THEN** schemas resolved from the registry are cached (repeated `getSchema` of the same
  subject and version skips the network)
- **WHEN** SchemaRegistry is constructed with `enableCaching: false` or omitted
- **THEN** the registry is contacted on every `getSchema` (parsed-schema reuse still applies)

#### Scenario: Construction fails on missing url

- **WHEN** SchemaRegistry is constructed with a config missing `url`
- **THEN** construction throws an error

#### Scenario: Construction fails on unreachable registry

- **WHEN** SchemaRegistry is constructed with an unreachable `url`
- **THEN** construction throws an error

### Requirement: Load schema from registry

`schemaRegistry.getSchema(schema)` SHALL retrieve a schema from the registry. The `schema`
parameter MUST include `subject` (required) and MAY include `version` (defaults to latest).
Returns the full schema object (Schema type, per index.d.ts) including `id`, `version`,
`subject`, `schema`, and `schemaType`. When caching is disabled (the default), the registry is
called on every invocation. When caching is enabled, the first resolution of a given subject and
version contacts the registry and subsequent resolutions of the same subject and version are
served from the cache (see the "Registry response caching" requirement).

#### Scenario: Load latest schema by subject

- **WHEN** getSchema is called with a subject that exists in the registry
- **THEN** the latest version of that schema is returned

#### Scenario: Load specific schema version

- **WHEN** getSchema is called with subject and version
- **THEN** that specific version is returned if it exists

#### Scenario: Load fails for missing subject

- **WHEN** getSchema is called with a subject not in the registry
- **THEN** an error is thrown

### Requirement: Register schema in registry

`schemaRegistry.createSchema(schema)` SHALL register a new schema or return the existing one.
The `schema` parameter MUST include `subject`, `schema` (the raw schema string), and
`schemaType` (e.g., `SCHEMA_TYPE_AVRO`, `SCHEMA_TYPE_JSON`). Returns the registered schema
(Schema type) with assigned `id`.

#### Scenario: Register new schema

- **WHEN** createSchema is called with a new subject and schema
- **THEN** the schema is registered and an id is assigned

#### Scenario: Idempotent registration

- **WHEN** createSchema is called with a subject and schema that already exist
- **THEN** the existing schema is returned (not duplicated)

#### Scenario: Registration fails on invalid schema type

- **WHEN** createSchema is called with an invalid or unsupported `schemaType`
- **THEN** an error is thrown

### Requirement: Subject naming strategy

`schemaRegistry.getSubjectName(subjectNameConfig)` SHALL return the subject name for a topic
and key-or-value designation. Per the `index.d.ts` contract the parameter MUST include `topic`
(topic name), `element` (KEY or VALUE constant), `subjectNameStrategy` (naming strategy
constant), and `schema` (schema definition string) — all four are always supplied. `element` is
used only by TopicNameStrategy; `schema` is used only by the record strategies (the others
ignore it).

The supported strategies are:

- **TopicNameStrategy** (also the default when `subjectNameStrategy` is empty): returns
  `{topic}-key` or `{topic}-value` per `element`; ignores `schema`.
- **RecordNameStrategy**: returns the schema's fully-qualified record name (e.g.
  `com.example.User`).
- **TopicRecordNameStrategy**: returns `{topic}-{record-fullname}`.

The record name is derived from an **Avro** named schema — record, enum, or fixed — via its
fully-qualified name. If `schema` is empty, or is not a parseable Avro named schema, the record
strategies SHALL return an error rather than guess. JSON Schema and Protobuf record naming are
not supported in v1. An unrecognized (non-empty) `subjectNameStrategy` SHALL return an error.

#### Scenario: TopicNameStrategy for value

- **WHEN** getSubjectName is called with topic="my-topic", element=VALUE, subjectNameStrategy=TOPIC_NAME_STRATEGY
- **THEN** "my-topic-value" is returned

#### Scenario: TopicNameStrategy for key

- **WHEN** getSubjectName is called with topic="my-topic", element=KEY, subjectNameStrategy=TOPIC_NAME_STRATEGY
- **THEN** "my-topic-key" is returned

#### Scenario: Empty strategy defaults to TopicNameStrategy

- **WHEN** getSubjectName is called with topic="my-topic", element=VALUE, and an empty subjectNameStrategy
- **THEN** "my-topic-value" is returned

#### Scenario: TopicNameStrategy ignores the schema

- **WHEN** getSubjectName is called with TopicNameStrategy, topic="my-topic", element=VALUE, and
  an empty (or arbitrary) `schema`
- **THEN** "my-topic-value" is returned regardless of the `schema` value

#### Scenario: RecordNameStrategy returns the record full name

- **WHEN** getSubjectName is called with subjectNameStrategy=RECORD_NAME_STRATEGY and an Avro
  record schema named `User` in namespace `com.example`
- **THEN** "com.example.User" is returned (`topic` and `element` are still supplied per the
  contract, but do not affect a RecordNameStrategy subject)

#### Scenario: RecordNameStrategy with a record that has no namespace

- **WHEN** getSubjectName is called with subjectNameStrategy=RECORD_NAME_STRATEGY and an Avro
  record named `User` with no namespace
- **THEN** "User" is returned (the bare name, with no leading dot)

#### Scenario: TopicRecordNameStrategy combines topic and record full name

- **WHEN** getSubjectName is called with topic="my-topic",
  subjectNameStrategy=TOPIC_RECORD_NAME_STRATEGY, and an Avro record schema `com.example.User`
- **THEN** "my-topic-com.example.User" is returned

#### Scenario: RecordNameStrategy works for other named Avro schemas

- **WHEN** getSubjectName is called with subjectNameStrategy=RECORD_NAME_STRATEGY and an Avro
  enum (or fixed) named `com.example.Color`
- **THEN** "com.example.Color" is returned (any named Avro schema, not only records)

#### Scenario: Record strategy with an empty schema errors

- **WHEN** getSubjectName is called with a record strategy and an empty `schema`
- **THEN** an error is returned

#### Scenario: Record strategy with a non-Avro or unnamed schema errors

- **WHEN** getSubjectName is called with a record strategy and a `schema` that is not a
  parseable Avro named schema (e.g. a JSON Schema, or an Avro primitive)
- **THEN** an error is returned

#### Scenario: Unknown strategy errors

- **WHEN** getSubjectName is called with a non-empty `subjectNameStrategy` that is not one of
  the three supported constants
- **THEN** an error is returned

### Requirement: Registry response caching

When the client-level `SchemaRegistryConfig.enableCaching` is `true`, `SchemaRegistry` SHALL
cache resolved schemas to avoid repeated registry round-trips within a test. The cache is
per-`SchemaRegistry` instance (per VU) and lives for the client's lifetime; it is NOT
invalidated during a run, because schemas are assumed stable for the duration of a test. When
`enableCaching` is `false` (the default) the registry is contacted on every `getSchema`.

- `getSchema` results SHALL be cached keyed by subject and requested version (including
  `latest`); a cache hit MUST NOT contact the registry.
- A cache hit SHALL return an independent `Schema` (a copy), so a caller mutating a returned
  schema cannot corrupt a subsequently returned cached value.
- `createSchema` does NOT populate the cache: the schema it returns may not carry a version
  (Confluent's registration response often omits it), so there is no reliable cache key. A
  subsequent `getSchema` performs the (cacheable) resolution.

Registry caching is governed solely by the client-level `SchemaRegistryConfig.enableCaching`
(default `false`). The per-schema `Schema.enableCaching` field (declared in index.d.ts) is
accepted but ignored in v1; it does not enable or disable caching for an individual schema.

Because a `latest` lookup is cached, a schema evolved after the first `latest` resolution SHALL
NOT be observed by that client for the rest of the run. Note this can surface as a hard error,
not just stale data: `deserialize` enforces a wire-format schema-id match, so a cached `latest`
consuming messages written with an evolved (higher-id) schema throws a schema-id mismatch.

#### Scenario: Repeated getSchema is served from cache

- **WHEN** caching is enabled and `getSchema` is called twice for the same subject and version
- **THEN** the first call contacts the registry and the second is served from the cache without
  a registry request, returning an equal (but independent) schema

#### Scenario: Caching disabled calls the registry every time

- **WHEN** caching is disabled and `getSchema` is called twice for the same subject
- **THEN** both calls contact the registry

#### Scenario: Cached hit is independent of caller mutation

- **WHEN** caching is enabled, `getSchema` returns a schema, the caller mutates a field on it,
  and `getSchema` is called again for the same subject and version
- **THEN** the second result is unaffected by the mutation

#### Scenario: Cached latest does not refresh mid-run

- **WHEN** caching is enabled, `getSchema` resolves `latest`, a newer version is then registered,
  and `getSchema latest` is called again on the same client
- **THEN** the originally cached schema is returned (the cache is not invalidated during a run),
  and the intervening `createSchema` did not seed or refresh the cache

#### Scenario: Per-schema enableCaching is ignored

- **WHEN** a `Schema` is passed with `enableCaching: true` (or `false`) while the client-level
  `enableCaching` says otherwise
- **THEN** the per-schema field has no effect; only the client-level setting governs caching

### Requirement: Parsed schema reuse

`SchemaRegistry` SHALL reuse parsed Avro schema definitions, keyed by the schema string, so
repeated `serialize` / `deserialize` of the same schema do not re-parse it. This is a
behavior-neutral optimization (parsing is deterministic), so it is **always on**, independent of
`enableCaching`, and applies in standalone (no-registry) mode as well. (JSON serdes have no
compile step, so only Avro parsing is reused. A schema string that fails to parse is not cached,
so the parse error surfaces on every call.)

#### Scenario: Parsed schema is reused across serdes

- **WHEN** `serialize` or `deserialize` is called repeatedly with the same Avro schema string
- **THEN** the schema string is parsed once and the parsed form is reused for subsequent calls

#### Scenario: Parsed reuse works in standalone mode

- **WHEN** a standalone `SchemaRegistry` (constructed with no config) serializes repeatedly with
  the same inline Avro schema string
- **THEN** the schema is parsed once and reused, even though no `enableCaching` flag exists in
  standalone mode

