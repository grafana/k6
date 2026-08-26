## Why

The repository currently has only the API contract (`index.d.ts`), docs, and
license — no Go code, so nothing builds. Issue #1 needs a foundation: a pure-Go
k6 extension that registers the `k6/x/kafka` module, exports the exact public
symbols the contract declares, and builds and lints in CI. Everything else
(producer, consumer, admin, Schema Registry) layers onto this scaffold.

## What Changes

- Add the Go module (`go.mod`, `CGO_ENABLED=0`) and a `pkg/kafka` package.
- Register the k6 module `k6/x/kafka` via `modules.Register`, exposing a default
  export object.
- Export every grouped value from `index.d.ts` as an individual top-level
  constant with the exact value (compression codecs, SASL mechanisms, TLS
  versions, balancers, group balancers, schema types, element types, subject
  name strategies, isolation levels, start offsets incl. `FIRST_OFFSET` /
  `LAST_OFFSET` aliases, and `TIME` units). No enum objects.
- Register the public symbols so scripts load: constructors `Writer`, `Reader`,
  `Connection`, `SchemaRegistry`, and the `LoadJKS` function. Method bodies are
  out of scope here — they arrive in later changes — but the symbols must exist
  and construct without error.
- Add CI as a thin caller workflow created from the `grafana/k6-ci` template
  (`templates/k6-ci.yml`) that invokes the reusable shared workflow
  (`grafana/k6-ci/.github/workflows/all.yml`), pinned to a commit SHA with a
  matching `k6-ci-ref`. The shared workflow runs lint, the multi-version Go
  tests, and `xk6` extension build/lint/test; the repo configures it via inputs
  (`skip-tests: false`, `skip-extension-testing: false`) rather than redefining
  steps.
- Add the `grafana/k6-ci` template `Makefile` (`lint` / `update-lint-patch` /
  `clean-lint`, which single-source the pinned ref from the workflow and produce
  the local `.golangci.yml`), plus convenience targets `build`, `test`, `it`, and
  a default target that prints usage.

## Capabilities

### New Capabilities

- `kafka-module`: the `k6/x/kafka` module registration, its exported constant
  surface (names and values matching `index.d.ts`), and the presence of the
  top-level `Writer` / `Reader` / `Connection` / `SchemaRegistry` / `LoadJKS`
  symbols.
- `build-and-ci`: pure-Go buildability via `xk6 build` (`CGO_ENABLED=0`) and the
  CI baseline — a thin caller workflow from the `grafana/k6-ci` template that
  invokes the pinned shared workflow, which runs lint, the multi-version Go
  tests, and `xk6` extension build/lint/test — plus the developer `Makefile`
  (k6-ci lint targets + `build`/`test`/`it`/usage).

### Modified Capabilities

<!-- None — this is the first change; no existing specs. -->

## Impact

- New files: `go.mod`/`go.sum`, `pkg/kafka/` (module registration + constants),
  a CI caller workflow under `.github/workflows/` from `grafana/k6-ci`
  `templates/k6-ci.yml`, and the `Makefile` (k6-ci lint targets + `build`/`test`/
  `it`/usage). The lint config is not hand-authored: `.golangci-base.yml` and the
  effective `.golangci.yml` are produced from the pinned k6-ci config and
  gitignored.
- New dependency: `go.k6.io/k6` (module/extension host). `twmb/franz-go` is not
  required by the scaffold and is introduced when the producer/consumer changes
  land.
- `index.d.ts` is unchanged; this change must conform to it, not alter it.
