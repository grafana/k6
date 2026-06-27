## ADDED Requirements

Every scenario below is written as a test vector: the **GIVEN** fixes the catalog data (and any relevant build state), the **WHEN** fixes the script and the k6 command, and the **THEN** fixes the expected usage report. These vectors become the rows of two table-driven integration tests, one per command surface (`k6 run`, `k6 x`). The tests use no Go-level mocks; the catalog and the report endpoint are stand-in local servers, so no report reaches the production endpoint (see the testing requirement at the end).

<!-- Phase 1: Importable extension (JS) telemetry -->

### Requirement: Report used catalog extensions in the usage report

The usage report MUST include an `extensions` field listing the extensions actually used during the run. An extension is "used" when the test exercises it, not merely when it is compiled into the binary: an importable extension MUST be reported only if one of its `k6/x/*` modules was resolved, an output extension MUST be reported only if it was selected with `--out`, and a subcommand extension MUST be reported only if it was invoked.

The field MUST contain only extensions present in the public registry catalog. Extensions absent from the catalog MUST NOT appear. When no catalog extension was used, the field MUST be omitted, consistent with how `modules` behaves today: the key appears only once a value is recorded, so an empty `extensions` array is never emitted.

#### Scenario: Used public import is reported
- **GIVEN** the stand-in catalog lists the module path that `k6/x/sql` resolves to
- **WHEN** `k6 run` executes a script that imports `k6/x/sql`
- **THEN** the report `extensions` contains one entry for that module path with kind `js`

#### Scenario: Compiled-but-unused extension is not reported
- **GIVEN** a catalogued extension compiled into the binary, and a script that imports, selects, and invokes nothing
- **WHEN** `k6 run` executes that script
- **THEN** the report omits the `extensions` key rather than sending an empty array

### Requirement: Importable extension usage is recorded

Importable JS extensions (Go extensions registered in the binary, today imported under the `k6/x/` prefix) that are resolved during a run MUST be recorded for reporting. This reverses the current behaviour, where `k6/x/` modules are deliberately excluded from usage collection. An import MUST be identified as an extension by membership in the extension registry rather than by its name, so detection keeps working if extensions are ever imported under a name that is not prefixed with `k6/x/`. Recording MUST happen at the same cached resolution site as built-in modules, so each distinct extension module is recorded once regardless of how many VUs import it. The report-assembly step MUST de-duplicate extension entries per (module, kind), so a module recorded more than once on one surface appears once.

#### Scenario: A module imported by many VUs is recorded once
- **GIVEN** the stand-in catalog lists the module path behind `k6/x/sql`
- **WHEN** `k6 run --vus 5 --iterations 5` executes a script that imports `k6/x/sql`
- **THEN** the report `extensions` contains exactly one `js` entry for that module path

#### Scenario: Multiple imported extensions are each reported
- **GIVEN** the stand-in catalog lists the module paths behind `k6/x/sql` and `k6/x/redis`
- **WHEN** `k6 run` executes a script that imports both `k6/x/sql` and `k6/x/redis`
- **THEN** the report `extensions` contains one `js` entry for each

### Requirement: Extension record shape

Each entry in the `extensions` field MUST identify the extension by its Go module path and MUST include its version and its kind. The module path and the version MUST come from the binary build information, the same source as `k6 version`. The kind MUST be one of `js`, `output`, or `subcommand`.

#### Scenario: Entry carries module, version, and kind
- **GIVEN** the stand-in catalog lists the module path behind an imported `k6/x/sql`
- **WHEN** `k6 run` builds the report
- **THEN** the entry reports that module path, the version taken verbatim from build info (empty for an in-tree test extension, which carries no build dependency), and kind `js`

<!-- Phase 2: Filtering -->

### Requirement: Public-catalog access is shared between run and x

The catalog fetch (`fetchCatalog`) already lives in the `cmd` package (in `internal/cmd/x_registry.go`, used by `k6 x`), so both the run path (imports and outputs) and the subcommand path can call it without a second fetch. The run path MUST reuse that same fetch rather than add its own, and a shared helper `catalogModulePaths` MUST read the per-entry `module` fields that the existing `catalogEntry` decoder drops. The file MAY be renamed to a neutral name (for example `catalog.go`) to show it is shared rather than `k6 x`-specific; that rename is a pure move with no behaviour change. `k6 x` behaviour MUST NOT change: the same registry URL, on-disk cache, TTL, and `K6_PROVISION_CATALOG_URL` override. This groundwork is self-contained and can land before any recording or filtering work.

