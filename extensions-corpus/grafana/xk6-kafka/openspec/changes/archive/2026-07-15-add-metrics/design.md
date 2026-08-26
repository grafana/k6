## Context

k6 extensions surface custom metrics by registering them on the k6 metrics
registry (from the module init environment) and pushing `metrics.Sample`s to the
VU's sample buffer (`state.Samples`) during VU execution. The community
`mostafa/xk6-kafka` v1 exposes `kafka_writer_*` / `kafka_reader_*` metrics that
users assert on in `thresholds`; this extension emits none. Those metrics came
straight from `segmentio/kafka-go`'s `WriterStats` / `ReaderStats`; franz-go
does not expose the same struct, but its hook interfaces (`kgo.WithHooks`) cover
bytes on the wire, broker connects, request latency (E2E), and produce/fetch
batch events. Message-level counts are cleanest at the produce/consume call
sites, which already run on the VU goroutine.

## Goals / Non-Goals

**Goals:**
- Emit the mappable community `kafka_writer_*` / `kafka_reader_*` metrics with
  the same names and sensible counter/trend types, tagged by `topic`.
- Keep sample emission on the VU goroutine (correct k6 semantics).
- Reuse one collector design across Writer and Reader.

**Non-Goals:**
- 1:1 numeric parity with segmentio-derived metrics (different client).
- Emitting metrics with no franz-go source (`kafka_reader_queue_length`,
  `kafka_reader_queue_capacity`, config-echo gauges) — omitted and documented.
- Declaring metrics in `index.d.ts` (community does not; they are summary-only).

## Decisions

- **Hybrid collection: hooks accumulate, call sites emit.** franz-go invokes
  hooks from its own internal goroutines, where the VU context and
  `state.Samples` are not safe to touch. So hooks only update collector state
  under a mutex/atomics; the Writer/Reader drain it into `metrics.Sample`s at
  the end of each `produce` / `consume` call — on the VU goroutine, where
  `state.Samples` and tags are valid. *Alternative:* push samples directly from
  hooks — rejected: wrong goroutine, no VU state, racy.

- **Counters emit deltas; trends emit buffered observations.** k6 **sums**
  counter samples, so a hook-sourced counter (monotonic atomic total) is flushed
  as `total - lastFlushed` (a delta), and `lastFlushed` is advanced — summation
  then reconstructs the true total. Flushing running totals would double-count
  every subsequent call, so it is prohibited. Trend metrics need per-observation
  values, so hooks that carry a value (dial/e2e/batch durations, batch/fetch
  sizes and bytes) append each observation to a small mutex-guarded buffer that
  the flush drains (the buffer only spans one produce/consume call, so it stays
  bounded). *Alternative:* emit a single aggregated trend value per flush —
  rejected: loses the distribution k6 trends are for.

- **Per-topic attribution, not a single tag.** A `produce` call can target
  multiple topics (`ProduceMessage.Topic` per message) and a group `consume`
  can span `groupTopics`. Message-level metrics (`*_message_count/bytes`, reader
  `lag`/`offset`) are therefore bucketed **by topic** at the call site (each
  record carries its topic) and emitted as one sample set per topic. Batch/fetch
  hooks (`HookProduceBatchWritten`, `HookFetchBatchRead`) receive the topic from
  franz-go, so those trends are bucketed per topic too. Connection-level
  (`*_dial_*`) and group-level (`rebalance`) metrics have no meaningful single
  topic and are emitted **untagged**. *Alternative:* one `topic` tag per call —
  rejected: misattributes mixed-topic batches (flagged in review).

  The Writer retains its configured default topic (`WriterConfig.Topic`) as a
  field, because `marshalRecord` leaves `record.Topic` empty when the default
  applies. A produced message with no explicit `Topic` is attributed to that
  default; one with an explicit `Topic` to its own.

- **Flush on close, not only on produce/consume.** Hooks accumulate between
  calls, so events after the last `produce`/`consume` (late dials, in-flight
  fetch completions, retries, a final rebalance) would be lost if only those
  calls flush. `Writer.close` and `Reader.close` therefore drain the collector
  before closing the client. Close runs in teardown (VU context present), and
  the flush is a no-op when no VU state is available, so it is safe.

- **Message counts/bytes and lag come from the call site, not hooks.**
  `produce` knows the records and their serialized sizes; `consume` knows each
  record's offset and the partition high watermark, so
  `kafka_reader_lag = max(0, highWatermark - offset - 1)` is computed at decode.
  This avoids depending on hook batch granularity for the most-used metrics.

- **One `metricsCollector` type**, constructed from the k6 metrics registry
  (`NewMetric`/`MustNewMetric`), holding the registered `*metrics.Metric`
  handles plus atomic accumulators and trend buffers. It exposes
  `writerHooks()` / `readerHooks()` (the `kgo.Hook` set) added in
  `clientOptions`, per-call flush entry points for Writer/Reader, and a
  close-time drain. Registry handles are created once per module instance.

- **Nil-VU safety.** Flushing checks `vu.State()`; when absent (init context) it
  skips emission rather than panicking, consistent with the produce/consume VU
  guards already in place.

- **Metric names/types mirror community v1** for familiarity: `*_count` /
  `*_bytes` as counters, `*_seconds` / `*_size` / `*_bytes`(per-batch) / `offset`
  / `lag` as trends. Names are the contract for compatibility even though the
  numbers are franz-go-derived.

## Risks / Trade-offs

- **Approximate trends (batch vs per-record).** franz-go reports batch-level
  write/fetch events; some `*_seconds` / `*_size` trends are batch-granular, not
  per-message. → Document as "franz-go-derived, approximate" in the compat
  matrix; keep the message counts exact (from call sites).
- **Hook goroutine races.** Mitigated by the accumulate-in-atomics /
  emit-at-call-site split; hooks never touch k6 state.
- **Per-VU vs shared client.** Each VU builds its own client and collector, so
  counters are per-VU; k6 aggregates across VUs in the summary, matching how
  community metrics behave. → No shared state needed.
- **Metric registration collisions.** k6's `Registry.NewMetric(name, type,
  valueType)` already returns the existing metric when the name and type match
  (and errors on a type conflict); `MustNewMetric` is the panic-on-conflict
  variant. → Register with `NewMetric`/`MustNewMetric` and rely on that reuse;
  there is no `GetOrNew` in the k6 registry API.

## Open Questions

- Exact franz-go hook → trend mapping for `*_wait_seconds` and
  `*_batch_seconds` (which hook timestamps best approximate segmentio's
  semantics) — resolved during implementation against the franz-go hook API.
- Whether to split delivery into two PRs (producer-metrics, consumer-metrics)
  sharing the collector, or ship as one — decided in tasks.
