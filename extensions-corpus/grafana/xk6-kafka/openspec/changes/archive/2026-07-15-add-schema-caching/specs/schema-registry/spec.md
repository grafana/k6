## MODIFIED Requirements

<!-- "SchemaRegistry construction" is reconciled directly in the base spec: this
     change renames its enableCaching scenario, which the archiver's delta
     application cannot express without dropping a scenario. The base spec also
     needed a manual structural repair (its requirements were under a stray
     "## ADDED Requirements" header), so the construction update is applied there. -->

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

## ADDED Requirements

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
