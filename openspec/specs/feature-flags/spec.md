# feature-flags Specification

## Purpose

TBD: created by archiving change add-feature-flags. Update Purpose after archive.

## Requirements

### Requirement: Configuration surfaces with winner-takes-all precedence

The system MUST accept feature flag activation from three configuration surfaces, in priority order: (1, highest) CLI flag `--features a,b` — multiple `--features` flags MUST merge into a single union per standard Cobra/pflag behavior; (2) environment variable `K6_FEATURES=a,b`; (3, lowest) JSON config file top-level `features` array.

A higher-priority surface, if *supplied*, completely overrides lower-priority surfaces. The system MUST NOT merge or union lists across surfaces. This is deliberate: a higher-priority surface can disable a feature a lower-priority surface enabled by simply omitting it.

This rule governs the **activation set**, not engine behavior. Omitting a `GA` flag from the winning surface MUST NOT suppress that feature's behavior — promotion to `GA` is a permanent engine-default shift, independent of what the user passes.

#### Scenario: CLI overrides JSON entirely
- **GIVEN** the JSON config sets `"features": ["feature-a", "feature-b"]`
- **WHEN** the user runs `k6 run --features feature-a script.js`
- **THEN** the CLI surface is supplied and overrides JSON with no merge
- **AND** the activation set is exactly `{feature-a}`
- **AND** if `feature-b` is `Experimental` or `Deprecated`, its gated code path is off
- **AND** if `feature-b` is `GA`, its behavior is on regardless (only the GA `INFO` log is suppressed by the omission)

#### Scenario: Multiple CLI flags merge
- **WHEN** the user passes `--features a --features b`
- **THEN** the CLI surface activation set is the union `{a, b}`

### Requirement: Definition of "supplied"

The system MUST treat a surface as *supplied* if and only if the user provided a value through it. CLI is supplied when `--features` was passed with any value (including `--features ""`). Env is supplied when `K6_FEATURES` is set with any value (including empty), OR any honored (non-tombstoned, Phase-1) legacy alias env var is set with a truthy value. JSON is supplied when the config object contains a `features` key with any value (including `[]`).

A surface that is not supplied is skipped and the next-down surface is consulted. An explicit empty value at any surface IS supplied and DOES override lower-priority surfaces — this is the documented mechanism for clearing inherited activation. Legacy aliases set to a falsy value, and tombstoned aliases regardless of value, MUST NOT mark the env surface as supplied.

#### Scenario: Empty value overrides and activates nothing
- **GIVEN** `K6_FEATURES=feature-a` is set
- **WHEN** the user runs `k6 run --features "" script.js`
- **THEN** the CLI surface is supplied and overrides env
- **AND** the activation set is empty

#### Scenario: Falsy legacy alias does not mark env supplied
- **GIVEN** `K6_FOO_ENABLED=false` (honored alias) and a JSON config with `features`
- **WHEN** the engine resolves surfaces
- **THEN** the env surface is not supplied
- **AND** the JSON surface is consulted normally

### Requirement: Kebab-case naming convention

All feature flag names MUST use `kebab-case` matching the regex `^[a-z][a-z0-9]*(-[a-z0-9]+)*$` — lowercase ASCII letters/digits, hyphen-separated tokens, no leading digit, no leading/trailing hyphen, no consecutive hyphens, at least one character.

Validation boundary depends on origin: at definition time (compile time, maintainer-authored) a non-conforming canonical name MUST be rejected as a hard error; at runtime parse (user input from any surface) a non-conforming name MUST be treated as Unknown — `ERROR` logged, run continues, name does not enter the activation set. A name MUST resolve to exactly one canonical identifier across all surfaces. Inputs MUST be processed with split-on-comma-then-trim; after trim, empty strings are silently dropped before regex validation.

#### Scenario: Invalid definition-time name rejected
- **GIVEN** a maintainer defines a flag with canonical name `Native-Histograms`
- **WHEN** the registry is initialized
- **THEN** the system rejects it as a hard error

#### Scenario: Invalid user-input name treated as Unknown
- **WHEN** the user runs `k6 run --features Native-Histograms script.js`
- **THEN** the name resolves as Unknown, an `ERROR` is logged naming the offending input, the run continues, and the name does not enter the activation set

### Requirement: Evaluation timing and scope

