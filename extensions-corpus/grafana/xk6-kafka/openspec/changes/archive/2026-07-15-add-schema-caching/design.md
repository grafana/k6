## Context

`SchemaRegistry` is constructed per VU (from the module instance) and its methods
are called from the VU's JS execution, which is single-goroutine per VU. Today
`getSchema` issues an HTTP GET every call and `serialize`/`deserialize` call
`avro.Parse(schema.Schema)` every call. Under load both are per-iteration costs.
The community extension caches schemas by id/subject and caches parsed codecs;
the current spec documents the absence as a v1 gap.

## Goals / Non-Goals

**Goals:**
- Avoid repeated registry round-trips for the same subject+version.
- Avoid re-parsing the same Avro schema string on every serdes call.
- Keep it simple and correct for the per-VU, test-scoped lifetime.

**Non-Goals:**
- Cross-VU / shared cache (each VU has its own `SchemaRegistry`).
- Cache invalidation / TTL / max-size eviction — schemas are stable per test.
- Caching by schema **id** for auto-fetch on deserialize (deserialize takes the
  schema in its `Container`, so it never fetches).

## Decisions

- **`enableCaching` gates only the registry-response cache (default false).**
  The flag is already in `index.d.ts` but accepted-ignored; this change makes it
  functional for the network-behavior cache (repeated `getSchema` skips the
  registry). Default `false` preserves today's network behavior and honors the
  declared flag; gating this cache is warranted because it changes observable
  behavior (a cached `latest` goes stale). The resolved value is stored as a
  `bool` on `SchemaRegistry` (read from `config.enableCaching`; `false` in
  standalone mode, where `config` is nil — so no nil deref).

- **Parsed-schema reuse is always on (not gated).** `schemaStr → avro.Schema` is
  a pure, deterministic function; memoizing it changes only speed, never output.
  Gating it would (a) pointlessly re-parse on the default path, and (b) be
  impossible to enable in standalone mode, which has no config object to carry
  the flag — yet standalone/inline serdes is exactly the per-iteration parse
  cost this change targets. So it is always on, independent of `enableCaching`.
  A schema string that fails to parse is not cached, so parse errors still
  surface on every call.

- **Two caches on `SchemaRegistry`.** A response cache
  `map[string]*Schema` keyed by `subject + "\x00" + version` (version `""`
  meaning `latest`), and a parsed-schema cache `map[string]avro.Schema` keyed by
  the raw schema string. Both guarded by a single `sync.RWMutex`. Access is only
  from the VU's JS goroutine today (no hook or background access, unlike the
  metrics collector), so the lock is defensive rather than load-bearing — kept
  because a concurrent map write is fatal and the cost is negligible.

- **`getSchema` reads/writes the response cache.** On a miss it fetches, stores,
  and returns; on a hit it returns a **copy** of the cached `*Schema` (`c :=
  *cached; return &c`) without a request. The copy matters because sobek exposes
  Go struct fields live to JS, so a script mutating a returned schema would
  otherwise corrupt every later cache hit. The cache key uses the requested
  version, so `latest` and an explicit version are distinct entries.

- **`createSchema` does NOT seed the cache.** Its response often omits the
  version (Confluent's `POST /subjects/{subject}/versions` typically returns only
  the id, so `Version` is 0/unknown), so there is no reliable subject+version
  key, and seeding a `latest` key would make a later `getSchema latest`
  ambiguous. A subsequent `getSchema` does the cacheable resolution. *Alternative:*
  a follow-up GET after create to learn the version — rejected: extra round-trip
  for marginal benefit; out of scope.

- **`parsedAvro(schemaStr)` helper.** `serialize`/`deserialize` route their
  `avro.Parse` through it (always-on reuse, per the decision above). JSON serdes
  use `encoding/json` (no compile step), so only Avro is memoized.

- **Unbounded but tiny.** A test uses a handful of distinct schemas, so the maps
  stay small; no eviction. *(ponytail: unbounded map, add LRU only if a real
  workload registers unbounded distinct schemas — not expected.)*

- **`latest` is cached like any key.** For a load test schemas are static, so
  serving a cached `latest` for the run is correct and matches community
  behavior. The staleness is documented (spec + README) rather than engineered
  away with revalidation.

- **Per-schema `Schema.enableCaching` is accepted-ignored.** `index.d.ts`
  declares `enableCaching` on both `SchemaRegistryConfig` and the `Schema`
  object. v1 honors only the client-level flag; the per-schema field is accepted
  but ignored (no Go `Schema` field needed — the unknown JS key is dropped on
  decode). *Alternative:* per-schema caching override — rejected: "which wins,
  client or schema" adds gating complexity for no v1 benefit. Documented as
  accepted-ignored, consistent with the project's other legacy-option handling.
  Because `index.d.ts` is authoritative and currently documents the per-schema
  field as functional ("keep a local copy … skips the network"), its **doc
  comment is corrected** to say accepted-but-ignored — a comment-only change, no
  type or surface change.

- **Spec Purpose prose.** The schema-registry `## Purpose` says "Caching is a
  planned v2 feature". That line is prose, not a delta-addressable requirement,
  so archiving this change won't rewrite it; it must be reconciled manually at
  archive time (a tasks item covers this).

## Risks / Trade-offs

- **Stale `latest` mid-run.** → Documented; acceptable for load tests (opt-in via
  `enableCaching`). Sharper than "stale data": `deserialize` enforces a
  wire-format schema-id match, so a cached `latest` consuming messages written
  with an evolved (higher-id) schema throws a schema-id mismatch, not a silent
  old decode. Called out in the spec and the README caveat. A user needing a
  fresh resolve can construct a new `SchemaRegistry` or leave caching off.
- **Cache-hit mutation.** → Resolved: `getSchema` hits return a copy (decision
  above), so a caller mutating a returned schema cannot corrupt the cache.
- **Memory growth if a script registers many unique schemas.** → Bounded in
  practice; noted above.
