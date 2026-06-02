## Context

k6's experimental capabilities are toggled today by ad-hoc `K6_*_ENABLED` env vars and nested config fields, with no registry, shared lifecycle, or discovery. This causes two problems: users cannot discover what experiments exist or when they retire, and maintainers accumulate flag rot because nothing forces removal of obsolete toggles and their conditional code.

The chosen design is recorded in the stakeholder-approved Product DNA & Design Doc (internal Google Doc — **not publicly available**). This document captures the non-normative implementation rationale; the binding behavior lives in the capability spec deltas under `specs/feature-flags/spec.md`.

## Goals / Non-Goals

**Goals:**
- One registry as the single source of truth for names, lifecycle, and descriptions.
- Three consistent configuration surfaces with deterministic winner-takes-all precedence.
- A discoverable, self-documenting `k6 features` command.
- Build-time forcing function that prevents flag rot on registry removal.
- Near-zero hot-path overhead (direct field reads).
- Observability via metric tags + telemetry, and structured migration of legacy env vars.

**Non-Goals:**
- xk6 extension-defined flags (feature package stays internal to k6 core).
- Mid-test toggling; per-VU/per-scenario/per-iteration variation. Flags are global, evaluated once before init.
- *Honoring* the script `options` block as a feature-activation surface. (Note: this change still ships the required warn-and-ignore behavior — detect `options.features`, emit `WARN { source: "options" }`, ignore it, continue. Only treating it as an activation surface is the non-goal.)
- Remote/dynamically fetched flag state.
- `GA → Deprecated` lifecycle path.
- Operational configuration variables (e.g. `K6_PROFILING_ENABLED`, `K6_CLOUD_TRACES_ENABLED`, `K6_WEB_DASHBOARD`) — distinguished by lifecycle intent: a feature flag is a toggle that **will eventually be removed**; operational config is intended to remain user-controllable indefinitely.
- An absolute nanosecond CI performance gate (sub-ns benchmark noise makes it unreliable).
- Exact log-message wording (illustrative only).

## Key domain model

- **Feature Definition** (compile time, maintainer): registers a flag, implements gated logic, ships the binary. **Feature Activation** (runtime, user): toggles an already-shipped flag on for one run. Lifecycle transitions act on the *definition*, never block *activation*.
- **Lifecycle stages**: `Experimental` (gated path runs only if activated), `GA` (gated path runs unconditionally; activation only nudges removal via `INFO`), `Deprecated` (gated path runs only if activated; activation triggers `WARN`). `Unknown` is a runtime resolution outcome, not a definable stage.
- **Lifecycle journey**: `Experimental` → `GA` → (grace) → removed, or `Experimental` → `Deprecated` → (grace) → removed. Registry removal is the forcing function.
- **Grace periods**: flexible, default guidance 1–2 minor releases, delegated to the retiring maintainer, informed by telemetry.
- **Removal action at gated sites**: when the struct field is deleted and the compiler flags each `if flags.X { ... }` site, the maintainer's action depends on how the feature ended. Adopted (was GA): keep the code inside the block, remove the `if` wrapper — the behavior is now unconditionally on. Abandoned (was Deprecated): delete the entire block — the behavior is being killed.
- **Activation set**: canonical names honored for one run = winner-takes-all surface choice → within-env union → legacy-alias canonical substitution. Drives metric tags and telemetry.
- **Resolution outcome**: each activation-set name is `Recognized` (matches registry → lifecycle log/behavior) or `Unknown` (`ERROR`, run continues, no contribution).

## Decisions

### D1 — Typed-identifier flags, not string-keyed lookups
The "registry removal must fail the build" invariant (maintainability requirement) excludes `if flags["foo"]` string lookups, which silently return zero when the key is gone. Each flag MUST be a named identifier engine code references directly, so removal produces a compile error at every gated site.

### D2 — Hand-written struct + reflection bootstrapper (chosen strategy)
Team consensus (Design Doc): flags declared as exported fields on a singleton struct; a small startup bootstrapper walks the struct via reflection to wire metadata (lifecycle, description) and apply user activation. Engine reads `flags.NewSummary` directly. Satisfies D1 and the typed-identifier family.

