## Why

The scaffold registers the module and its symbols but does not connect to Kafka.
Producer, consumer, and admin all need the same foundation: an authenticated,
optionally TLS-secured client. Building that plumbing once — SASL, TLS, JKS
loading, and a shared client builder — and proving it via `Connection`
de-risks every capability that follows.

## What Changes

- Implement `LoadJKS`: load a JKS keystore (read via k6's init-context
  filesystem, archive-aware) and return PEM material (`clientCertsPem`,
  `clientKeyPem`, `serverCaPem`). Pure Go; PKCS#12 is rejected.
- Map `SASLConfig.algorithm` to a `twmb/franz-go` SASL mechanism: `none`,
  `sasl_plain`, `sasl_scram_sha256`, `sasl_scram_sha512`. `sasl_ssl` uses PLAIN
  and requires TLS to be enabled (matching v1). `sasl_aws_iam` is deferred to a
  dedicated change (it needs an AWS credential provider and the AWS SDK) and
  returns a "not yet implemented" error for now.
- Run the broker-backed integration tests in CI against a Kafka service, in
  addition to the shared `grafana/k6-ci` workflow.
- Map `TLSConfig` to a `*tls.Config`: `enableTls`, `insecureSkipTlsVerify`,
  `minVersion`, client cert/key PEM (mutual TLS), and server CA PEM.
- Add a shared client builder that turns brokers + `SASLConfig` + `TLSConfig`
  into franz-go client options, reused by all classes.
- Make `Connection` establish a real connection on construction (using the
  builder) and release it on `close()`. Topic admin methods stay out of scope.

## Capabilities

### New Capabilities

- `auth`: SASL mechanism selection, TLS configuration, JKS loading (`LoadJKS`),
  and the shared authenticated client builder.
- `connection`: the `Connection` class establishing and closing an
  authenticated connection to the cluster.

### Modified Capabilities

- `build-and-ci`: adds a requirement that the broker-backed integration tests
  run in CI against a Kafka service (the shared workflow has no broker).

<!-- `kafka-module` is unchanged: the symbols exist and construct. New behavior
lives in the new `auth` / `connection` capabilities. -->

## Impact

- New code in `pkg/kafka` (auth/TLS mapping, JKS, client builder, `Connection`
  wiring) plus unit tests. New dependency: `twmb/franz-go` (and its SASL
  packages), and a pure-Go JKS keystore library.
- A new integration CI workflow under `.github/workflows/` that starts a Kafka
  service container and runs the integration tests; integration tests skip only
  when no broker address is configured (local/dev) and fail in CI if the
  configured service is unreachable.
- `index.d.ts`: doc remarks are clarified to match this behavior — `SASL_SSL`
  (PLAIN credentials, requires TLS) and `LoadJKS` (init-context). No type
  changes. Otherwise this change implements part of the contract it already
  declares.
