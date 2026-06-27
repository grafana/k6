## How to work these tasks

The whole capability is verified by two table-driven integration tests on the real command harness (`internal/cmd/tests`, `NewGlobalTestState`), one per surface: `TestRunReportsExtensions` for `k6 run` and `TestSubcommandReportsUsage` for `k6 x`. Nothing is mocked; extensions register through the normal paths and both the catalog and the report endpoint are local HTTP servers.

Each task is one increment and is self-contained: a fresh implementer needs only this task, the spec (`specs/extension-telemetry/spec.md`), the design (`design.md`), and the codebase. Every task follows the same shape:

- **Prereq**: what earlier tasks already put in place, so you know the starting state.
- **Row**: the scenario vector to add as one row of an existing table. Do not add a new test function; extend the table to avoid test churn.
- **Red or guard**: a red task drives new production code (write the row, watch it fail on its assertion before touching production code). A guard row passes the moment it is added because an earlier task already satisfied it; it exists to fail if someone regresses. If a guard row is red on arrival, an earlier task is broken.
- **Implement**: the exact production change.
- **Verify**: run the named test and confirm the expected red-to-green (or green-on-arrival), then `make lint` clean. The tree stays green at the end of every task, so the next task starts from a working state.

Tasks are ordered so each builds on the last. The two groundwork tasks have no dependency on recording and come first. Module-path placeholders (`M_import`, `M_import2`, `M_output`, `M_sub`, `M_fork`) match the design's testing tables.

## Groundwork

- [ ] 1. **Share catalog access between run and x** (spec: "Public-catalog access is shared between run and x")
  - Prereq: none. This is the recommended first commit.
  - `fetchCatalog` (`internal/cmd/x_registry.go:110`), `readCachedCatalog` (`:81`), and `writeCachedCatalog` (`:141`) already live in package `cmd`, called from `subcommand.go`, so there is no cross-package move. Add `catalogModulePaths(ctx, gs)` beside them. It returns the set of Go module paths in the catalog, acquired exactly the way `k6 x` does so it shares the same cache file, TTL, and `K6_PROVISION_CATALOG_URL` override: read the on-disk cache (`writeCachedCatalog` stores raw JSON); on a miss, `fetchCatalog(ctx, cmp.Or(gs.Env[state.ProvisionCatalogURL], defaultCatalogURL()))` and `writeCachedCatalog` the fresh bytes, with `ctx` bounded by the report send so the fetch is non-fatal and never on the run's hot path. Parse those raw bytes with a new struct that reads each entry's `module` field, which the existing `catalogEntry`/`registrySubcommands` decoder drops. Both the run report (task 7) and the subcommand report (task 14) call this one helper, so the acquisition is not duplicated. Optionally rename the file to a neutral name (for example `catalog.go`) as a pure move with no logic change.
  - Test: this is a behaviour-preserving refactor of the shared fetch, so the "test" is that `k6 x` catalog resolution is unchanged. Keep the existing `k6 x` catalog tests green; if none exercises membership through `K6_PROVISION_CATALOG_URL`, add one characterization test that a name resolves identically before and after. Do not add a bespoke unit test for `catalogModulePaths`; it gets behavioural coverage from the filter task (7) via the run table, so a separate unit test would be inflation. Every catalog fixture from here on MUST carry a `module` field per entry (production shape: top-level keys are import specifiers and `subcommand:` tokens, each entry has a `module`); the legacy `testCatalogJSON` keys by module path with no `module` field, so reusing it verbatim yields an empty set.
  - Verify: `go test -race ./internal/cmd/ -run 'TestX|TestExtensionSubcommands'` stays green; `make lint` clean.

