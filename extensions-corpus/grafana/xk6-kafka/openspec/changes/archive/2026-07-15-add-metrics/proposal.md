## Why

The community `mostafa/xk6-kafka` v1 emits custom k6 metrics (`kafka_writer_*`,
`kafka_reader_*`) that users read in the end-of-test summary and assert on in
`thresholds`. This extension emits none, so ported v1 scripts silently lose
those metrics and any thresholds built on them. Metrics were deliberately
deferred by the producer and consumer changes; this change delivers them.

## What Changes

- Writer emits per-produce metrics: `kafka_writer_write_count`,
  `kafka_writer_message_count`, `kafka_writer_message_bytes`,
  `kafka_writer_error_count`, `kafka_writer_write_seconds`,
  `kafka_writer_wait_seconds`, `kafka_writer_batch_size`,
  `kafka_writer_batch_bytes`, `kafka_writer_dial_count`,
  `kafka_writer_dial_seconds`.
- Reader emits per-consume metrics: `kafka_reader_message_count`,
  `kafka_reader_message_bytes`, `kafka_reader_fetches_count`,
  `kafka_reader_error_count`, `kafka_reader_lag`, `kafka_reader_offset`,
  `kafka_reader_fetch_bytes`, `kafka_reader_fetch_size`,
  `kafka_reader_read_seconds`, `kafka_reader_wait_seconds`,
  `kafka_reader_dial_count`, `kafka_reader_dial_seconds`,
  `kafka_reader_timeouts_count`.
- Metrics are collected via franz-go client hooks (`kgo.WithHooks`) plus
  counting at produce/consume, registered on the k6 metrics registry, and
  pushed to the VU sample buffer. Topic-scoped metrics carry a `topic` tag;
  connection- and group-level metrics are untagged (no `group` tag).
- Community metrics with no franz-go source are **not** emitted and are
  documented as omitted: the `segmentio/kafka-go` stats gauges
  (`kafka_reader_queue_length`, `kafka_reader_queue_capacity`, config-echo
  gauges) and the ones franz-go exposes no hook for
  (`kafka_writer_retries_count`, `kafka_writer_batch_seconds`,
  `kafka_reader_rebalance_count`).
- No change to `index.d.ts`: like the community extension, these metrics appear
  in the summary and are not part of the typed API surface.

## Capabilities

### New Capabilities
- `metrics`: the custom k6 metrics the Writer and Reader emit, their types
  (counter/trend), tags, and the community names they map to — plus the
  explicitly omitted community metrics that have no franz-go equivalent.

### Modified Capabilities
<!-- None. producer/consumer specs deferred metrics to a separate change; this
     adds a new capability rather than changing their existing requirements. -->

## Impact

- **Code**: `pkg/kafka/producer.go`, `pkg/kafka/consumer.go`, and a new
  metrics module (registry + franz-go hooks + sample emission). Writer/Reader
  gain a metrics collector wired through `clientOptions`.
- **Dependencies**: none new — uses `go.k6.io/k6/v2/metrics` (already present)
  and franz-go hooks.
- **Contract**: none (`index.d.ts` unchanged).
- **Docs**: a README "Metrics" section (self-contained in this change) lists the
  supported and omitted metrics; the broader compatibility matrix (#70) can
  reference it later.
