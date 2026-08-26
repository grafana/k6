## Context

The scaffold (#3) registers symbols but does not connect. This change adds the
shared connection plumbing on `twmb/franz-go`: SASL, TLS, JKS loading, a client
builder, and a working `Connection`. It must conform to `index.d.ts` and stay
pure Go.

## Goals / Non-Goals

**Goals:**

- `LoadJKS` returns real PEM material from a JKS keystore.
- A single client builder maps brokers + `SASLConfig` + `TLSConfig` to franz-go
  options, unit-tested for mechanism/TLS selection.
- `Connection` connects on construction and closes cleanly.

**Non-Goals:**

- Producer/consumer/admin method behavior and Schema Registry serdes.
- `SASL_AZURE_ENTRA` and other v2 surface.

## Decisions

- **franz-go SASL packages.** Use `pkg/sasl/plain` and `pkg/sasl/scram`
  (Sha256/Sha512), selected by a switch on `algorithm`. `sasl_aws_iam` is
  deferred (it needs an AWS credential provider and pulls in the AWS SDK); it
  returns a "not yet implemented" error here and lands in its own change.
  Rationale: keep this change's dependency footprint small.
- **`sasl_ssl` mapping.** Treat `sasl_ssl` as SASL/PLAIN credentials carried over
  a TLS connection (TLS is enabled separately via `TLSConfig`), matching v1
  intent where "SSL" meant transport security rather than a distinct mechanism.
  Documented as a compatibility mapping.
- **TLS from PEM.** Build `*tls.Config` directly from the PEM fields
  (`tls.X509KeyPair` for the client cert/key, a cert pool for the server CA),
  set `MinVersion` from `minVersion`, and `InsecureSkipVerify` from
  `insecureSkipTlsVerify`. Rationale: no files needed; PEM is what the contract
  and `LoadJKS` provide.
- **JKS via a pure-Go keystore library.** Parse the JKS with a pure-Go keystore
  reader and re-encode entries as PEM. Reject PKCS#12 explicitly. Rationale:
  keeps `CGO_ENABLED=0`; PKCS#12 is out of scope per the contract.
- **Eager connect.** `Connection` builds the client and verifies connectivity
  (a franz-go ping/metadata request) during construction, so an unreachable
  broker or bad credentials fail fast — matching the spec.
- **Integration tests run in CI against a Kafka service.** The shared
  `grafana/k6-ci` workflow has no Kafka, so add a dedicated integration job (a
  separate workflow) that starts a Kafka service container and runs the
  broker-backed tests (`make it` / `xk6 test`) against it. This job proves the
  real **connection path** (connect/close) against a broker; SASL mechanism
  selection, TLS config building, and JKS loading are validated by unit tests.
  An authenticated/TLS-backed integration case (a SASL- or TLS-configured
  broker) is a desirable follow-up but out of scope here to keep the broker
  setup simple. The tests skip only when no broker address is configured
  (local / dev); in CI a broker address is set, so an unreachable service fails
  the job rather than skipping. Rationale:
  the networked connection path must not merge unverified; complementing (not
  forking) the shared workflow keeps the lint/unit gate intact while adding the
  broker coverage it can't provide.

## Risks / Trade-offs

- **`sasl_ssl` semantics differ across tools** → document the PLAIN-over-TLS
  mapping; revisit if users report a mismatch.
- **JKS library choice** → pick a maintained pure-Go keystore lib; if it pulls
  cgo or is unmaintained, swap it. Guard with `CGO_ENABLED=0` in CI.
- **Eager connect adds construction latency / failure modes** → acceptable and
  matches the contract (construction fails on unreachable cluster).
- **CI Kafka service flakiness / startup time** → use a health-gated service
  container and keep the integration job separate so a broker hiccup doesn't
  mask lint/unit results.

## Open Questions

- Exact franz-go connectivity check for `Connection` (Ping vs a metadata
  request) — settle in implementation; not contract-visible.
- AWS IAM credential source (profile / env / role) — settled in the dedicated
  `SASL_AWS_IAM` change, not here.
