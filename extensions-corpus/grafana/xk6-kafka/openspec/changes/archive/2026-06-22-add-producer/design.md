## Context

`Writer` is a scaffold constructor. This change implements producing on top of
the client/auth foundation (#9): reuse the shared client-options builder, add
producer-specific options, and marshal JS messages to franz-go records. Must
conform to `index.d.ts` and stay pure Go.

## Goals / Non-Goals

**Goals:**

- `Writer` builds a franz-go producer and `produce` writes messages, blocking
  until acknowledged; `close` flushes and closes.
- Map `compression`, `balancer`, `requiredAcks`, batching, and `autoCreateTopic`.

**Non-Goals:**

- Consumer, admin, Schema Registry behavior.
- Metrics (`kafka_writer_*`) — separate change.
- Honoring `BALANCER_CRC32` / custom balancer fn / `batchSize` (accepted-ignored
  per the contract).

## Decisions

- **Producer client options.** Extend the shared `clientOptions` with producer
  opts: `kgo.DefaultProduceTopic`, `kgo.ProducerBatchCompression`,
  `kgo.RecordPartitioner`, `kgo.RequiredAcks`, `kgo.ProducerBatchMaxBytes`,
  `kgo.ProducerLinger` (from `batchTimeout`), `kgo.ProduceRequestTimeout` (from
  `writeTimeout`), and `kgo.AllowAutoTopicCreation`. `maxAttempts` maps to both
  `kgo.RecordRetries` and `kgo.UnknownTopicRetries`, so unknown-topic failures
  follow the same retry budget rather than franz-go's separate default (4).
  Rationale: one client path; producer concerns layered on top, and every
  `WriterConfig` field declared in `index.d.ts` is either mapped or documented
  as accepted-ignored/approximate.
- **`writeTimeout` semantics.** Map to `kgo.ProduceRequestTimeout` (how long the
  broker may take to ack a produce request) — the closest franz-go knob to v1's
  socket write timeout. `index.d.ts` wording is updated to match.
- **`readTimeout` and `connectLogger` are accepted-but-ignored.** franz-go
  manages socket read deadlines internally (no equivalent to v1's socket read
  timeout), and wiring the k6 logger into the shared client builder is deferred
  (it also applies to the reader). Both are accepted without error and have no
  effect; documented in `index.d.ts`.
- **Synchronous produce.** Use `client.ProduceSync(ctx, records...)` so
  `produce` blocks until the broker acknowledges and surfaces errors directly —
  matching the v1 blocking semantics k6 scripts expect. Rationale: simplest
  correct behavior; async/batched producing can be added later if needed.
- **Balancer mapping.** round-robin → `RoundRobinPartitioner`; hash / murmur2 →
  `StickyKeyPartitioner` (murmur2/Kafka-compatible); least-bytes →
  `LeastBackupPartitioner` (approximate). `BALANCER_CRC32` and a custom
  `BalancerFunction` are not honored yet — fall back to the default partitioner
  (documented accepted-ignored). Rationale: use the closest maintained
  franz-go partitioners; avoid inventing a CRC32 one in this change.
- **`requiredAcks` mapping.** `-1` → all ISR, `0` → none, `1` → leader.
- **Produce runs in the VU context.** The `Writer` holds the `modules.VU`;
  `produce` uses `vu.Context()` so it aborts when the VU stops, and rejects
  init-context calls (`vu.State() == nil`), matching v1's "call from the VU
  function" contract. Rationale: correct cancellation and lifecycle.
- **Construction validation.** `brokers` is required — `openWriter` errors on an
  empty list rather than failing later on first produce. `topic` is the default
  produce topic (per-message `topic` overrides it), so `index.d.ts` marks it
  optional rather than falsely-required.
- **Message marshaling.** `key`/`value`: a JS string → its UTF-8 bytes, a
  `Uint8Array` → its bytes. `headers`: a plain object → `kgo.RecordHeader`s.
  `topic`: per-message override of the default. `time`: a `Date` → the record
  timestamp. Rationale: matches the `Message` contract and the auth-change
  config-decoding approach (sobek `ExportTo`).

## Risks / Trade-offs

- **Partitioner fidelity** (least-bytes ≈ least-backup; no CRC32) → documented as
  accepted-ignored/approximate in `index.d.ts`; revisit if users need exact
  parity.
- **Synchronous produce throughput** is lower than batched async → acceptable for
  the v1 baseline; async is a possible later option.
- **Error aggregation** from `ProduceSync` over a batch → surface the first
  error with context; all-or-nothing per call.

## Open Questions

- Whether to expose async/fire-and-forget producing later (v1 had an `async`
  notion) — out of scope here.
