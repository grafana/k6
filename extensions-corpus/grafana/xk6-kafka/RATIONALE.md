# Rationale

Grafana-owned pure-Go Kafka extension for k6.

## Problem

k6 users who need pure-Go builds can no longer add Kafka load testing: the only available extension, the community `mostafa/xk6-kafka`, now requires a C toolchain (CGO) since its move to `confluentinc/librdkafka`. That breaks the pure-Go builds many teams depend on — no lightweight containers, harder cross-compilation, blocked CI/CD pipelines. Because the extension is community-owned, Grafana also cannot guarantee the support or security patches enterprises expect for critical infrastructure tooling.

## Significance

The teams blocked are exactly the ones Grafana most wants to serve: enterprises that run Kafka as core infrastructure and enforce strict pure-Go build and supply-chain rules. Today they cannot test Kafka in k6 without taking on a CGO dependency their pipelines forbid, so they either drop Kafka coverage or test it outside k6. The same dependency keeps Kafka testing out of Grafana Cloud k6 (GCK6), which is built on a pure-Go model and so cannot offer it as a first-class capability. The result is no official, supported, pure-Go Kafka option for the segment most likely to ask for one.

## Desired state

A new Grafana-owned `grafana/xk6-kafka` extension that:

- Compiles as **100% pure Go** (`CGO_ENABLED=0`) — no C toolchain, scratch-container friendly.
- Is **officially supported** by Grafana with a clear ownership and patching path.
- Is built on a modern, actively maintained pure-Go Kafka client.
- Is a **near-drop-in replacement for `mostafa/xk6-kafka` v1 scripts**: same import and API shape, so common producer, consumer, admin, auth, and Schema Registry scripts run unchanged.
- **Prioritizes pure-Go portability over raw throughput**: performs no worse than the historical v1.x baseline, and does not aim to match the CGO-based `mostafa/xk6-kafka` v2 (`confluentinc/librdkafka`) throughput.

Two compatibility boundaries are intentional and will be documented, not engineered away:

1. **Runs unchanged ≠ behaves identically.** Some legacy tuning options have no equivalent and are accepted but ignored, so behavior can differ in edge cases.
2. **`mostafa/xk6-kafka` v2 is a future, additive target — not day one.** The v2 API can be added later, layered on the same extension. Even then, v2 users get working scripts at the v1 performance baseline, not Confluent-level throughput.

The goal is familiarity, not a compatibility guarantee: most v1 scripts run unchanged, but identical behavior across every script is not promised. Users who need behavior-identical, zero-change continuity should stay on `mostafa/xk6-kafka`. Migration is supported through documentation — per-workflow guidance and explicit notes on unsupported or changed behavior.

## Cost of inaction

Pure-Go and enterprise k6 users keep hitting a wall: they either accept a CGO dependency many of their pipelines forbid, or they abandon Kafka testing in k6 entirely. Every quarter without an official pure-Go option keeps Kafka out of Grafana Cloud k6 and pushes Kafka-heavy enterprises toward other tools, eroding k6's credibility in exactly the accounts that most want official support.