- [ ] 2. **Make the report endpoint observable** (spec: harness baseline for "Behaviour is verified by table-driven integration tests")
  - Prereq: none.
  - Row: create `TestRunReportsExtensions` with its first row. GIVEN any catalog, WHEN a trivial script runs under `k6 run` with reporting enabled and `K6_USAGE_REPORT_URL` pointed at an `httptest.Server`, THEN that server receives the existing usage report. This row is the harness baseline, not a spec scenario; it proves the seam every later run-surface row reuses.
  - Red: it fails because the report endpoint is a hardcoded `const`.
  - Implement: add `K6_USAGE_REPORT_URL` to `cmd/state/state.go` (defaulting to the production endpoint) and read it at the send site in `internal/cmd/report.go`. No other behaviour change.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestRunReportsExtensions` red then green; `make lint` clean.

## Importable extension telemetry

- [ ] 3. **Record and report an imported extension** (spec: "Report used catalog extensions" / "Used public import is reported", "Importable extension usage is recorded", "Extension record shape" / "Entry carries module, version, and kind")
  - Prereq: task 2's `K6_USAGE_REPORT_URL` seam and the `TestRunReportsExtensions` table.
  - Row: GIVEN the stand-in catalog lists `M_import`, WHEN `k6 run` runs a script importing `k6/x/testimport`, THEN the report's `extensions` holds one entry `{M_import, version, kind: "js"}`.
  - Test setup: register the in-tree `k6/x/testimport` extension once through a `sync.Once` (mirror `registerTestSubcommandExtensions` in `internal/cmd/subcommand_test.go:416`), because `ext.Register` panics on a duplicate name/type and the registry is process-global; every later run-table extension (tasks 5, 11) registers through that same once.
  - Red: it fails because `js/modules/resolution.go` deliberately drops `k6/x/*` imports and no code assembles `extensions`.
  - Implement: in `js/modules/resolution.go`, at the cached resolution site, route imports identified as extensions by registry membership (`ext.Get(ext.JSExtension)`, not by the `k6/x/` name) into a new `extensions` bucket on the `usage.Usage` collector; leave the `modules` list unchanged. Split `internal/cmd/report.go` into `createReport` plus a shared `postUsageReport`, and add `enrichExtensions` to resolve each recorded name through `ext.Get(ext.JSExtension)[name].Path`, attach version and kind from the registry, and emit the bucket.
  - By construction (no separate row): `version` is empty for an in-tree test extension because it carries no build dependency, so assert `module` and `kind` and treat `version` as the verbatim `ext.Extension.Version`. Because the reporting code reads the running binary, an xk6 build and a provisioning build report identically, so build-method independence needs no row.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestRunReportsExtensions` red then green; `make lint` clean.

- [ ] 4. **De-duplicate a module imported by many VUs** (spec: "A module imported by many VUs is recorded once")
  - Prereq: task 3's recording and `enrichExtensions`.
  - Row: GIVEN the catalog lists `M_import`, WHEN `k6 run --vus 5 --iterations 5` runs a script importing `k6/x/testimport`, THEN `extensions` holds exactly one `js` entry for `M_import`.
  - Red: it fails if recording happens per import rather than once per module.
  - Implement: de-duplicate per (module, kind) inside `enrichExtensions`. Recording at the cached resolution site already collapses repeated VU imports; this makes the guarantee explicit at assembly.
  - By construction (no separate row): because dedup is per (module, kind), a single module used as both an import and an output would yield two entries, one per kind. The in-tree harness cannot make one module resolve to both an import path and an output constructor name, so dual-purpose coexistence needs no row.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestRunReportsExtensions` red then green; `make lint` clean.

- [ ] 5. **Report each of several imported extensions** (spec: "Multiple imported extensions are each reported")
  - Prereq: task 3 (appends each resolved import) and task 4 (dedup per module).
  - Row: GIVEN the catalog lists `M_import` and `M_import2`, WHEN `k6 run` runs a script importing both `k6/x/testimport` and `k6/x/testimport2`, THEN `extensions` holds one `js` entry for each.
  - Guard: passes once tasks 3 and 4 land; it exists to fail if recording overwrites a single slot or dedup collapses distinct modules. Register the two in-tree test extensions in two distinct Go packages so they resolve to distinct module paths (a same-package pair resolves to one path and dedup would collapse them), and list both paths in the stand-in catalog.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestRunReportsExtensions` green on arrival; `make lint` clean.

- [ ] 6. **Omit the key when nothing was used** (spec: "Compiled-but-unused extension is not reported")
  - Prereq: task 3's recording.
  - Row: GIVEN a catalogued extension compiled into the binary, WHEN `k6 run` runs a script that imports, selects, and invokes nothing, THEN the report omits the `extensions` key entirely (never `[]`).
  - Red for the omission half: the empty bucket serializes as `[]`. Make it pass by omitting the key when the bucket is empty, consistent with how `modules` behaves. The compiled-but-unused half is a guard: it already holds because task 3 records only at the import site, so an unused extension is never recorded.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestRunReportsExtensions` red then green; `make lint` clean.

## Filtering

- [ ] 7. **Filter reported extensions against the catalog** (spec: "Public-catalog filter via the registry catalog" / "Extension not in the catalog is filtered out")
  - Prereq: task 1's `catalogModulePaths`, task 3's `enrichExtensions`.
  - Row: GIVEN the stand-in catalog omits `M_import`, WHEN `k6 run` runs a script importing `k6/x/testimport`, THEN the report holds no entry for it.
  - Red: it fails because task 3 reports everything recorded. Make it pass inside `enrichExtensions`: resolve each recorded name to its module path via `ext.Get(<type>)[name].Path`, then keep only paths in the set returned by `catalogModulePaths(ctx, gs)` (task 1), which reads the catalog through the same cache-then-fetch path as `k6 x`. Call it within the bounded, non-fatal report send, never on the run's hot path.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestRunReportsExtensions` red then green; `make lint` clean.

- [ ] 8. **Match by module path, not import name** (spec: "Private fork reusing a public import name is dropped")
  - Prereq: task 7's filter.
  - Row: GIVEN the catalog lists the real `M_import` for import name `k6/x/testimport` while an in-tree fork registered under the same import name resolves to `M_fork`, WHEN `k6 run` runs a script importing that name, THEN the report holds no entry, because matching is by module path.
  - Guard: passes as soon as task 7 lands; it exists to fail if the match ever compares import names instead of module paths. No production change beyond task 7.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestRunReportsExtensions` green on arrival; `make lint` clean.

## Privacy

- [ ] 9. **Fail closed when the catalog is unavailable** (spec: "No errors when the catalog is unavailable" / "Unavailable catalog reports nothing")
  - Prereq: task 7's filter.
  - Row: GIVEN `K6_PROVISION_CATALOG_URL` points at an unreachable server with no cache, WHEN `k6 run` runs a script importing a used extension, THEN the report holds no extensions AND the run exits 0.
  - Red: it fails because task 7's filter has no failure handling, so a fetch error either aborts the run or leaks the unfiltered bucket. Make it pass by dropping the bucket before the fetch and reporting nothing when the catalog cannot be read, keeping the fetch and send bounded and non-fatal.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestRunReportsExtensions` red then green; `make lint` clean.

- [ ] 10. **Opt-out suppresses everything on the run path** (spec: "Extension telemetry respects the existing opt-out" / "Opt-out suppresses extension data")
  - Prereq: tasks 2, 7, 9 (the send and the catalog fetch now sit behind report assembly).
  - Row: GIVEN `K6_NO_USAGE_REPORT=true` with both stand-in servers wired, WHEN `k6 run` runs a script importing a catalogued extension, THEN neither the report endpoint nor the catalog server receives a request.
  - Guard: passes as long as the existing opt-out gate stays ahead of report assembly; it exists to fail if extracting `postUsageReport` (task 3) or adding the catalog fetch (task 7) moved work in front of that gate. No production change if the gate already wraps enrichment.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestRunReportsExtensions` green on arrival; `make lint` clean.

## Output extension telemetry

- [ ] 11. **Report a selected output extension** (spec: "Output extension usage is reported" / "Catalogued output extension is reported")
  - Prereq: task 3's `enrichExtensions`, task 7's filter (output rides the same bucket and filter).
  - Row: GIVEN the catalog lists `M_output`, WHEN `k6 run --out testoutput` runs a script, THEN `extensions` holds one entry `{M_output, version, kind: "output"}`. Register an in-tree output extension via `output.RegisterExtension` and list its resolved path in the stand-in catalog.
  - Red: it fails because output extensions are never recorded. Make it pass by recording a selected `--out` into the `extensions` bucket in `internal/cmd/outputs.go` when the output is an extension (membership in `ext.Get(ext.OutputExtension)`), tagged kind `output`, and let `enrichExtensions` resolve it via `ext.Get(ext.OutputExtension)[name].Path`. Leave the built-in `outputs` recording untouched.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestRunReportsExtensions` red then green; `make lint` clean.

- [ ] 12. **Drop a non-catalog output extension** (spec: "Non-catalog output extension is dropped")
  - Prereq: task 11's output recording, task 7's filter.
  - Row: GIVEN the catalog omits `M_output`, WHEN `k6 run --out testoutput` runs a script, THEN the report holds no entry for it.
  - Guard: passes because task 7's filter already covers the output bucket; it exists to fail if the output path bypasses the filter. No new production change if the filter already covers this bucket.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestRunReportsExtensions` green on arrival; `make lint` clean.

- [ ] 13. **Leave built-in outputs unchanged** (spec: "Built-in output is unchanged")
  - Prereq: task 11's output recording.
  - Row: GIVEN a built-in output such as `json`, WHEN `k6 run --out json` runs a script, THEN the report holds no `kind: "output"` entry AND `outputs` still lists `json`.
  - Guard: passes because built-ins are classified by the built-in output enum and never enter the extensions bucket; it exists to fail if output recording ever broadens to all outputs instead of output-registry members. This is what keeps the literal-registered `web-dashboard` out of the bucket. Keep the built-in path in `internal/cmd/outputs.go` unchanged.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestRunReportsExtensions` green on arrival; `make lint` clean.

## Subcommand extension telemetry

- [ ] 14. **Report a baked-in subcommand run** (spec: "Subcommand extension usage is reported" / "Catalogued subcommand run is reported", "At most one usage report per invocation")
  - Prereq: task 2's `K6_USAGE_REPORT_URL` seam and `postUsageReport`, task 1's `catalogModulePaths`.
  - Row: create `TestSubcommandReportsUsage` with its first row, mirroring task 2's setup. GIVEN the catalog lists `M_sub` for a baked-in subcommand `testsub`, WHEN the user runs `k6 x testsub`, THEN the stand-in server receives exactly one report `{M_sub, version, kind: "subcommand"}` AND a marker the subcommand body writes is present, proving the wrapped run hook still ran.
  - Red: it fails because no subcommand reports. Make it pass by having `getCmdForExtension` in `internal/cmd/subcommand.go` wrap the command's run hook (preserving whichever of `Run` or `RunE` it set, since Cobra runs `RunE` and skips `Run` when both exist) and call a new `reportSubcommandUsage` once after the hook returns, sending `k6_version`, `is_ci`, `goos`, `goarch`, and the entry through the shared `postUsageReport`, keeping the entry only if its module path is in the set returned by `catalogModulePaths(ctx, gs)` (task 1, the same shared cache-then-fetch helper the run path uses), with `ctx` bounded like the run report.
  - By construction (no separate row): the baked-in report is tested end to end here; a provisioned subcommand reports once from the child that runs it, because the host never builds the command for an extension it lacks, so the provisioned-child once-only case holds without a row the single-binary harness cannot drive.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestSubcommandReportsUsage` red then green; `make lint` clean.

- [ ] 15. **Drop a private subcommand** (spec: "Registered private subcommand is dropped")
  - Prereq: task 14's subcommand report, task 1's `catalogModulePaths`.
  - Row: GIVEN a baked-in subcommand whose module path the catalog omits, WHEN the user runs `k6 x <name>` to completion, THEN the report holds no entry for it.
  - Red: it fails because task 14 reports every subcommand that runs. Make it pass by reusing the module-path filter before sending.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestSubcommandReportsUsage` red then green; `make lint` clean.

- [ ] 16. **Never report an unknown subcommand** (spec: "Unknown subcommand is not reported")
  - Prereq: task 14's subcommand report.
  - Row: GIVEN `K6_PROVISION_CATALOG_URL` points at an unreachable server, WHEN the user runs `k6 x not-a-catalog-name`, THEN neither stand-in server receives a request, because the name cannot be provisioned and the host builds no command for it.
  - Guard: passes because the host never builds a command for an extension it lacks, so the wrap never runs; it exists to fail if `reportSubcommandUsage` is ever called before the wrapped hook actually runs the extension.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestSubcommandReportsUsage` green on arrival; `make lint` clean.

- [ ] 17. **Never delay or fail the command on a slow endpoint** (spec: "Slow endpoint does not delay or fail the command")
  - Prereq: task 14's `reportSubcommandUsage` and `postUsageReport`.
  - Row: GIVEN `K6_USAGE_REPORT_URL` points at an unreachable or slow server and the catalog lists `testsub`, WHEN the user runs `k6 x testsub`, THEN the command exits 0 with no error surfaced, the send is abandoned within the same bounded timeout the run report uses, and only a debug-level log records the failure (asserted through the harness `LoggerHook`).
  - Red: it fails because a naive send blocks or propagates the error. Make it pass by routing the subcommand send through the bounded, non-fatal `postUsageReport`.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestSubcommandReportsUsage` red then green; `make lint` clean.

- [ ] 18. **Opt-out suppresses the subcommand report** (spec: "Extension telemetry respects the existing opt-out" / "Opt-out suppresses extension data")
  - Prereq: task 14's `reportSubcommandUsage`.
  - Row: GIVEN `K6_NO_USAGE_REPORT=true` with both stand-in servers wired, WHEN the user runs `k6 x testsub`, THEN neither server receives a request.
  - Red: it fails because `reportSubcommandUsage` is a new send with no opt-out check (the `--no-usage-report` flag lives on the per-test `Config`, not `GlobalState`). Make it pass by gating the send on the `K6_NO_USAGE_REPORT` env var, which reaches a provisioned child through `buildSubprocessEnv`.
  - Verify: `go test -race ./internal/cmd/tests/ -run TestSubcommandReportsUsage` red then green; `make lint` clean.