#### Scenario: Shared catalog access leaves k6 x unchanged
- **GIVEN** `K6_PROVISION_CATALOG_URL` points at a stand-in catalog, after `catalogModulePaths` is added beside the existing `fetchCatalog`
- **WHEN** `k6 x` resolves catalog membership for a name
- **THEN** it resolves identically to before the change (same URL, cache, TTL, override), guarded by the existing `k6 x` catalog tests

### Requirement: Public-catalog filter via the registry catalog

The system MUST decide whether an extension is public by comparing its Go module path against the public registry catalog. It MUST reuse the shared catalog fetch described in the previous requirement: the same registry URL, the same on-disk cache and TTL, and the same `K6_PROVISION_CATALOG_URL` override. Each catalog entry carries a `module` field with its Go module path; the catalog's top-level keys are import specifiers and subcommand tokens, not module paths. The system MUST build the public module-path set from those `module` fields, and MUST NOT report any extension whose module path is absent from that set.

Each surface records the resolved extension at the point of use, not its name: the registry entry it records already carries the Go module path and version (read from build info), so assembly performs no re-resolution. Matching MUST be by module path, never by the recorded name, so a private fork that reuses a public import name resolves at its recording site to its own module path and is dropped.

The catalog is fetched, or read from the existing cache, as part of the bounded, non-fatal usage-report send, never on the run's hot path. A newly catalogued extension becomes reportable as soon as the cache refreshes, with no new k6 release required.

#### Scenario: Extension not in the catalog is filtered out
- **GIVEN** the stand-in catalog omits the module path of a used extension
- **WHEN** `k6 run` executes a script that imports it and reports usage
- **THEN** the report omits that extension

#### Scenario: Private fork reusing a public import name is dropped
- **GIVEN** a fork registered under the import name `k6/x/sql` that resolves to module path `github.com/acme/xk6-sql-fork`, while the stand-in catalog lists only the real `github.com/grafana/xk6-sql`
- **WHEN** `k6 run` executes a script that imports `k6/x/sql` and reports usage
- **THEN** the report omits it, because matching is by module path and the fork resolves to its own path

<!-- Phase 3: Privacy -->

### Requirement: No errors when the catalog is unavailable

If the registry catalog cannot be fetched or read from cache, or is empty or cannot be parsed, the system MUST report no extensions and MUST NOT fail or interrupt the run. Reporting nothing is the privacy-safe default; the system MUST NEVER emit unfiltered extension data as a fallback.

#### Scenario: Unavailable catalog reports nothing
- **GIVEN** `K6_PROVISION_CATALOG_URL` points at an unreachable server and no cached copy exists
- **WHEN** `k6 run` executes a script that imports a used extension
- **THEN** the report contains no extensions
- **AND** the run exits 0

### Requirement: Extension telemetry respects the existing opt-out

Extension telemetry MUST NOT introduce a separate opt-out. It MUST be suppressed whenever the existing usage report is disabled, via `--no-usage-report` or `K6_NO_USAGE_REPORT`, across the run path and the subcommand path. When usage reporting is enabled, extension data is included by the same mechanism that sends the rest of the report.

#### Scenario: Opt-out suppresses extension data
- **GIVEN** `K6_NO_USAGE_REPORT=true`, `K6_USAGE_REPORT_URL` pointed at a stand-in server, and the catalog listing the extension
- **WHEN** `k6 run` executes a script that imports it, or `k6 x <name>` runs a catalogued subcommand
- **THEN** the stand-in server receives no request

#### Scenario: No separate flag is required
- **GIVEN** usage reporting enabled (the default) and the stand-in catalog listing the extension
- **WHEN** `k6 run` executes a script that imports it
- **THEN** the extension is reported with no additional flag or environment variable

<!-- Phase 4: Subcommand usage telemetry -->