A build-time code generator emitting the struct from YAML was considered and rejected: it satisfies the same invariants but adds a `go generate` step and a separate source-of-truth file. Hand-written struct keeps definitions co-located with Go code. SDK-based and string-keyed strategies are excluded by D1.

#### Struct shape

Each flag is an exported `bool` field. Lifecycle stage, description, and optional name override are declared as struct tags on the same field:

```go
type Flags struct {
    NativeHistograms bool `lifecycle:"experimental" help:"Use HDR histograms for built-in metrics"`
    NewSummary       bool `lifecycle:"ga"           help:"Enable the new end-of-test summary format"`
    OldCsvOutput     bool `lifecycle:"deprecated"   help:"Keep the legacy CSV output format" name:"old-csv"`
}
```

Engine code gates on fields directly:

```go
if flags.NativeHistograms {
    // experimental code path
}
```

#### Canonical name derivation

The bootstrapper derives the canonical kebab-case name from the Go field name by inserting a hyphen before each uppercase letter and lowercasing everything (e.g., `NativeHistograms` → `native-histograms`, `NewSummary` → `new-summary`). If a `name` struct tag is present, it overrides the derived name (e.g., `OldCsvOutput` with `name:"old-csv"` uses `old-csv` instead of `old-csv-output`). The resulting canonical name is validated against `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`; a non-conforming name is a hard error at definition time.

### D3 — Winner-takes-all governs the activation set, not engine behavior
A higher-priority *supplied* surface fully overrides lower ones (enables "disable by omission"). But `GA` features are forced on at startup regardless of the activation set; omitting a `GA` flag from the winning surface does not turn its behavior off.

### D4 — Non-fatal Unknown names
Unknown names (typo, grace-removed flag, tombstoned alias) log `ERROR` but never halt the run and never force a non-zero exit. Rationale: removing a flag is routine cleanup; CI referencing it should not break, especially for a now-redundant `GA` flag whose behavior is on by default. Accepted trade-off: typos manifest as the feature silently not applied — users monitor CI `ERROR` logs. The build-time invariant (D1) stays strict; only the runtime invariant is relaxed.

### D5 — Per-sample metric tags, kebab→snake for tag keys only
Activation set surfaces as sparse per-sample boolean tags `k6_feature_<name_snake>="true"` (kebab `-` → `_` only for the tag key, for Prometheus compatibility). Reuses existing k6 tag machinery so all outputs inherit it. Telemetry uses canonical kebab-case directly (no char-set constraints).

### D6 — Legacy aliases as a three-phase mechanism
Honored (Phase 1) → Tombstoned (Phase 2) → Truly removed (Phase 3, escape hatch). The mandatory tombstone subregistry preserves a logged-`ERROR` surface after canonical removal so retired env vars aren't silent no-ops. Tombstone detection precedes value parsing. Initial migration: `K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM` → `native-histograms` (Phase-1 alias while canonical is `Experimental`). All three phases must be test-proven at first merge regardless of production list size.

### D7 — Distributed execution via env passthrough
The initiating CLI resolves locally and sets `K6_FEATURES` in the existing env-var passthrough bag. Orchestrators (Cloud, operator) forward it verbatim without parsing — no new API contract. Version skew degrades non-fatally per D4.

## Risks / Trade-offs

- **Reflection bootstrapper** adds a small startup cost and reflection complexity; mitigated by paying it once per process and keeping the struct small. Acceptable vs. a codegen build dependency.
- **Non-fatal typos (D4)**: a misspelled flag silently does nothing. Mitigated by a clear logged `ERROR`; users must watch CI logs. Deliberately favors CI continuity over halt-on-typo.
- **Version skew (D7)**: a feature requested across an older worker silently does not apply. Accepted to keep orchestrators flag-agnostic; users monitor worker logs.
- **Tombstone growth**: ~one entry per retired alias; cost grows slowly and is effectively permanent by default. `Truly removed` is the rare documented escape hatch.