Feature flags MUST be evaluated once, before test initialization, and the resolved state MUST remain stable for the entire run. Mid-test toggling and per-VU/per-scenario/per-iteration variation are not supported.

#### Scenario: State stable for the run
- **WHEN** flags are resolved at startup
- **THEN** the resolved activation set does not change for the duration of the run

### Requirement: Activation drives field state

The activation-resolution step MUST set the boolean field corresponding to every canonical name in the resolved activation set to `true`. Names not in the activation set leave their fields at `false`, except as overridden by the GA-default rule. Resolution happens once at startup; engine code reads `if flags.MyFeature { ... }` and the field value determines whether the gated path runs.

#### Scenario: Activated name sets field true
- **GIVEN** `native-histograms` is `Experimental` and in the activation set
- **WHEN** resolution completes
- **THEN** the `native-histograms` boolean field is `true` and its gated code path runs

#### Scenario: Unactivated experimental field stays false
- **GIVEN** an `Experimental` flag not in the activation set
- **THEN** its field is `false` and its gated code path does not run

### Requirement: GA flags are forced on at startup

The activation-resolution step MUST set the boolean field of every `GA`-lifecycle flag to `true` regardless of whether the user's activation set contains it. The user's activation set still drives the GA `INFO` log (emitted only when the user activated the flag) and the metric tag (emitted only for activated names). Engine-side default (forced true) is independent from user-side activation.

#### Scenario: GA field forced true without activation
- **GIVEN** `js-modules-v2` is `GA` and the user did not activate it
- **THEN** the `js-modules-v2` field is `true`, no GA `INFO` log is emitted, and no metric tag is emitted

#### Scenario: GA field forced true with activation
- **GIVEN** `js-modules-v2` is `GA` and the user activated it
- **THEN** the field is `true`, a GA `INFO` "please remove this flag" log is emitted, and the tag `k6_feature_js_modules_v2="true"` is emitted

### Requirement: Script options block is unsupported

Setting feature flags from inside a script's `options` block MUST NOT be supported. If the script's `options` object contains a `features` key, the engine MUST emit a `WARN` naming the supported surfaces (`--features` / `K6_FEATURES` / JSON config) before the test starts, carrying the structured field `{ "source": "options" }`. The activation set MUST be unaffected and the run continues normally.

#### Scenario: options.features warns and is ignored
- **GIVEN** `script.js` exports `options` containing a `features` key
- **WHEN** the user runs `k6 run script.js` with no other feature configuration
- **THEN** a `WARN` is emitted with field `{ "source": "options" }`
- **AND** the activation set is unaffected (empty here) and the run continues

### Requirement: Hot-path overhead is a direct field read

Per-call overhead in engine hot paths MUST be a direct boolean field read (or equivalent constant-time access) — no per-call map lookup, string compare, allocation, syscall, or lock on the gated branch. This is a structural requirement on the access path, verified by code review (and optionally `go build -gcflags=-m` inliner inspection), not a wall-clock threshold.

Resolution cost (parsing, validating, logging, and any reflection-based bootstrap) MUST be paid once per process at startup — never on a hot path — and MUST NOT noticeably impact CLI startup time perceived by users. Expensive startup reflection or validation that perceptibly slows CLI startup violates this requirement even if the hot-path read is structurally correct.

#### Scenario: Gated branch performs no map lookup
- **WHEN** a reviewer inspects an engine gated branch
- **THEN** the access is a direct struct-field read with no map lookup, string compare, allocation, syscall, or lock

#### Scenario: Resolution paid once at startup
- **WHEN** the engine resolves feature flags
- **THEN** parsing/validation/logging/bootstrap runs once at startup, not per hot-path call
- **AND** it does not noticeably impact user-perceived CLI startup time

### Requirement: Logging behavior per lifecycle stage

When an activated name resolves as Recognized, the engine MUST emit exactly one structured log per activated flag, at a level determined by lifecycle stage: `Experimental` → `INFO` ("experimental, may change/removed"); `GA` → `INFO` ("now available by default, please remove this flag"); `Deprecated` → `WARN` ("deprecated, will be removed"). Exact wording is illustrative; normative are (a) the level per stage, (b) exactly one message per activated recognized flag, and (c) structured `logrus` fields including `{ "feature": "<canonical-name>", "lifecycle": "<stage>" }` where `<stage>` is the lowercase form (`experimental`/`ga`/`deprecated`).

