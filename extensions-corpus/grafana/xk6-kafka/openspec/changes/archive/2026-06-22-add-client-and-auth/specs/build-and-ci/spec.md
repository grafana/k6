## MODIFIED Requirements

### Requirement: CI via the shared k6-ci workflow, pinned

The repository SHALL run CI through a thin caller workflow created from the
`grafana/k6-ci` template (`templates/k6-ci.yml`) that invokes the reusable shared
workflow `grafana/k6-ci/.github/workflows/all.yml`. The caller SHALL pin the
`uses:` reference to a specific commit hash (not a moving branch) and pass that
same hash as the `k6-ci-ref` input so the lint config it downloads matches. It
SHALL enable the shared workflow's Go tests and extension testing
(`skip-tests: false`, `skip-extension-testing: false`), which is how the unit
tests, lint, and `xk6` extension build/lint/test run. The repository SHALL NOT
re-implement those steps that the shared workflow provides (lint, unit tests,
and extension build/lint/test) — they are owned by the shared workflow and
configured here only through inputs. Capabilities the shared workflow does not
provide, such as broker-backed integration testing, MAY be added as separate
workflows.

#### Scenario: Caller invokes the pinned shared workflow

- **WHEN** the CI workflow under `.github/workflows/` is inspected
- **THEN** it calls `grafana/k6-ci/.github/workflows/all.yml` pinned to a commit
  hash, and the `k6-ci-ref` input is set to the same hash

#### Scenario: Tests and extension testing are enabled

- **WHEN** the caller workflow inputs are inspected
- **THEN** `skip-tests` and `skip-extension-testing` are both false, so the
  shared workflow runs lint, the multi-version Go tests, and `xk6` extension
  build/lint/test

## ADDED Requirements

### Requirement: Integration tests run in CI against a Kafka service

The repository SHALL run the broker-backed integration tests in CI against a
Kafka service (e.g. a service container), in a separate workflow from the shared
`grafana/k6-ci` one (which provides lint, unit tests, and extension
build/lint/test but no Kafka). This job SHALL exercise the real connection path
(connect and close) against a running broker; the SASL mechanism selection, TLS
config building, and JKS loading are covered by unit tests rather than this job.
The integration tests SHALL skip only when no broker is **configured** (local /
dev runs without a broker address). The CI job SHALL configure a broker address
pointing at the Kafka service it starts, so a service it cannot reach causes a
test **failure**, not a skip — the gate must not pass on broker startup or
connectivity problems.

#### Scenario: CI exercises the connection path

- **WHEN** a pull request is opened
- **THEN** a CI job starts a Kafka service, configures the broker address, and
  runs the integration tests (`make it` / `xk6 test`) — connecting to and
  closing against the broker — failing the gate if they fail

#### Scenario: CI fails when its broker is unreachable

- **WHEN** the CI job has configured a broker address but the Kafka service is unreachable
- **THEN** the integration tests fail (the job does not skip or pass)

#### Scenario: Integration tests skip locally when no broker is configured

- **WHEN** the integration tests run with no broker address configured (local / dev)
- **THEN** they skip rather than fail
