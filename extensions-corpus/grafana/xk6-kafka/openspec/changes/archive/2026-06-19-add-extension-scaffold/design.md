## Context

The repo holds the frozen-enough API contract (`index.d.ts`), docs, and license,
but no Go code. This change adds the buildable skeleton every later capability
depends on: the Go module, the `k6/x/kafka` module registration, the exported
constant surface, the public symbols, and a CI baseline. It must conform to
`index.d.ts` exactly and stay pure Go.

## Goals / Non-Goals

**Goals:**

- A `k6` binary builds via `xk6 build` with `CGO_ENABLED=0`.
- `import … from "k6/x/kafka"` resolves; all flat constants and the
  `Writer`/`Reader`/`Connection`/`SchemaRegistry`/`LoadJKS` symbols exist.
- CI runs from the `grafana/k6-ci` template — a thin caller that invokes the
  pinned shared workflow, which runs lint, the multi-version Go tests, and `xk6`
  extension build/lint/test — plus a developer `Makefile`.

**Non-Goals:**

- Method behavior (produce/consume/admin/serdes) — later changes.
- Any `twmb/franz-go` integration — not needed to register the module.
- Metrics, examples, and the migration guide.

## Decisions

- **Module path & package.** Register `k6/x/kafka` via
  `modules.Register("k6/x/kafka", new(RootModule))` (the standard k6 extension
  pattern), with implementation under `pkg/kafka`. Rationale: matches k6
  extension conventions and `xk6` expectations; keeps the JS-facing surface in
  one package.
- **Constants as a single source.** Define the constant values once in Go and
  attach them to the module's default export so the runtime names/values match
  `index.d.ts`. A test asserts parity against the contract values. Rationale:
  `index.d.ts` is authoritative; drift is the most likely scaffold bug, so it is
  guarded by a test. Alternative considered: generating the constants from
  `index.d.ts` — rejected for now as over-engineering; revisit if drift recurs.
- **Symbols now, behavior later.** Constructors are registered and construct an
  empty/holding instance; methods land in subsequent changes. Rationale: lets
  scripts (and CI smoke) load the module before features exist, and keeps each
  later change small and reviewable.
- **CI via the shared k6-ci workflow, not a fork.** Use the `grafana/k6-ci`
  template (`templates/k6-ci.yml`): a thin caller that invokes the reusable
  shared workflow `all.yml`, pinned to a SHA with a matching `k6-ci-ref`, and
  enable Go tests + extension testing via inputs. Rationale: the shared workflow
  already runs lint, multi-version Go tests, and `xk6` extension build/lint/test;
  reusing it (with Renovate SHA bumps) avoids drift and maintenance. Alternative
  considered: forking a local copy of `all.yml` — rejected for the scaffold; any
  improvements should go upstream to `grafana/k6-ci` so all extensions benefit.
- **Makefile = k6-ci template + convenience targets.** Adopt the `grafana/k6-ci`
  template `Makefile` (`lint` / `update-lint-patch` / `clean-lint`), which reads
  the pinned ref from the workflow's `uses:` line (single source, never
  duplicated) and produces the effective `.golangci.yml` locally so local lint
  matches CI. Add `build` / `test` / `it` and a default usage target on top.
  Rationale: standard lint mechanism kept intact; convenience targets give one
  obvious way to build/test without touching the lint wiring.

## Risks / Trade-offs

- **Constant drift from `index.d.ts`** → contract-parity test over names and
  values; CI fails on mismatch.
- **Stub constructors mask missing behavior** → scope is explicit in the spec
  (symbol presence only); later changes add behavior and their own scenarios.
- **`grafana/k6-ci` template assumptions may not fit a fresh repo** → if the
  template needs adjustment, capture the delta rather than forking the pipeline.

## Open Questions

- Exact `pkg/kafka` file layout (single `module.go` vs split per capability) —
  defer to implementation; not contract-visible.
- Minimum Go and k6 versions to pin — align with what `xk6 sync` / `grafana/k6-ci`
  expect.