The `source` field uses a single uniform vocabulary across all emission points: `cli`, `env`, `env_legacy_alias`, `env_tombstoned_alias`, `json`, `options` — all snake_case, single-token, no dots. `feature` always carries a canonical kebab-case name; `env` always carries a legacy `K6_*` var name; `lifecycle` always carries a lowercase stage discriminator (`experimental`/`ga`/`deprecated`/`deprecated_alias`).

#### Scenario: Experimental activation emits one INFO
- **GIVEN** `native-histograms` is `Experimental` and activated
- **THEN** exactly one `INFO` log is emitted with fields `{ "feature": "native-histograms", "lifecycle": "experimental" }`

#### Scenario: Deprecated activation emits WARN
- **GIVEN** a `Deprecated` flag is activated
- **THEN** exactly one `WARN` log is emitted with `lifecycle: "deprecated"`

### Requirement: Unknown names are logged but non-fatal

When an activated name resolves as Unknown (typo, grace-period-removed flag, or tombstoned legacy alias) in any surface, the engine MUST emit an `ERROR`-level log AND the run MUST continue. The process MUST exit with the normal test result exit code (which can be `0`). The unknown name MUST NOT contribute to the activation set. There is no Unknown-class outcome that halts the run, and the startup error does not force a non-zero exit code.

The `ERROR` MUST carry structured fields: `outcome` = literal `unknown` (always); `source` = one of `cli`/`env`/`env_legacy_alias`/`env_tombstoned_alias`/`json` (always); `feature` = the user-supplied canonical kebab-case name (present for `cli`/`env`/`json`/`env_legacy_alias`, optional/omitted for `env_tombstoned_alias`); `env` = the user-supplied legacy `K6_*` var name (only for `env_legacy_alias`/`env_tombstoned_alias`).

For the tombstoned-alias case (`source = env_tombstoned_alias`), the `ERROR` MUST also include a migration hint naming the canonical surfaces (`--features` / `K6_FEATURES`), so the retirement path stays actionable — a bare "unknown" message is insufficient when an old env var is being retired.

#### Scenario: Typo logs ERROR and continues
- **GIVEN** the registry has `new-summary` but not `new-sumary`
- **WHEN** the user runs `k6 run --features new-sumary script.js`
- **THEN** an `ERROR` is logged with fields `{ "feature": "new-sumary", "outcome": "unknown", "source": "cli" }`
- **AND** the run continues and `new-sumary` does not enter the activation set

#### Scenario: GA flag removed after grace period still completes
- **GIVEN** `<removed-name>` was previously `GA` and is now removed from the registry
- **WHEN** the user's CI runs `k6 run --features <removed-name> script.js`
- **THEN** it resolves as Unknown, an `ERROR` is logged, and the run continues
- **AND** because the underlying feature was promoted to `GA` its behavior is on by default, so the test outcome is unchanged

### Requirement: k6 features discovery command

The system MUST provide a `k6 features` subcommand that prints a table of all currently registered flags. The table MUST show three columns for every flag — feature, lifecycle, and description — so the command is self-documenting and requires no other documentation source to convey what flags exist and what they do. The table column headers are displayed uppercase (`FEATURE`, `LIFECYCLE`, `DESCRIPTION`); the corresponding `--json` keys are the lowercase forms (`feature`, `lifecycle`, `description`). Default sort: grouped by lifecycle in the order `Experimental` → `Deprecated` → `GA` (most-actionable first), alphabetical by feature within each group. The lifecycle column uses Title Case display (`Experimental`/`GA`/`Deprecated`). The command MUST list only flags currently in the registry (removed flags MUST NOT appear), and MUST NOT reflect run-specific activation state.

The command MUST support a `--json` flag emitting a JSON array of objects with exactly the keys `feature` (kebab-case), `lifecycle` (Title Case), `description` (human-readable), in the same sort order; an empty registry MUST emit `[]`. Exit codes MUST follow existing read-only k6 subcommand conventions (e.g. `k6 inspect`).

#### Scenario: Default table grouped and sorted
- **GIVEN** the registry contains a mix of `GA`, `Experimental`, and `Deprecated` flags
- **WHEN** the user runs `k6 features`
- **THEN** every registry flag is listed with its feature, lifecycle, and description columns, grouped `Experimental` → `Deprecated` → `GA`, alphabetical within each group, and removed flags do not appear

