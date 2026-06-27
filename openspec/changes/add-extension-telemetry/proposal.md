## Why

k6 collects an anonymous usage report but **deliberately excludes extensions**: `js/modules/resolution.go` skips every `k6/x/*` import. The original reason was privacy: reporting extension names would leak private extensions, which the community disliked.

Now that binary provisioning is GA and there is a public catalog of official and community extensions, we have no visibility into which extensions are actually used. We cannot tell which community extensions deserve promotion to official, where to spend maintenance effort, or how binary provisioning is being adopted. Cloud-executed runs are partly tracked via a BigQuery `k6_dependencies` column, but **local execution, most OSS usage, is invisible**.

This change reports extension usage through the existing usage report, scoped to the **public catalog only** so private extensions are never reported.

## What Changes

- Add an `extensions` field to the usage report listing the extensions actually used in a run, filtered to the public catalog.
- Cover the importable JS (`k6/x/*`), output (`--out`), and subcommand (`k6 x <name>`) surfaces. The output path ships now but reports nothing in production until the public catalog lists output extensions (tracked separately).
- Filter against the **public registry catalog**, reusing the catalog fetch that `k6 x` already performs (same cache, TTL, and override); no embedded snapshot and no build-time generator, and private/unlisted extensions are dropped.
- Report identically whether the binary was built by `xk6` or produced by binary provisioning (the running binary reports on itself).
- Reuse the existing relay, storage, and opt-out (`--no-usage-report` / `K6_NO_USAGE_REPORT`).

## Capabilities

### New Capabilities
- `extension-telemetry`: report public-catalog extension usage (importable, output, subcommand) through the existing usage report, filtered against the public registry catalog (reusing the existing cached fetch), respecting the existing opt-out.

### Modified Capabilities
<!-- none; additive, rides the existing usage report -->

## Impact

- **grafana/k6**, the only code change:
  - `js/modules/resolution.go`: keep the `modules` list unchanged; record the resolved registry entry for Go extension imports (identified via the extension registry, not by the `k6/x/` name) into a dedicated `extensions` usage bucket instead of discarding them.
  - `internal/usage/usage.go`: add a generic `Values(key, any)` method beside `Strings`/`Uint64`, so callers can record a resolved extension entry without `usage.Usage` knowing about extensions.
  - `internal/cmd/outputs.go`: record a selected `--out` into the `extensions` bucket when it is an output extension (membership in `ext.Get(ext.OutputExtension)`). Built-in output recording stays exactly as today, so built-ins and the literal-registered `web-dashboard` never reach the bucket.
  - `internal/cmd/root.go` and `internal/cmd/subcommand.go`: report subcommand usage once, from the root `execute` seam. `root.go` runs the command with cobra's `ExecuteC` (which returns the executed command and its error) and calls `reportSubcommandUsage`, which reports whenever the executed command is a registered subcommand extension, whether it succeeded or failed, through the shared mechanism, not a bespoke one. In-process when baked in, from the provisioned child otherwise. The launcher is not touched: the host never builds the command for an extension it lacks, so only the running command reports.
  - `internal/cmd/report.go`: keep `createReport` and `reportUsage` (the shared transport now takes a `create` closure); assemble the `extensions` field by reading the recorded `*ext.Extension` entries back out of the bucket and filtering them by module path against the catalog fetched via the existing `internal/cmd/x_registry.go` fetch. No re-resolution: module path, version, and kind come straight off the recorded entry.
  - `cmd/state/state.go`: add one env var, `K6_USAGE_REPORT_URL`, so the report endpoint is overridable for integration tests; the catalog reuses the existing `K6_PROVISION_CATALOG_URL`. No embedded file and no new generator.
  - `release notes/`: add the entry documenting the new `extensions` report field.
- **Backend / analytics**: no change required to store or query. The usage-stats service stores the whole report JSON as a blob with no field allowlist (`is_ci` and `features` already ride it), so `extensions` is queryable via JSON extraction. A typed column or a dbt column are optional later optimizations; a dashboard panel is the only step needed to visualize.
- **grafana/k6-docs**: the [usage collection page](https://grafana.com/docs/k6/latest/set-up/usage-collection/) states "Only k6 built-in JavaScript modules and outputs are considered. Private modules and custom extensions are excluded." This change makes that sentence false: update the page to document the `extensions` field, its public-catalog-only scope, and that the existing opt-out covers it.
- **Scope**: single PR. The importable, output, and subcommand surfaces are in scope. The output path ships now; it reports nothing until output extensions are added to the public catalog, which is tracked separately.
- **Out of scope**: counting unique users or installations ([telemetry issue](https://github.com/grafana/k6/issues/4077)); distinguishing xk6-built from provisioning-built binaries ([grafana/k6#6111](https://github.com/grafana/k6/issues/6111)); reporting unlisted/private extensions; secret-source extensions; a delisted-but-buried `k6/x/*` extension still used on a hand-built binary, which the catalog filter drops (a shrinking cohort, since the modern path is already counted as a built-in once the extension is migrated into core).

## References

- [grafana/k6#6109](https://github.com/grafana/k6/issues/6109), the feature issue (assignee: inancgumus).
- [grafana/k6-cloud#3575](https://github.com/grafana/k6-cloud/issues/3575), original tracking issue.
- [grafana/k6#2952](https://github.com/grafana/k6/issues/2952), request to add extension versions to the usage report.
- Existing report mechanism PR [grafana/k6#3917](https://github.com/grafana/k6/pull/3917); current exclusion at `js/modules/resolution.go:81-87`.
