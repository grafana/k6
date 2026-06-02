## Why

k6 exposes experimental and transitional engine capabilities through disconnected, ad-hoc environment variables (e.g. `K6_FOO_ENABLED`) and deeply nested config fields. There is no central registry, no shared lifecycle, and no discovery mechanism. Users cannot find what experiments exist or when they will be removed, and maintainers accumulate "flag rot" — obsolete toggles and conditional code that linger because nothing forces cleanup.

This change introduces **k6 Feature Flags**: a single standardized system governing how internal capabilities are named, configured, logged, observed, and retired. It formalizes the stakeholder-approved Product DNA & Design Doc (internal Google Doc, **not publicly available**) and supersedes the open GitHub proposal (k6 issue #4869).

## What Changes

- **Unified configuration** under three surfaces (CLI `--features`, env `K6_FEATURES`, JSON config `features`) with winner-takes-all precedence and a strict `kebab-case` naming convention.
- **Discovery command** `k6 features` (table + `--json`) that is self-documenting and lists every registered flag grouped by lifecycle.
- **Lifecycle model** — `Experimental` / `GA` / `Deprecated` — with per-stage runtime logging and a registry-removal **forcing function** that fails the build when retired flags are still referenced.
- **Near-zero hot-path overhead** via direct boolean field reads (no per-call map lookup/alloc/lock).
- **Observability**: per-sample metric tags (`k6_feature_<name>="true"`) and usage telemetry over the resolved activation set.
- **Legacy migration**: existing standalone env vars become registry aliases with a three-phase lifecycle (Honored → Tombstoned → Truly removed). Initial migration target: `K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM` → `native-histograms`.
- **Distributed execution**: resolved `K6_FEATURES` propagated to workers via the existing env-var passthrough; no new orchestrator contract.

## Capabilities

### New Capabilities
- `feature-flags`: central registry, configuration surfaces, lifecycle, discovery command, observability/telemetry, and legacy-alias migration for k6 internal feature toggles.

### Modified Capabilities
<!-- none — greenfield capability -->

## Impact

- **New code**: feature registry + reflection bootstrapper (singleton struct of typed boolean fields), activation resolver, `k6 features` subcommand.
- **CLI/cmd**: new `--features` global flag and `features` subcommand (`internal/cmd/`).
- **Config**: new top-level `features` JSON property and `K6_FEATURES` env handling (`lib/options.go`, config loading).
- **Metrics**: per-sample tag emission through existing tag machinery (`metrics/`, outputs).
- **Telemetry**: activation set added to usage report.
- **Migration**: `K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM` rewired as a Phase-1 honored alias for `native-histograms`.
- **Distributed**: Cloud API and k6-operator env-var passthrough carries `K6_FEATURES` (no orchestrator parsing).
- **Out of scope**: xk6 extension-defined flags, mid-test toggling, remote-fetched flag state, per-VU variation, `GA→Deprecated` path, operational config variables (e.g. `K6_PROFILING_ENABLED`).

## References

- Stakeholder-approved source document (Product DNA & Design Doc): internal Google Doc — **not publicly available**.
- Existing GitHub proposal (to be closed and redirected): k6 issue #4869.
- Naming-convention precedent: Prometheus `--enable-feature=<name>` flag style.