#### Scenario: JSON output is valid and ordered
- **WHEN** the user runs `k6 features --json`
- **THEN** the output is a valid JSON array of objects each with exactly `feature`, `lifecycle`, `description`
- **AND** the array follows the default sort order
- **AND** an empty registry produces `[]`

### Requirement: Metrics metadata tags

The system MUST attach the resolved activation set as per-sample k6 tags so downstream platforms can correlate metric values with active features. For each feature in the activation set, every emitted sample carries a boolean tag `k6_feature_<name_snake>="true"`, where `<name_snake>` is the canonical kebab-case name with `-` substituted to `_` (Prometheus-compatible). Tags are sparse — emitted only for activated features; the value is always the literal `"true"` (no `"false"` tags). A `GA` flag's tag is emitted only if the user activated it, not because the feature is on by default. Because it is a standard k6 tag, it propagates to every output format via existing tag machinery.

#### Scenario: Activated feature emits snake_case tag
- **GIVEN** `native-histograms` is in the activation set
- **THEN** every metric sample carries `k6_feature_native_histograms="true"`

#### Scenario: Unactivated feature emits no tag
- **GIVEN** a defined-but-unactivated flag
- **THEN** no tag is emitted for it

### Requirement: Usage telemetry

Active feature flags MUST be included in k6 usage telemetry alongside existing usage signals. Telemetry MUST report the same resolved activation set that the metrics tags expose, as a sorted list of canonical kebab-case names (NOT the snake_case tag form — telemetry has no character-set constraints). A defined-but-unactivated flag does not appear; a `GA` flag the user did not activate does not appear; a flag activated via a legacy alias appears once under its canonical kebab-case name.

#### Scenario: Telemetry reports canonical names
- **GIVEN** the activation set is `{native-histograms, new-summary}`
- **THEN** telemetry reports the sorted canonical list `["native-histograms", "new-summary"]`

#### Scenario: Legacy-alias activation reported canonically
- **GIVEN** `foo` was activated via legacy alias `K6_FOO_ENABLED=true`
- **THEN** telemetry reports `foo` once, not the env var name

### Requirement: Legacy env var to canonical name distinction

Existing standalone environment variables (e.g. `K6_FOO_ENABLED`) MUST be integrated into the unified registry rather than kept as parallel mechanisms. Two names are involved: the **legacy env var name** (fixed by history, never renamed) and the **canonical feature name** (kebab-case, the only identifier appearing in `k6 features`, the three surfaces, metrics, and telemetry). A legacy env var is purely an alias: setting it contributes the canonical name to the activation set. Its value MUST be parsed with `strconv.ParseBool` semantics (`true`/`1`/`t`/`T`/`TRUE` activate; everything else is ignored with no log) — for honored, Phase-1 aliases only. The legacy env var name is never reported back as a feature identifier; it appears only in deprecation warnings and tombstone errors.

#### Scenario: Truthy alias contributes canonical name
- **GIVEN** `K6_FOO_ENABLED` is a Phase-1 honored alias for canonical `foo`
- **WHEN** `K6_FOO_ENABLED=true` is set
- **THEN** `foo` is added to the activation set

### Requirement: Legacy alias lifecycle phases

A legacy alias MUST pass through two default phases plus an optional third escape hatch. Phase 1 (Honored alias): env var honored, canonical feature defined with independent lifecycle; contributes canonical name to the activation set and emits `WARN`. Phase 2 (Tombstoned): env var recognized but rejected, canonical removed; emits `ERROR` (Unknown-class), run continues, no canonical name added. Phase 3 (Truly removed, escape hatch): env var invisible, silently ignored.

The alias phase is independent of the canonical feature's lifecycle stage. The tombstone phase is mandatory machinery — without it, setting a retired env var after canonical removal would be a silent no-op, defeating the Unknown-name visibility. Tombstones live as a small subregistry of retired alias names, are effectively permanent by default (no automatic promotion to Truly removed), and `Truly removed` is a documented per-alias maintainer decision. The initial implementation MUST prove all three phases end-to-end via unit and/or integration tests, independent of production migration-list size.

