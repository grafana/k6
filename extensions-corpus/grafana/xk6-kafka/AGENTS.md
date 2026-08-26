# AGENTS.md

Guidance for AI coding agents working in this repo. Humans: see
[README.md](README.md) and [RATIONALE.md](RATIONALE.md).

## What this is

`grafana/xk6-kafka` — the official, Grafana-owned, **pure-Go** k6 extension for
load testing Apache Kafka. Imported in k6 scripts as `k6/x/kafka`. Targets
familiarity with the community `mostafa/xk6-kafka` **v1** API.

## Commands

```bash
make build         # xk6 build a k6 binary with this extension
make test          # unit tests, no broker needed
make integration   # start a Kafka container, run integration tests, tear down
make lint          # golangci-lint with the pinned grafana/k6-ci config
xk6 lint           # k6-extension compliance
```

Integration against an already-running broker:

```bash
make broker-up
KAFKA_BROKER=localhost:9092 KAFKA_SASL_BROKER=localhost:9094 make it
make broker-down
```

## Non-negotiables

- **Pure Go, `CGO_ENABLED=0`.** No dependency may pull in cgo (no
  `confluentinc/librdkafka`). Kafka client is `twmb/franz-go`.
- **`index.d.ts` is the API source of truth** — class/method/constant/config
  names, types, optionality, accept-but-ignore behavior. Change it *first*; do
  not improvise API shape in Go.
- **Config structs bind from JS via `js:` struct tags, not `json:`.** The k6
  field-name mapper ignores `json:` tags. A camelCase config field (`groupID`,
  `autoCreateTopic`, TLS fields, …) that uses `json:` silently never binds.
- **Familiarity, not a guarantee.** Legacy v1 tuning options with no franz-go
  equivalent are accepted but ignored — never error. Keep such options
  annotated as `@remarks` in `index.d.ts`.
- Community **v2** surface (`Producer`/`Consumer`/`AdminClient`,
  `SASL_AZURE_ENTRA`, protobuf) is out of scope.

## Change workflow (OpenSpec)

Spec-driven changes use the `openspec` CLI: **propose → apply → archive**.
Proposals live under `openspec/changes/<name>/`; the `.claude/` skills and
commands drive the flow. See [openspec/project.md](openspec/project.md) for
full project context and conventions.

## Releases & commits

- Releases are **GitHub Releases** with auto-generated notes (grouped by PR
  label). There is **no `CHANGELOG` file** — do not add one.
- Tags follow semver `vX.Y.Z`; a `-` suffix marks a prerelease.
- **Do not add a `Co-Authored-By` trailer** to commits or PRs.
- Run `make lint` and `make test` before pushing.
