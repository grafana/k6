## Context

The usage report (`internal/cmd/report.go`) builds a `map[string]any` from a shared `*usage.Usage` collector plus fixed fields and POSTs it to `https://stats.grafana.org/k6-usage-report` after a `k6 run`, gated by `K6_NO_USAGE_REPORT`. Extensions are currently excluded at the collection point: `js/modules/resolution.go:81-87` guards module recording with `!strings.HasPrefix(name, "k6/x/")`. Extension version and kind are already available at runtime via `ext.GetAll()` (`ext/ext.go`), populated from build info.

This change reports public-catalog extension usage for the importable, output, and subcommand surfaces, filtered against the public registry catalog by reusing the existing cached catalog fetch, as a single PR. The output path is built now; because the catalog lists no output extensions yet, it reports nothing in production until they are catalogued (tracked separately).

## Goals / Non-Goals

Goals:
- Report public-catalog extension usage for importable, output, and subcommand extensions.
- Never leak private or unlisted extensions.
- No new endpoint, no embedded catalog snapshot, no new opt-out; reuse the existing catalog fetch and cache.

Non-Goals:
- Unique-installation identity (the denominator that makes counts interpretable); see the [telemetry issue](https://github.com/grafana/k6/issues/4077).
- Distinguishing xk6-built vs provisioning-built binaries (tracked in [grafana/k6#6111](https://github.com/grafana/k6/issues/6111)).
- Reporting unlisted or private extensions, even anonymously.
- Secret-source extensions (a non-user-facing `ExtensionType`). Of the four extension types, only the three user-facing ones (importable, output, subcommand) are in scope; this is why `kind` has exactly three values (`js`, `output`, `subcommand`).
- Populating the catalog with output extensions. The output code path ships now, but the public registry catalog lists none, so no output is reported in production until they are added; that work is tracked separately. Built-in output reporting stays unchanged, classified by output-registry membership so literal-registered built-ins like `web-dashboard` never reach the `extensions` bucket.

## Decisions

### Decision: Reuse the existing live catalog fetch, not an embedded snapshot
Filter against the public catalog by reusing the fetch that `k6 x` already performs (`fetchCatalog`, today in `internal/cmd/x_registry.go`): the same registry URL, the same on-disk cache and TTL, and the same `K6_PROVISION_CATALOG_URL` override. That fetch already sits in package `cmd` (`x_registry.go`, called from `subcommand.go`), so the run path only needs to call it too; there is no cross-package move. The groundwork that lands first is self-contained: add `catalogModulePaths` and, optionally, rename the file to a neutral name so it reads as shared rather than `k6 x`-specific, with no behaviour change. The catalog top-level keys are import specifiers (`k6/x/sql`) and subcommand tokens (`subcommand:docs`), not module paths; each entry carries a `module` field with its Go module path. The existing `catalogEntry` decoder ignores `module`, so `catalogModulePaths` acquires the catalog through the same cache-then-fetch path `k6 x` uses (`readCachedCatalog`, else `fetchCatalog` against the `K6_PROVISION_CATALOG_URL`-or-default URL, then `writeCachedCatalog`), then re-parses the raw bytes into a new struct that reads each entry's `module` field and collects those values, and `filterExtensions` filters against that set. One helper serves both the run report and the subcommand report, so the acquisition is not duplicated. The live cached fetch needs no build-time generator, `go:embed`, or new make target, and makes a newly catalogued extension reportable as soon as the cache refreshes rather than only at the next k6 release. It costs a per-run network call, mitigated by the existing cache, and bounded and non-fatal because it rides the same bounded report send. All three surfaces filter through this one set: an output extension is dropped today because the catalog lists none, and starts reporting automatically once the catalog lists it.

### Decision: Record the resolved extension, filter at assembly
Record each used extension into its own `extensions` entry on the `usage.Usage` collector at the point of use: the resolution site for imports (`js/modules/resolution.go`) and the output-construction site for outputs (`internal/cmd/outputs.go`), leaving the existing `modules` and `outputs` lists untouched. What is recorded is the resolved `*ext.Extension` itself (the registry entry already in hand at the point of use), not its name. `usage.Usage` keeps a generic `Values(key, any)` method beside `Strings`/`Uint64` that appends an arbitrary value to a `[]any`; the recording sites call it, so `usage.Usage` stays type-agnostic and learns nothing about extensions. At report assembly, `resolveExtensions` reads that `[]any` bucket back out of the map, and `filterExtensions` keeps only the extensions whose module path is in the catalog, de-duplicates per (module, kind) so a module recorded more than once on a surface appears once, converts each survivor to a `{module, version, kind}` entry, and replaces the raw bucket with that list. Because de-duplication is per (module, kind), the same module used as both an import and an output yields two entries, one of each kind.

Recording the resolved extension rather than its name means assembly never re-derives anything from the registry: the module path, version, and kind are read straight off the recorded `*ext.Extension`. Because extension entries never enter `modules`/`outputs`, those lists stay untouched. Privacy stays in one place (the catalog filter) and off the hot path. Fail closed: the raw bucket is dropped before the catalog is consulted, so an unfetchable catalog reports no extensions, never the unfiltered list.

### Decision: One shared reporting mechanism, every surface binds to it
Both surfaces feed the single `usage.Usage` collector that already hangs off `GlobalState` (`cmd/state/state.go`), and one transport sends it. No surface builds its own sender.

The hard invariant: a single invocation hits the report URL at most once.

- Importable usage already flows through the `k6 run` report; it only needs recording. `js/modules/resolution.go` keeps the `modules` list exactly as today and records the resolved registry entry (`ext.Get(ext.JSExtension)[name]`, identified by registry membership rather than by the `k6/x/` name) into the dedicated `extensions` bucket instead of discarding it. `k6 run` sends its own report, exactly as today; this change does not touch that, so the run path needs no once-only handling.
- Output usage also flows through the `k6 run` report. `internal/cmd/outputs.go` records a selected `--out` into the `extensions` bucket when the output is an extension (membership in `ext.Get(ext.OutputExtension)`), and leaves the built-in `outputs` list and the literal-registered `web-dashboard` on their current path. No new send: it rides the run report like imports.
- Subcommands report from the one place that knows both which command ran and whether it succeeded: the root `execute` seam in `internal/cmd/root.go`. It runs the command with cobra's `ExecuteC`, which returns the executed leaf command and its error, then calls `reportSubcommandUsage` with that command. `reportSubcommandUsage` reports only when the executed command's name is a registered subcommand extension (`ext.Get(ext.SubcommandExtension)`), so `k6 run`, the `k6 x` help, and the provisioning stubs are skipped. The report fires whether the subcommand exited zero or non-zero, since telemetry counts the invocation regardless of outcome. A baked-in subcommand reports in-process. A provisioned subcommand reports from the child that actually runs it: the host never builds the command for an extension it lacks, so its `ExecuteC` never returns that command and it never reports, while the provisioned child reports exactly once. The launcher is not involved and needs no child suppression. The subcommand's own args and internals are out of scope; the name, version, and kind are enough. The report fires uniformly whether the command succeeded or failed and leaves the command's own run hook untouched.
- Opt-out: the `--no-usage-report` flag lives on the per-test `Config`, not `GlobalState`, so the subcommand send, which runs outside the run path, gates on the same option parsed from the environment with `readEnvConfig(gs.Env)` (honoring `K6_NO_USAGE_REPORT`), which is reachable everywhere and passes through to children.

Minimal factoring: `internal/cmd/report.go` holds one shared transport, `reportUsage(ctx, gs, create)`, which builds the report via the passed `create` closure, sends it bounded, and debug-logs the outcome; both surfaces call it. `createReport` builds the run report (execution stats plus the extensions field) and `createSubcommandReport` builds the small subcommand report (just the identity fields plus the extensions field); the identity fields (`k6_version`, `goos`, `goarch`, `is_ci`) are stamped by a shared `addEnvironmentInfo` so they live in one place. `k6 run` runs `reportUsage` in the background as today; the `k6 x` path calls it directly. One transport, two builders, no abstraction layer.

### Decision: Reuse relay and storage unchanged
The usage-stats backend stores the whole report JSON verbatim with no field allowlist (`is_ci` and `features` already ride the blob with no typed column). So `extensions` is stored and queryable (`JSON_EXTRACT_ARRAY(report, '$.extensions')`) with zero backend change. A typed column and a dbt staging column are optional later optimizations for cleaner dashboards.

### Decision: One new env for the report endpoint, none for the catalog
The report endpoint is a hardcoded `const` sent via `http.DefaultClient`, so a test cannot capture the report today. Make it overridable with one new env var, `K6_USAGE_REPORT_URL` (defined in `cmd/state/state.go` beside `K6_PROVISION_CATALOG_URL`), defaulting to the production endpoint. This is the only new env var. The catalog needs no new seam: tests steer membership through the existing `K6_PROVISION_CATALOG_URL`, pointed at a stand-in catalog. No `//go:linkname` and no package-level test hook.

### Decision: Keep the existing names, thread the new inputs through them
`createReport` and `reportUsage` keep their names, so the run path reads as it did. Their signatures do change, because the feature threads new inputs through them: `createReport` gains the catalog set it filters against, and `reportUsage` becomes the shared transport, taking a `create` closure so both surfaces build their own report and share one bounded send. The rest of the behaviour lands in new helpers, all in `internal/cmd/report.go` except the two catalog and root pieces: `addEnvironmentInfo` (the identity fields), `createSubcommandReport` (the subcommand's report), `resolveExtensions` and `filterExtensions` (read the recorded bucket, drop non-catalogued, de-duplicate, convert to entries), `postUsageReport` (marshal and POST), `catalogModulePaths` (the shared catalog set, in `x_registry.go`), and `reportSubcommandUsage` (the root `execute` seam that reports the invoked subcommand). The diff against master stays additive apart from the two threaded signatures and the recording sites.

## Data shape

```json
"extensions": [
  {"module": "github.com/grafana/xk6-sql", "version": "v0.4.0", "kind": "js"},
  {"module": "github.com/grafana/xk6-docs", "version": "v0.1.0", "kind": "subcommand"}
]
```

Field names align with the cloud `k6_dependencies` BigQuery column so cloud-executed and local-executed sources can be merged in analytics. This is a forward goal, not a join that exists today: that column covers cloud-executed runs only and is keyed by import name, while this field is keyed by module path.

## Risks / Trade-offs

- **Community sentiment on CLI telemetry**. Mitigated by public-only scope and the existing opt-out. No new data category beyond what already ships for built-in modules, and no new endpoint.
- **Per-run catalog fetch**: reusing the live fetch adds a network call, but it rides the existing catalog cache and TTL, runs inside the bounded report send, and is non-fatal: a fetch failure just reports no extensions.
- **Usefulness without unique-user counting**: real but orthogonal; this change still improves on the current zero-visibility baseline for local execution.

## Testing

Integration tests only, no unit tests, and table-driven: one test per command surface (`k6 run`, `k6 x`) iterating over a slice of cases, not a separate test function per behaviour. Every behaviour is verified by driving a real `k6 run` / `k6 x` through the command test harness (`internal/cmd/tests`, `NewGlobalTestState`) and asserting on the report actually sent. Follow the existing conventions in that package; do not change production code or comments beyond the one new env var below.

The report can't be observed in a test today: the endpoint is a hardcoded `const` sent via `http.DefaultClient`, and the harness disables reporting by default (`K6_NO_USAGE_REPORT=true`).

- Overridable report endpoint via the new `K6_USAGE_REPORT_URL` (`cmd/state/state.go`). A test enables reporting and points the endpoint at an `httptest.Server`, then asserts on the received JSON.
- Catalog membership via the existing `K6_PROVISION_CATALOG_URL` override (tests already point it at `http://127.0.0.1:1/unreachable`), aimed at a stand-in catalog whose entries carry `module` fields matching each test extension's resolved module path. Test extensions register through the normal `modules.Register` / `ext.Register` paths, so nothing is faked.

Two tables, as input/output vectors. Input is the stand-in catalog data, the script and flags, and the command; output is the asserted usage report. `M_import`, `M_output`, `M_sub`, and `M_fork` denote the module paths the test extensions resolve to.

`k6 run` surface (`TestRunReportsExtensions`):

| Catalog | Script / flags | Command | Expected report |
|---|---|---|---|
| any | trivial script | `k6 run` | the existing report posts to the stand-in endpoint (harness baseline, not a spec scenario; proves the `K6_USAGE_REPORT_URL` seam) |
| lists `M_import` | imports `k6/x/testimport` | `k6 run` | `extensions` is one `{M_import, version, js}` |
| lists `M_import` | imports `k6/x/testimport` | `k6 run --vus 5 --iterations 5` | `extensions` is exactly one `{M_import, _, js}` |
| lists `M_import` and `M_import2` (distinct Go packages) | imports both `k6/x/testimport` and `k6/x/testimport2` | `k6 run` | `extensions` is one `{M_import, _, js}` and one `{M_import2, _, js}` |
| lists `M_import` | imports nothing | `k6 run` | no `extensions` key (not `[]`) |
| omits `M_import` | imports `k6/x/testimport` | `k6 run` | no entry for it |
| lists real `M_import`; fork resolves to `M_fork` | imports the fork's `k6/x/testimport` | `k6 run` | no entry (matched by module path) |
| lists `M_output` | trivial script | `k6 run --out testoutput` | `extensions` is one `{M_output, _, output}` |
| omits `M_output` | trivial script | `k6 run --out testoutput` | no entry for it |
| any | trivial script | `k6 run --out json` | no `output`-kind entry; `outputs` still lists `json` |
| unreachable, no cache | imports `k6/x/testimport` | `k6 run` | no `extensions`; exit 0 |
| lists `M_import` | imports `k6/x/testimport` | `k6 run` with `K6_NO_USAGE_REPORT=true` | stand-in server receives no request |

`k6 x` surface (`TestSubcommandReportsUsage`):

| Catalog | Command | Expected report |
|---|---|---|
| lists `M_sub` | `k6 x testsub` | exactly one request, `{M_sub, _, subcommand}` |
| omits `M_sub` | `k6 x testsub` | no entry for it |
| unreachable | `k6 x not-a-catalog-name` | no request (the name cannot be provisioned) |
| lists `M_sub`, report URL unreachable or slow | `k6 x testsub` | exit 0, no error, a debug-level log, bounded to the run-report timeout |
| lists `M_sub` | `k6 x testsub` with `K6_NO_USAGE_REPORT=true` | no request |

Four cases are not integration-testable from a single binary and hold by construction, so they need no end-to-end row:
- Build-method independence: the reporting code reads the running binary, so xk6 and provisioning report identically.
- The exact `version` value: an in-tree test extension is absent from build deps, so `ext.Extension.Version` resolves to empty; tests assert `module` and `kind` and treat `version` as passed through verbatim.
- Provisioned-child once-only: provisioning runs a separately built binary the harness cannot produce; the baked-in once-only is tested end to end, and the provisioned child holds because the host never builds the command for an extension it lacks.
- Dual-purpose (import plus output) coexistence: an import resolves to its package path and an output to its constructor's function name, converging on one module path only for a real external dependency, so the in-tree harness cannot make a single module match both kinds. The (module, kind) de-duplication handles it by construction.

## Migration

Additive. No config migration. The only behavioural change is that `k6/x/*` usage, previously discarded, is now collected and (when catalogued) reported, covered by the existing opt-out.
