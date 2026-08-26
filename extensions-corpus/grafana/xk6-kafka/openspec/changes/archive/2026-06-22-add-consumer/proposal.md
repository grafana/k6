## Why

`Reader` currently only constructs (scaffold). Consuming messages is the other
half of load testing Kafka and, together with the producer (#13), enables the
full produce → consume round-trip. It also unlocks porting the common community
v1 scripts as compatibility fixtures.

## What Changes

- `new Reader(ReaderConfig)` builds a `twmb/franz-go` consumer client via the
  shared client builder (brokers, SASL, TLS) in one of two modes:
  - **consumer group:** `groupID` with `groupTopics` (or `topic`), using
    `groupBalancers`, heartbeat/session/rebalance timeouts, and commit interval;
  - **direct:** a single partition of `topic` (`partition` defaults to `0`;
    multiple partitions need a group), starting at `offset` or `startOffset`.
  It also maps `minBytes`, `maxBytes`, `maxWait`, `isolationLevel`, and
  `maxAttempts`.
- `reader.consume({ limit, nanoPrecision, expectTimeout })` polls up to `limit`
  messages (or until `maxWait`), returning `Message[]`: `key`/`value` as
  `Uint8Array`, `headers` as a plain object, `time` as an RFC3339 string
  (nanosecond precision when `nanoPrecision`), plus `topic`, `partition`,
  `offset`, `highWaterMark`. It runs in the VU context (rejected from init).
- `reader.close()` closes the client.
- Accepted-but-ignored options are honored as documented in `index.d.ts`:
  `queueCapacity`, `readBatchTimeout`, `readLagInterval`,
  `partitionWatchInterval`, `watchPartitionChanges`, `joinGroupBackoff`,
  `retentionTime`, `readBackoffMin`/`readBackoffMax`, `connectLogger`, and the
  `GROUP_BALANCER_RACK_AFFINITY` value (no rack-affinity balancer in franz-go).
- Add a produce + consume round-trip integration test (raw string value, no
  Schema Registry). The community compat scripts (`test_string.js`, etc.) route
  values through Schema Registry serdes, so porting them into
  `test/integration/compat/` lands with the schema-registry change.

## Capabilities

### New Capabilities

- `consumer`: the `Reader` class consuming messages from Kafka — client
  construction from `ReaderConfig`, `consume`, and `close`.

### Modified Capabilities

<!-- None. `kafka-module` still holds (Reader exists and constructs); this adds
the consumer behavior as a new capability. -->

## Impact

- New consumer code in `pkg/kafka` (Reader client build, group vs direct
  consumption, message decoding) plus unit tests. Reuses the existing client
  builder; no new dependencies.
- A produce + consume round-trip integration test, exercised by the integration
  CI job. (Community compat scripts depend on Schema Registry serdes and are
  ported once that lands.)
- `index.d.ts`: `Reader.consume` doc broadened to "VU context" (matching the
  producer) and given timeout semantics (default throws on `maxWait`;
  `expectTimeout` returns a partial/empty batch); `topic` documented as also
  serving as the group's topic when `groupID` is set; `startOffset` documented as
  applying to direct readers too (with `offset` taking precedence);
  `readBackoffMin`/`readBackoffMax` reclassified from approximate to
  accepted-but-ignored; `connectLogger` annotated accepted-but-ignored.
  Behavior-only clarifications, no type-shape changes.
