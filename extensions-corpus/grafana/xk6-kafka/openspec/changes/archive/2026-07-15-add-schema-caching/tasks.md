## 1. Config + cache structures

- [x] 1.1 Add `EnableCaching bool` (json `enableCaching`) to `SchemaRegistryConfig`; resolve it into a `bool` field on `SchemaRegistry` (`false` in standalone mode, where `config` is nil — no nil deref)
- [x] 1.2 Add a response cache (`map[string]*Schema` keyed by subject + version, used only when caching is enabled) and an always-on parsed-Avro cache (`map[string]avro.Schema` keyed by schema string) to `SchemaRegistry`, guarded by a `sync.RWMutex`
- [x] 1.3 Add a `cacheKey(subject, version)` helper (version `""` = latest) and a `parsedAvro(schemaStr)` helper that parses once and caches (always on; a parse failure is not cached)

## 2. Registry response cache (gated by enableCaching)

- [x] 2.1 When caching is enabled, `getSchema` returns a **copy** of the cached schema on hit (`c := *cached; return &c`, no registry request); on miss it fetches, stores, and returns. When disabled, it fetches every time (unchanged)
- [x] 2.2 Confirm `createSchema` does NOT seed the cache (its response has no reliable version) — no code needed beyond a comment noting why

## 3. Parsed-schema reuse (always on)

- [x] 3.1 Route `serialize`/`deserialize` Avro parsing through `parsedAvro(...)` so a repeated schema string is parsed once, regardless of `enableCaching` and including standalone mode

## 4. Tests

- [x] 4.1 Unit: with caching enabled, a second `getSchema` for the same subject+version issues no second registry request (httptest server counts requests) and returns an equal schema
- [x] 4.2 Unit: with caching disabled (default), two `getSchema` calls both hit the registry
- [x] 4.3 Unit: `parsedAvro` parses a given schema string once and returns the same parsed value on repeat — and does so with caching **disabled** too (always on)
- [x] 4.4 Unit: distinct versions (`latest` vs explicit) are cached separately
- [x] 4.5 Unit: `createSchema` does not seed — with caching on, `createSchema(subject)` then `getSchema(subject)` still issues a registry request
- [x] 4.6 Unit: cached `latest` is stale-stable — with caching on, `getSchema(latest)` (registry returns v1), then `createSchema` of a newer version, then `getSchema(latest)` returns the originally cached schema with no second registry request
- [x] 4.7 Per-schema `enableCaching` is ignored — satisfied by construction: the Go `Schema` struct has no `enableCaching` field, so the JS field is dropped on decode and cannot affect behavior (no runtime test possible/needed)
- [x] 4.8 Unit: a `getSchema` hit returns an independent copy — mutating the returned schema does not change a subsequently returned cached value
- [x] 4.9 Unit: standalone (`config == nil`) serialize reuses the parsed Avro schema across calls

## 5. Docs

- [x] 5.1 Update the README "Caching" note: `enableCaching` (opt-in) governs registry-response caching; parsed-schema reuse is always on; a cached `latest` is not refreshed mid-run and can surface as a hard `deserialize` schema-id mismatch (not just stale data)
- [x] 5.2 Fix the `Schema.enableCaching` doc comment in `index.d.ts` (currently "keep a local copy … skips the network") to state it is accepted-but-ignored in v1 (caching is client-level via `SchemaRegistryConfig.enableCaching`); no type/surface change

## 6. Archive

- [ ] 6.1 At archive time, reconcile the schema-registry `## Purpose` prose ("Caching is a planned v2 feature") to reflect the now-functional `enableCaching` (prose is not a delta-addressable requirement)
