## 1. Metrics collector scaffold

- [x] 1.1 Add `pkg/kafka/metrics.go` with a `metricsCollector` that registers the Writer and Reader metric handles on the k6 metrics registry via `NewMetric`/`MustNewMetric` (which reuse an existing metric when name+type match), obtained from the module instance
- [x] 1.2 Add hook-sourced state: monotonic atomic counters (bytes, dial count, retries, fetches, rebalances, timeouts) plus mutex-guarded observation buffers for trend values (dial/e2e/batch/fetch durations, batch/fetch sizes and bytes), each keyed by topic where the hook provides one
- [x] 1.3 Implement delta emission: for counters, flush `total - lastFlushed` and advance `lastFlushed`; for trends, drain the buffered observations (one sample per value); guard the whole flush as a no-op when `vu.State()` is nil
- [x] 1.4 Emit per-topic samples with a `topic` tag for topic-scoped metrics; emit connection/group-level metrics (`*_dial_*`, `rebalance`) untagged

## 2. Writer metrics

- [x] 2.1 Wire `writerHooks()` into the producer client options (`kgo.WithHooks`) for dial and produce-batch events, bucketing batch trends by the hook-provided topic
- [x] 2.2 Retain `WriterConfig.Topic` on the `Writer` as the default-topic fallback for metric attribution (since `marshalRecord` leaves `record.Topic` empty when the default applies)
- [x] 2.3 In `Writer.produce`, bucket messages by topic (per-message `Topic`, else the writer default), count messages + serialized bytes per topic, and flush `kafka_writer_*` (including `error_count` on failure)
- [x] 2.4 In `Writer.close`, drain the collector (flush pending deltas/trends) before closing the client
- [x] 2.5 Omit `kafka_writer_retries_count` and `kafka_writer_batch_seconds` (no franz-go hook source) — document, don't fake

## 3. Reader metrics

- [x] 3.1 Wire `readerHooks()` into the consumer client options for dial and fetch-batch events, bucketing fetch trends by topic (`kafka_reader_rebalance_count` is omitted — no franz-go rebalance hook)
- [x] 3.2 In `Reader.consume`, bucket returned messages by their own `Topic`; per topic count messages + bytes and record `kafka_reader_lag = max(0, highWatermark-offset-1)` and `kafka_reader_offset`; flush `kafka_reader_*`
- [x] 3.3 Emit `kafka_reader_timeouts_count` on a consume timeout and `kafka_reader_error_count` on fetch errors
- [x] 3.4 In `Reader.close`, drain the collector (flush pending deltas/trends) before closing the client

## 4. Tests

- [x] 4.1 Unit: collector registers all expected metric names with the correct k6 types (Counter/Trend) on a fresh registry
- [x] 4.2 Unit: `flush` is a no-op with a nil VU state (no panic, no sample)
- [x] 4.3 Unit: hook-sourced counters emit **deltas** — two successive flushes of a rising total produce samples that sum to the total, not to an inflated value
- [x] 4.4 Unit: lag derivation returns `max(0, highWatermark-offset-1)`
- [x] 4.5 Unit: per-topic bucketing — a mixed-topic message set yields one sample set per topic, tagged correctly; connection/group metrics emit untagged
- [x] 4.6 Unit: `close` drains pending accumulated metrics (a flush recorded after the last call is emitted on close, no-op with nil VU state)
- [x] 4.7 Integration: a produce/consume round-trip emits `kafka_writer_message_count` / `kafka_reader_message_count` with a `topic` tag (threshold on both); assert an omitted metric (`kafka_reader_queue_length`) is absent

## 5. Docs (self-contained in this change)

- [x] 5.1 Add a README "Metrics" section listing the supported `kafka_writer_*` / `kafka_reader_*` metrics (name, type, tags) and the explicitly omitted ones (`kafka_reader_queue_length`, `kafka_reader_queue_capacity`, config-echo gauges) — so this change satisfies its own documentation requirement without depending on the compatibility-matrix issue (#70)
- [x] 5.2 Note that metrics are franz-go-derived and that batch-granular trends are approximate vs the community's segmentio-derived values
