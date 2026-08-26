# schema-registry Specification

## Purpose

SchemaRegistry client for managing Kafka message schemas. Supports schema registration, lookup,
and subject naming strategies. Integrates with Confluent Schema Registry HTTP API. Caching is
a planned v2 feature; v1 makes registry calls on every operation.

## ADDED Requirements

### Requirement: SchemaRegistry construction

`new SchemaRegistry(schemaRegistryConfig)` SHALL construct a SchemaRegistry client. When
`schemaRegistryConfig` is omitted, the client operates in standalone mode (string, bytes,
or inline Avro/JSON schemas; no HTTP calls). When config is provided, it MUST include `url`
(registry endpoint) and MAY include `basicAuth` (object with `username` and `password` fields)
and `tls` (TLS config). The `enableCaching` field is accepted but ignored in v1 (caching is
planned for v2). Construction SHALL throw if `url` is missing, invalid, or unreachable.

#### Scenario: Construct in standalone mode

- **WHEN** SchemaRegistry is constructed with no config
- **THEN** the client is ready for standalone string/bytes/inline-schema serdes (no registry calls)

#### Scenario: Construct with registry URL

- **WHEN** SchemaRegistry is constructed with `url` pointing to a running registry
- **THEN** the client is ready and can reach the registry

#### Scenario: Construct with basic auth

- **WHEN** SchemaRegistry is constructed with `url` and `basicAuth` containing `username` and `password`
- **THEN** the client uses those credentials for registry HTTP requests

#### Scenario: Construct with enableCaching (v1 ignores it)

- **WHEN** SchemaRegistry is constructed with `enableCaching: true` or `false`
- **THEN** the flag is accepted but has no effect in v1 (caching is deferred to v2)

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
`subject`, `schema`, and `schemaType`. The registry is called on every invocation (no caching in v1).

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
and key-or-value designation. The parameter MUST include `topic` (topic name), `element`
(KEY or VALUE constant), `subjectNameStrategy` (naming strategy constant), and `schema`
(schema definition string). Implements TopicNameStrategy: `{topic}-key` / `{topic}-value`.

#### Scenario: TopicNameStrategy for value

- **WHEN** getSubjectName is called with topic="my-topic", element=VALUE, subjectNameStrategy=TOPIC_NAME_STRATEGY
- **THEN** "my-topic-value" is returned

#### Scenario: TopicNameStrategy for key

- **WHEN** getSubjectName is called with topic="my-topic", element=KEY, subjectNameStrategy=TOPIC_NAME_STRATEGY
- **THEN** "my-topic-key" is returned
