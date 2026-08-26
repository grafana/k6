## Why

Today every `getSchema` hits the registry and every `serialize`/`deserialize`
re-parses the schema string, so a script that resolves or (de)serializes per
iteration pays a network round-trip and a parse on every VU iteration. The
community `mostafa/xk6-kafka` exposes an `enableCaching` flag for exactly this;
our `index.d.ts` already declares it, but v1 accepts-and-ignores it and the spec
documents caching as "deferred to v2". This change makes `enableCaching`
functional.

## What Changes

- `enableCaching` (already in `index.d.ts`, currently ignored) becomes
  functional and gates the **registry-response cache** only. It defaults to
  `false`, preserving today's network behavior; when `true`, repeated
  `getSchema` of the same subject + version (including `latest`) skip the
  network. A new `EnableCaching bool` is stored on the `SchemaRegistry`
  (resolved from `SchemaRegistryConfig.enableCaching`; `false` in standalone
  mode, which has no config).
- **Parsed Avro schemas are always reused**, keyed by schema string, regardless
  of `enableCaching` and including standalone mode — it is a behavior-neutral
  optimization (parsing is deterministic), and standalone/inline serdes is
  exactly the per-iteration parse cost this change targets. (JSON has no compile
  step; only Avro is reused. A schema that fails to parse is not cached.)
- A `getSchema` cache hit returns an **independent copy** of the cached schema,
  so a caller mutating a returned schema cannot corrupt later cache hits.
- `createSchema` does **not** seed the cache: its response often omits the
  version, so there is no reliable cache key. A later `getSchema` performs the
  cacheable resolution.
- The registry cache is per-`SchemaRegistry` (per VU), lives for the client's
  lifetime, and is not invalidated — schemas are assumed stable for a test, so a
  cached `latest` reflects the first resolution (documented; can surface as a
  hard deserialize schema-id mismatch, not just stale data).
- `index.d.ts` doc-comment update only (no surface/type change). `enableCaching`
  is declared in two places: on `SchemaRegistryConfig` (now functional) and on
  the `Schema` object. The per-schema `Schema.enableCaching` is **accepted but
  ignored** in v1 — caching is governed solely by the client-level flag — so its
  doc comment (currently "keep a local copy … skips the network") is corrected
  to say it is accepted-but-ignored, reconciling the contract with the spec.

## Capabilities

### New Capabilities
<!-- None: caching is behavior of the existing schema-registry capability. -->

### Modified Capabilities
- `schema-registry`: `enableCaching` changes from accepted-ignored to functional,
  and `getSchema` no longer necessarily contacts the registry on every
  invocation. The "SchemaRegistry construction" and "Load schema from registry"
  requirements are updated, and "Registry response caching" and "Parsed schema
  reuse" requirements are added.

## Impact

- **Code**: `pkg/kafka/schema_registry.go` — add `EnableCaching` to the config
  and cache maps (guarded) to `SchemaRegistry`; `getSchema` reads/populates the
  response cache **when enabled**; `serialize`/`deserialize` reuse parsed Avro
  **always** (ungated).
- **Contract**: `index.d.ts` doc-comment fix for `Schema.enableCaching`
  (accepted-but-ignored); no type/surface change.
- **Docs**: README "Caching" limitation note updated to describe `enableCaching`
  and the cached-`latest` caveat.
- **Spec prose**: the schema-registry `## Purpose` line ("Caching is a planned
  v2 feature") must be reconciled at archive time (it is prose, not a
  delta-addressable requirement).
