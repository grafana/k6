## Context

`Reader` is a scaffold constructor. This change implements consuming on the
client/auth foundation (#9): reuse the shared client-options builder, add
consumer options (group or direct), poll records, and decode them to the
`Message` shape. Must conform to `index.d.ts` and stay pure Go.

## Goals / Non-Goals

**Goals:**

- `Reader` builds a consumer (group or direct partition) and `consume` returns
  up to `limit` messages (or until `maxWait`); `close` releases the client.
- Map fetch sizing, offsets, isolation level, group balancers/timeouts.

**Non-Goals:**

- Admin / Schema Registry behavior; metrics (`kafka_reader_*`) — separate change.
- Honoring `GROUP_BALANCER_RACK_AFFINITY` and the other accepted-ignored knobs.

## Decisions

- **Group vs direct.** With `groupID`: `kgo.ConsumerGroup`, `kgo.ConsumeTopics`,
  `kgo.Balancers`, `kgo.HeartbeatInterval`, `kgo.SessionTimeout`,
  `kgo.RebalanceTimeout`, `kgo.AutoCommitInterval` (from `commitInterval`), with
  `startOffset` → `kgo.ConsumeResetOffset`. Without `groupID` (direct mode):
  consume a **single partition** of `topic` via `kgo.ConsumePartitions` —
  `partition` defaults to `0` when omitted — starting at the explicit `offset`
  if set, otherwise the `startOffset` position. Multi-partition consumption uses
  a consumer group (matching v1). Both modes apply `kgo.FetchMinBytes`,
  `kgo.FetchMaxBytes`, `kgo.FetchMaxWait`, `kgo.FetchIsolationLevel`, and
  `kgo.RequestRetries` (`maxAttempts`).
- **Polling & timeout.** `consume` uses `client.PollRecords(ctx, limit)` with a
  context bounded by `maxWait` (parsed duration string; default ~5s — franz-go's
  fetch-wait default — when unset). `maxWait` also sets `kgo.FetchMaxWait`, so it
  bounds both the broker fetch wait and how long `consume` blocks. It collects up
  to `limit` messages. When the deadline is reached with
  fewer than `limit`, behavior branches on `expectTimeout`: `true` returns the
  partial batch (possibly empty); `false` (default) throws a timeout error —
  matching the v1 contract and community reader, where a timeout is an error
  unless `expectTimeout` is set.
- **Message decode.** Iterate fetches per partition to capture the partition
  `HighWatermark`, then map each record to the `Message` shape: `key`/`value`
  as byte arrays, `headers` as a plain object, `time` formatted RFC3339
  (RFC3339Nano when `nanoPrecision`), plus `topic`/`partition`/`offset`.
- **VU context & lifecycle.** Like the producer, `Reader` holds the VU;
  `consume` uses `vu.Context()`, rejects init-context calls (`vu.State() == nil`)
  and calls after `close`. A canceled parent context (VU stopping) is surfaced as
  a cancellation error — distinct from a `maxWait` timeout — and is not masked by
  `expectTimeout`. Rationale: correct cancellation and lifecycle.
- **Group balancers.** range → `RangeBalancer`, round-robin →
  `RoundRobinBalancer`. `GROUP_BALANCER_RACK_AFFINITY` has no franz-go
  equivalent → fall back to a supported balancer (documented accepted-ignored).
  When `groupBalancers` is unset — or only `GROUP_BALANCER_RACK_AFFINITY`
  (ignored) is given — default to **range** (the v1 default) rather than
  franz-go's hidden cooperative-sticky default, which is not in the public API
  and has one-way migration semantics. `GROUP_BALANCER_RACK_AFFINITY` is never
  mapped to cooperative-sticky.
- **Offset validation.** A direct `offset` of `-1` means latest, `0` the
  beginning, any positive value an exact offset; values below `-1` are rejected
  at construction rather than silently treated as "latest".

## Risks / Trade-offs

- **Group rebalance timing** can make integration flaky → the compat round-trip
  test produces then consumes with a bounded wait and retries the poll.
- **HighWatermark availability** depends on the fetch carrying it. franz-go
  provides it per partition (`FetchTopicPartition.HighWatermark`); when truly
  unavailable, leave `highWaterMark` at 0 (unknown) — never substitute the
  record offset, which would falsely collapse lag to zero.

## Open Questions

- None outstanding.
