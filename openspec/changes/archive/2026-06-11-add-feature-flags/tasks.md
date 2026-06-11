## 1. Registry & typed-identifier core

- [x] 1.1 Define the singleton feature struct with exported boolean fields and metadata struct tags (lifecycle stage, description)
- [x] 1.2 Implement the reflection bootstrapper that walks the struct at startup to wire metadata and validate canonical names against `^[a-z][a-z0-9]*(-[a-z0-9]+)*$` (hard error at definition time)
- [x] 1.3 Implement lifecycle stage enum (`Experimental`/`GA`/`Deprecated`) with lowercase log form and Title Case display form
- [x] 1.4 Unit-test: invalid definition-time name rejected; registry exposes name/lifecycle/description as single source of truth

## 2. Configuration surfaces & resolution

- [x] 2.1 Add `--features` CLI flag (Cobra/pflag, multi-flag union) and `K6_FEATURES` env binding
- [x] 2.2 Add top-level `features` JSON config property (`lib/options.go` / config loading)
- [x] 2.3 Implement split-on-comma-then-trim, drop empties, kebab-case validation of user input
- [x] 2.4 Implement "supplied" detection per surface (including empty-value-is-supplied) and winner-takes-all precedence
- [x] 2.5 Implement within-env-surface union (`K6_FEATURES` ∪ honored aliases) and legacy-alias canonical substitution
- [x] 2.6 Implement once-before-init evaluation producing a stable activation set; set struct fields; force `GA` fields true regardless of activation
- [x] 2.7 Unit-test resolution/precedence as pure deterministic functions (CLI>env>JSON, empty-override, env-union, GA-default-on)

## 3. Lifecycle logging & Unknown handling

- [x] 3.1 Emit exactly one structured log per activated Recognized flag at the lifecycle-appropriate level with `{ feature, lifecycle }`
- [x] 3.2 Emit `ERROR` for Unknown names with `{ feature/env, outcome:"unknown", source }`; run continues, normal exit code, no activation-set contribution
- [x] 3.3 Emit `WARN` `{ source:"options" }` when `options.features` is set; do not affect activation set
- [x] 3.4 Establish the uniform `source` vocabulary (`cli`/`env`/`env_legacy_alias`/`env_tombstoned_alias`/`json`/`options`)
- [x] 3.5 Unit-test each log level/field-set and the non-fatal/exit-code behavior

## 4. `k6 features` discovery command

- [x] 4.1 Add `features` subcommand printing a name/lifecycle/description table grouped `Experimental`→`Deprecated`→`GA`, alphabetical within group, Title Case lifecycle column
- [x] 4.2 Add `--json` flag emitting array of `{ name, lifecycle, description }` in default sort order; `[]` for empty registry
- [x] 4.3 Follow read-only subcommand exit-code conventions (cf. `k6 inspect`)
- [x] 4.4 Integration-test table output, JSON output, and that removed flags do not appear

## 5. Observability — metrics tags & telemetry

- [x] 5.1 Emit sparse per-sample boolean tags `k6_feature_<name_snake>="true"` for activated features via existing tag machinery (kebab→snake for tag key only)
- [x] 5.2 Verify propagation across output formats (JSON, CSV, Prometheus RW, Cloud, StatsD, Datadog, InfluxDB, OpenTelemetry)
- [x] 5.3 Add resolved activation set (sorted canonical kebab-case names) to usage telemetry
- [x] 5.4 Unit/integration-test: GA-not-activated emits no tag/telemetry; legacy-alias activation reported once under canonical name

## 6. Legacy alias migration & tombstones

- [x] 6.1 Implement Phase-1 honored alias: `strconv.ParseBool` value parsing, canonical-name contribution, dual-log emission (alias `WARN` + canonical lifecycle log)
- [x] 6.2 Implement tombstone subregistry and Phase-2 detection before value parsing (`ERROR` with migration hint regardless of value, no contribution, not "supplied")
- [x] 6.3 Implement Phase-3 "truly removed" silent path (escape hatch, documented at call site)
- [x] 6.4 Rewire `K6_PROMETHEUS_RW_TREND_AS_NATIVE_HISTOGRAM` as a Phase-1 honored alias for `native-histograms`
- [x] 6.5 Unit/integration-test all three phases end-to-end (independent of production list size)

## 7. Hot-path & maintainability guarantees

- [x] 7.1 Ensure gated engine branches are direct field reads (no map lookup/alloc/lock); verify via review and optional `go build -gcflags=-m`
- [x] 7.2 Add optional regression-detection micro-benchmark (no absolute-ns CI gate)
- [x] 7.2a Verify resolution/bootstrap cost is paid once at startup and does not noticeably impact CLI startup time
- [x] 7.3 Verify registry removal of an identifier fails the build at every gated site (forcing function)
- [x] 7.4 Add test demonstrating add → promote (`Experimental`→`GA`) → retire lifecycle end-to-end

## 8. Distributed execution

- [x] 8.1 Have the initiating CLI resolve locally and set `K6_FEATURES=<canonical,list>` in the Cloud/operator env-var passthrough bag (no new API contract)
- [x] 8.2 Confirm orchestrator forwards `K6_FEATURES` without parsing individual names; worker evaluates identically to local
- [x] 8.3 Test version-skew degradation: unknown name on worker logs `ERROR`, run continues, feature not applied

## 9. Docs & rollout

- [x] 9.1 Document `k6 features`, `--features`/`K6_FEATURES`/JSON surfaces, lifecycle, and migration in user docs
- [x] 9.2 Close and redirect GitHub proposal #4869
- [x] 9.3 Run `make check` (lint + race tests) and `openspec validate add-feature-flags --strict`