### Requirement: Subcommand extension usage is reported

Subcommand extensions (`k6 x <name>`) MUST report their usage through the same shared usage-report mechanism every command uses: the same collector, transport, opt-out, and bounded, debug-logged, never-failing send, not a bespoke path. The report is sent from the seam that runs the command and thus learns which command actually ran, reporting only when that command is a registered subcommand extension: a baked-in subcommand reports in-process; a provisioned subcommand reports from the provisioned child that runs it, because the host never builds the command for an extension it lacks and so never reports. The report MUST be sent whether the subcommand exited zero or non-zero, since telemetry counts the invocation regardless of outcome, and MUST NOT alter the command's own run hook. Only subcommands present in the registry catalog MUST be reported.

Reporting the subcommand name, version, and kind is sufficient. The subcommand's own arguments or internal behaviour are out of scope. The subcommand report carries the identifying fields the run report already sends that do not depend on a run (`k6_version`, `is_ci`, `goos`, `goarch`) alongside the `extensions` entry, and omits the run-only execution stats (`duration`, `iterations`, `vus_max`, `executors`) that require a run scheduler.

#### Scenario: Catalogued subcommand run is reported
- **GIVEN** the stand-in catalog lists the module path behind a baked-in subcommand `testsub`
- **WHEN** the user runs `k6 x testsub`
- **THEN** the stand-in server receives exactly one report listing that module path with kind `subcommand`

#### Scenario: Unknown subcommand is not reported
- **GIVEN** `K6_PROVISION_CATALOG_URL` points at an unreachable server
- **WHEN** the user runs `k6 x not-a-catalog-name`, an unregistered name
- **THEN** the stand-in server receives no request, because the name cannot be provisioned and the host builds no command for it

#### Scenario: Registered private subcommand is dropped
- **GIVEN** a baked-in subcommand whose module path the stand-in catalog omits
- **WHEN** the user runs `k6 x <name>` to completion
- **THEN** the report contains no entry for it

#### Scenario: Slow endpoint does not delay or fail the command
- **GIVEN** `K6_USAGE_REPORT_URL` points at an unreachable or slow server and the catalog lists `testsub`
- **WHEN** the user runs `k6 x testsub`
- **THEN** the command exits 0 with no error surfaced
- **AND** the send is abandoned within the same bounded timeout the run report uses
- **AND** only a debug-level log records the failure

<!-- Phase 5: Output extension telemetry -->

### Requirement: Output extension usage is reported

Output extensions (a third-party `--out` provided by an `xk6-output-*` extension) that a run selects MUST be reported with kind `output`, filtered against the public catalog by module path like every other surface. An output MUST be identified as an extension by membership in the output registry (`ext.Get(ext.OutputExtension)`), not by the built-in output enum, so built-in outputs, including the literal-registered `web-dashboard`, keep their current reporting and never reach the `extensions` bucket. Usage MUST be recorded at the point the output is selected and constructed (`internal/cmd/outputs.go`), leaving the existing built-in `outputs` list untouched.

The public registry catalog lists no output extensions today, so in production no output is reported until the catalog lists them. The code path is built now so it activates automatically once they are catalogued; adding output extensions to the catalog is tracked separately.

#### Scenario: Catalogued output extension is reported
- **GIVEN** the stand-in catalog lists the module path that an output extension `testoutput` resolves to
- **WHEN** `k6 run --out testoutput` executes a script
- **THEN** the report `extensions` contains one entry for that module path with kind `output`

#### Scenario: Non-catalog output extension is dropped
- **GIVEN** the stand-in catalog omits the module path behind `testoutput`
- **WHEN** `k6 run --out testoutput` executes a script
- **THEN** the report contains no entry for it

#### Scenario: Built-in output is unchanged
- **GIVEN** a built-in output such as `json`
- **WHEN** `k6 run --out json` executes a script and reports usage
- **THEN** the report contains no `extensions` entry of kind `output`
- **AND** the `outputs` field still lists `json`

<!-- Existing-behaviour preservation -->

### Requirement: At most one usage report per invocation