#### Scenario: Phase 1 honored alias activates feature
- **GIVEN** a Phase-1 honored alias set to a truthy value
- **THEN** the canonical feature activates, a `WARN` is emitted, and the run continues

#### Scenario: Phase-1 alias with removed canonical errors and warns
- **GIVEN** a Phase-1 honored alias `K6_FOO_ENABLED` maps to canonical `foo`
- **AND** `foo` has been removed from the registry (maintainer has not yet moved alias to Phase 2)
- **WHEN** `K6_FOO_ENABLED=true` is set
- **THEN** the alias-side `WARN` is emitted (Phase-1 alias deprecation)
- **AND** `foo` resolves as Unknown, an `ERROR` is emitted with `{ "feature": "foo", "outcome": "unknown", "source": "env_legacy_alias" }`
- **AND** `foo` does not enter the activation set and the run continues

Note: when removing a canonical from the registry, maintainers SHOULD move its Phase-1 aliases to Phase 2 (tombstoned) to avoid this transitional state.

#### Scenario: Phase 2 tombstoned alias errors and continues
- **GIVEN** a tombstoned alias env var is set
- **THEN** an `ERROR` is emitted with a migration hint naming `--features` / `K6_FEATURES`, no feature is activated, and the run continues

#### Scenario: Phase 3 truly-removed alias is silent
- **GIVEN** a truly-removed alias env var is set
- **THEN** it is silently ignored, no log is emitted, and the run continues

### Requirement: Order of operations for legacy env-surface resolution

Tombstone detection MUST happen during env-surface parsing and MUST precede value parsing. As soon as a tombstoned env var name is observed set in the environment, the engine emits the Unknown `ERROR` for it regardless of truthy/falsy/empty value; `strconv.ParseBool` does not apply to tombstoned aliases. A tombstoned env var contributes nothing to the env-surface activation set and never marks the env surface as supplied. Whether env is supplied is determined as usual by `K6_FEATURES` or any honored (non-tombstoned) alias.

#### Scenario: Tombstone errors regardless of value
- **GIVEN** `K6_FOO_ENABLED` is tombstoned
- **WHEN** it is set to `false`, `true`, or empty
- **THEN** all three produce the same outcome: `ERROR` logged (including a migration hint naming `--features` / `K6_FEATURES`), no activation-set contribution, env surface not marked supplied by it

### Requirement: Log emissions for legacy alias activation

When a Phase-1 honored alias is activated, the engine MUST emit a deprecation `WARN` that includes a migration hint naming the canonical surfaces (`--features` and `K6_FEATURES`) — a bare "feature is deprecated" is insufficient. The `WARN` MUST carry fields `{ "feature": "<canonical>", "env": "<K6_*>", "source": "env_legacy_alias", "lifecycle": "deprecated_alias" }`.

Activation via a Phase-1 alias MUST emit **two** logs (dual emission, intentionally not deduplicated): (1) the alias-side `WARN` above (env-var migration), and (2) the canonical-side log per the lifecycle-stage logging rule (feature lifecycle). The "exactly one message per activated recognized flag" rule applies to the canonical-side log only.

#### Scenario: Dual log on legacy-alias activation
- **GIVEN** `K6_FOO_ENABLED=true` is a Phase-1 honored alias for `foo`, and `foo` is `Experimental`
- **THEN** an alias-side `WARN` is emitted with `{ "feature": "foo", "env": "K6_FOO_ENABLED", "source": "env_legacy_alias", "lifecycle": "deprecated_alias" }`
- **AND** a canonical-side `INFO` is emitted with `{ "feature": "foo", "lifecycle": "experimental" }`
- **AND** `foo` activates

### Requirement: Within-env-surface merge

Inside the env surface, `K6_FEATURES` and any number of legacy alias env vars may coexist; the engine MUST treat them as a set union (each alias contributing its canonical kebab-case name). Duplicates collapse. Legacy aliases can only contribute names, never disable a feature. The merged env-surface set then competes with CLI and JSON under winner-takes-all. `K6_FEATURES=""` contributes the empty set and MUST NOT reach across to suppress independent legacy alias env vars. There is no single-knob "clear everything in env" mechanism — to clear env contributions the user must unset both `K6_FEATURES` and every legacy alias.

