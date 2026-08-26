## ADDED Requirements

### Requirement: Pure-Go build via xk6

The extension SHALL build into a `k6` binary using
`xk6 build --with github.com/grafana/xk6-kafka` with `CGO_ENABLED=0`. No
dependency may require cgo or a C toolchain.

#### Scenario: Building with CGO disabled

- **WHEN** `xk6 build --with github.com/grafana/xk6-kafka` runs with
  `CGO_ENABLED=0`
- **THEN** a working `k6` binary is produced with no C toolchain present

### Requirement: Lint passes

The codebase SHALL pass `golangci-lint` using the shared configuration from
`grafana/k6-ci` (not a bespoke config) and SHALL pass `xk6 lint` for k6
extension compliance.

#### Scenario: golangci-lint is clean

- **WHEN** `golangci-lint` runs with the `grafana/k6-ci` configuration
- **THEN** it reports no errors

#### Scenario: xk6 lint is clean

- **WHEN** `xk6 lint` runs against the extension
- **THEN** it reports no compliance errors

### Requirement: CI via the shared k6-ci workflow, pinned

The repository SHALL run CI through a thin caller workflow created from the
`grafana/k6-ci` template (`templates/k6-ci.yml`) that invokes the reusable shared
workflow `grafana/k6-ci/.github/workflows/all.yml`. The caller SHALL pin the
`uses:` reference to a specific commit hash (not a moving branch) and pass that
same hash as the `k6-ci-ref` input so the lint config it downloads matches. It
SHALL enable the shared workflow's Go tests and extension testing
(`skip-tests: false`, `skip-extension-testing: false`), which is how the unit
tests, lint, and `xk6` extension build/lint/test run. The repository SHALL NOT
re-implement those individual steps — they are owned by the shared workflow and
configured here only through inputs.

#### Scenario: Caller invokes the pinned shared workflow

- **WHEN** the CI workflow under `.github/workflows/` is inspected
- **THEN** it calls `grafana/k6-ci/.github/workflows/all.yml` pinned to a commit
  hash, and the `k6-ci-ref` input is set to the same hash

#### Scenario: Tests and extension testing are enabled

- **WHEN** the caller workflow inputs are inspected
- **THEN** `skip-tests` and `skip-extension-testing` are both false, so the
  shared workflow runs lint, the multi-version Go tests, and `xk6` extension
  build/lint/test

### Requirement: Developer task runner

The repository SHALL include the `grafana/k6-ci` template `Makefile`, providing
the lint targets `lint`, `update-lint-patch`, and `clean-lint`. These derive the
pinned `grafana/k6-ci` ref from the CI workflow (single source — read from the
`uses:` line, never duplicated) and produce the effective local `.golangci.yml`
so local lint matches CI. The generated `.golangci-base.yml` and the effective
`.golangci.yml` SHALL be gitignored — they are produced from the pinned config,
not committed. The repository SHALL also add convenience targets: `build`
(`xk6 build`), `test` (unit tests), `it` (integration via `xk6 test`), and a
default target that prints usage. The convenience targets SHALL NOT alter the
k6-ci lint mechanism or duplicate the pinned ref.

#### Scenario: k6-ci lint targets are present and single-sourced

- **WHEN** the `Makefile` is inspected
- **THEN** it provides `lint`, `update-lint-patch`, and `clean-lint`, and derives
  the `grafana/k6-ci` ref from the CI workflow rather than a duplicated literal

#### Scenario: Convenience targets run the expected commands

- **WHEN** the convenience targets are invoked
- **THEN** `build` runs `xk6 build`, `test` runs the unit tests (`go test`), and
  `it` runs the integration tests via `xk6 test`

#### Scenario: Generated lint files are gitignored

- **WHEN** the repository ignore rules are inspected
- **THEN** `.golangci-base.yml` and `.golangci.yml` are gitignored

#### Scenario: Default target prints usage

- **WHEN** a developer runs `make` with no target
- **THEN** it prints usage information listing the available targets