A single k6 invocation MUST hit the report URL at most once. `k6 run` sends its own report, as it does today, and nothing in this change alters that. For the subcommand path, exactly one process reports: a baked-in subcommand reports in-process, and a provisioned subcommand reports only from the child that runs it, because the host never builds the command for an extension it lacks. The launcher does not report, so no child suppression is needed.

#### Scenario: Provisioned subcommand reports once, from the child
- **GIVEN** a `k6 x docs` that triggers binary provisioning
- **WHEN** the command completes
- **THEN** exactly one usage report is sent, by the provisioned child that runs the command
- **AND** the host process sends no report

### Requirement: Coverage is independent of build method

Extension usage MUST be reported identically whether the running binary was produced by `xk6` or by k6 binary provisioning. The reporting code lives in k6 core and reads what is compiled into the running binary, so it MUST NOT depend on how the binary was built.

#### Scenario: Provisioned binary reports like an xk6 binary
- **GIVEN** two binaries containing the same catalog extension, one built by xk6 and one produced by binary provisioning
- **WHEN** each runs the same test that uses the extension
- **THEN** both emit the same `extensions` entry

<!-- Testing -->

### Requirement: Behaviour is verified by table-driven integration tests

Every behaviour in this capability MUST be verified by integration tests that drive real k6 commands through the standard command test harness (`internal/cmd/tests`, `NewGlobalTestState`) and assert on the usage report that is actually sent, not on internal functions. Unit tests of report internals MUST NOT be the means of verifying these behaviours. The tests MUST be table-driven, with exactly two tables, one per command surface (`k6 run` and `k6 x`), each iterating over the scenario vectors above, rather than a separate verbose test function per scenario. Nothing may be mocked: extensions register through the normal `modules.Register` / `output.RegisterExtension` / `ext.Register` paths, and both the catalog and the report endpoint are real local HTTP servers, so no report leaves the test process.

Each scenario above is an acceptance criterion and MUST be exercised end to end through a real `k6 run` or `k6 x` invocation, except four cases the single-binary harness cannot drive, which hold by construction: the exact `version` value in the record-shape scenario (a test extension is absent from build deps, so its version resolves to empty; the test asserts `module` and `kind` and treats `version` as the verbatim `ext.Extension.Version`), the provisioned-child once-only scenario (provisioning runs a separately built binary the harness cannot produce; the baked-in once-only is tested end to end and the provisioned child holds because the host never builds the command for an extension it lacks), build-method independence (the reporting code reads the running binary, so xk6 and provisioning report identically), and a same-module extension used as both an import and an output (an import resolves to its package path and an output to its constructor's function name, converging only for a real external dependency, so the harness cannot make one in-tree module match both; the (module, kind) de-duplication handles it by construction, and there is no separate scenario for it). The shared-catalog groundwork scenario is a behaviour-preserving refactor verified by the existing `k6 x` catalog tests, separate from the two telemetry tables.

To make the sent report observable, a test MUST enable reporting (the harness disables it by default) and point the report endpoint at a local stand-in server via the new `K6_USAGE_REPORT_URL` env var, then assert on the received JSON. Catalog membership MUST be controlled through the existing `K6_PROVISION_CATALOG_URL` override, pointed at a stand-in catalog whose `module` fields match each test extension's resolved module path. For the private-fork case, the stand-in catalog MUST map the public import name to the real catalogued module path while the in-tree fork resolves to a different module path, so the drop is exercised by the module-path mismatch rather than by a missing catalog entry. The single new env var (`K6_USAGE_REPORT_URL`, the overridable report endpoint) is the only production addition the tests require beyond the feature itself; the catalog reuses the existing override, and no other production code or comments may change for testing.

#### Scenario: A reported behaviour is exercised end to end
- **GIVEN** the command test harness with reporting enabled, the report endpoint pointed at a local server, and the stand-in catalog listing a used extension
- **WHEN** `k6 run` executes a script that imports that extension
- **THEN** the test asserts the received report's `extensions` contains it with its module, version, and kind

#### Scenario: Opt-out is verified by the absence of a request
- **GIVEN** `K6_NO_USAGE_REPORT=true` and the endpoint pointed at a local server
- **WHEN** `k6 run` executes a script that imports a catalogued extension
- **THEN** the local server receives no request