#### Scenario: Empty K6_FEATURES unions with legacy alias
- **GIVEN** `K6_FEATURES=""` and `K6_FOO_ENABLED=true` (honored alias for `foo`) are both set
- **THEN** the env-surface activation set is `{} ∪ {foo}` = `{foo}`
- **AND** the env surface is supplied and overrides JSON, and `foo` activates

### Requirement: Distributed execution propagation

When a test runs in a distributed/cloud environment with a control plane (`k6 cloud`, k6-operator), the local/initiating CLI MUST resolve the activation set locally and convey the canonical resolved kebab-case list by setting `K6_FEATURES=a,b` in the existing env-var passthrough bag — no new orchestrator API contract or typed `features` field. The orchestrator MUST NOT parse or validate individual flag names; it passes `K6_FEATURES` through to workers using its standard machinery. Workers evaluate the injected variable identically to local execution. Version skew is an accepted consequence of the non-fatal Unknown rule: an unknown name on an older worker logs `ERROR`, the run continues, and the feature is silently not applied; users are responsible for monitoring worker logs.

#### Scenario: CLI resolves, orchestrator passes through
- **WHEN** the local CLI resolves the activation set `{native-histograms}`
- **THEN** it sets `K6_FEATURES=native-histograms` in the env-var passthrough bag
- **AND** the orchestrator forwards it to workers without parsing individual names
- **AND** each worker evaluates `K6_FEATURES` identically to local execution

#### Scenario: Version skew degrades non-fatally
- **GIVEN** a worker binary whose registry lacks a propagated feature name
- **THEN** that name resolves as Unknown on the worker, an `ERROR` is logged, the run continues, and the feature is not applied

### Requirement: Single localized flag definition (maintainability)

Defining a new flag MUST be a localized change in one place — the system MUST NOT require parallel edits across multiple files (registry + lookup table + docs) to stay consistent. Lifecycle metadata (stage, description) MUST live next to the flag definition, not in a separate document that can drift.

#### Scenario: New flag added in one place
- **WHEN** a maintainer defines a new flag
- **THEN** the name, lifecycle stage, and description are declared together in one location, with no required parallel edits elsewhere

### Requirement: Registry removal is a build-time forcing function

Removing a feature definition from the registry MUST cause any engine code still gated on that feature to fail to build (not merely emit a runtime warning). This requires gating engine code on typed named identifiers rather than string-keyed lookups, because a string-keyed lookup silently returns a zero value when the key is removed and would violate the build-time loud-failure invariant. This build-time invariant is separate from and stricter than the runtime non-fatal Unknown-name rule.

#### Scenario: Removed flag still referenced fails build
- **GIVEN** a flag has been removed from the registry
- **AND** engine code still references its identifier
- **WHEN** the project is built
- **THEN** the build fails with a compile error at every gated site

#### Scenario: Adding, promoting, and retiring is test-demonstrable
- **THEN** adding a flag, promoting it `Experimental` → `GA`, and retiring it MUST each be demonstrable end-to-end with unit and/or integration tests

### Requirement: Cross-surface consistency

A given flag name MUST behave identically across all three surfaces (CLI, env, JSON). Kebab-case violations MUST be rejected symmetrically for definition-time names and runtime user input (per the naming requirement's per-origin boundary). The list rendered by `k6 features`, the list accepted by `--features`, and the metadata attached to metrics MUST all derive from the same single source of truth and cannot drift.

#### Scenario: Same name same behavior across surfaces
- **GIVEN** a registered flag name
- **WHEN** it is supplied via CLI, env, or JSON
- **THEN** it resolves to the same canonical identifier and produces identical behavior

### Requirement: Testability of registry and resolution

The flag registry, lifecycle metadata, and naming rules MUST be verifiable by the project's standard unit-test runner without a full binary build or invocation. Resolution and precedence behavior MUST be testable as pure/deterministic units independent of the rest of k6 startup machinery.

#### Scenario: Resolution tested as a pure unit
- **WHEN** resolution and precedence logic is unit-tested
- **THEN** it runs deterministically without invoking full k6 startup

### Requirement: Dependency footprint

The feature flag system MUST be implementable without heavyweight external dependencies and SHOULD avoid adding them; industry feature-flag SDKs are designed for long-running services and add machinery k6 does not need.

#### Scenario: No heavyweight SDK added
- **WHEN** the system is implemented
- **THEN** it does not pull in a heavyweight external feature-flag SDK
