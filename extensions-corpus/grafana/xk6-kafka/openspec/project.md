# Project: grafana/xk6-kafka

## Purpose

The official, Grafana-owned, pure-Go k6 extension for load testing Apache Kafka:
producing and consuming messages, managing topics, authenticating, and working
with Schema Registry. Imported in k6 scripts as `k6/x/kafka`.

It exists to give pure-Go / enterprise k6 users a Kafka option that needs no C
toolchain, and that Grafana can officially support and offer in Grafana Cloud
k6. See `README.md` and `RATIONALE.md` for the full problem statement and scope,
and GitHub issue #1 for the implementation epic.

## Tech stack

- **Language:** Go (`CGO_ENABLED=0`, 100% pure Go — no `confluentinc/librdkafka`).
- **Kafka client:** `twmb/franz-go` — `kgo` (client, produce, consume), `kadm`
  (topic admin), and the franz-go SASL/TLS packages for auth.
- **Extension host:** k6 via `xk6` (`go.k6.io/k6`), registered as module
  `k6/x/kafka`.

## Source of truth

- **`index.d.ts`** is the authoritative API contract: class/method/constant
  names, config field names, types, optionality, and accept-but-ignore
  behavior. The grouped values (codecs, SASL mechanisms, schema types, etc.) are
  individual top-level constants, not enum objects. Any surface change goes
  through `index.d.ts` first; do not improvise API shape during implementation.
  The full declared surface is delivered **incrementally across changes**: each
  change must conform to the contract for the parts it implements (and never
  deviate), but is not required to implement the entire surface at once. Exact,
  complete conformance to `index.d.ts` is the cumulative end state, not a gate on
  every single change.
- The API targets familiarity with community `mostafa/xk6-kafka` **v1** scripts.
  The community **v2** surface (`Producer`/`Consumer`/`AdminClient`,
  `SASL_AZURE_ENTRA`, protobuf metadata fields) is out of scope for now.

## Conventions

- **Public names mirror the v1 API:** `Writer`, `Reader`, `Connection`,
  `SchemaRegistry`, `LoadJKS`, and the flat constants.
- **Familiarity, not a guarantee.** Legacy tuning options with no franz-go
  equivalent are accepted but ignored (never error). The current
  accepted-but-ignored / approximate set is annotated as `@remarks` in
  `index.d.ts` — keep code and that file in sync.
- **Message shape:** produce accepts `string | Uint8Array` keys/values, plain
  object headers, and `Date` time; consume returns `Uint8Array` key/value, plain
  object headers, and an RFC3339 string time.
- **Go style:** standard `gofmt`. Lint with `golangci-lint` using the shared
  configuration from `grafana/k6-ci` (do not maintain a bespoke config), plus
  `xk6 lint` for k6-extension compliance.

## Tooling (xk6)

- **Build:** `xk6 build --with github.com/grafana/xk6-kafka`.
- **Run scripts:** `xk6 run script.js` (exercise the extension with a k6 script).
- **Integration tests:** `xk6 test` (run integration tests with the custom k6).
- **Lint/compliance:** `xk6 lint`.
- **Dependency alignment:** `xk6 sync` (keep dependencies in step with k6).

## Testing

- **Unit tests** for config mapping, serdes, and helpers (no broker needed).
- **Integration tests** via `xk6 test` against a Kafka container for the
  required compatibility workflows (see issue #1): basic produce/consume
  (string, bytes, JSON), topic admin, consumer group, SASL (PLAIN, SCRAM), TLS
  incl. JKS, and Schema Registry serdes (Avro, JSON).
- **Compatibility fixtures** live under `test/integration/compat/` and are
  modernized ports of the community v1 scripts (broker address and Schema
  Registry URL from env, e.g. `KAFKA_BROKER`), keeping the original
  `check()` assertions. Most run essentially unchanged (env wiring only) as
  behavior-parity evidence; where the community script relies on behavior this
  extension intentionally diverges from, the port is a **migrated** fixture that
  documents the migration instead (e.g. JSON serdes require a schema here, so
  `compat/json.js` supplies one). `test/integration/compat/README.md` classifies
  each fixture and records such divergences. They are added incrementally: each
  capability change ports the community script(s) it makes runnable.
- CI must run `golangci-lint`, `xk6 lint`, unit tests, `xk6 test`, and an
  `xk6 build` smoke test.

## Constraints

- Must stay pure Go (`CGO_ENABLED=0`); no dependency may pull in cgo.
- Performance target: no worse than the historical pure-Go v1.x baseline; not
  expected to match the librdkafka-based v2 throughput.
- Migration is documentation-driven; zero-script-change continuity is a non-goal.
