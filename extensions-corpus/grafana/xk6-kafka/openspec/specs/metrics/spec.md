# metrics Specification

## Purpose
The custom k6 metrics the Writer and Reader emit into the test summary — their
names (matching community `mostafa/xk6-kafka` v1), types, tags, and delta/trend
semantics — plus the community metrics that have no `twmb/franz-go` source and
are documented as omitted rather than emitted.
## Requirements
### Requirement: Emitted metrics use community names and correct k6 types

The Writer and Reader SHALL emit custom k6 metrics registered on the k6 metrics
registry, using the community `mostafa/xk6-kafka` v1 metric names. `*_count`
metrics SHALL be k6 **Counters**; `*_seconds`, `*_bytes` (per batch/fetch),
`*_size`, `*_offset`, and `*_lag` metrics SHALL be k6 **Trends**.

Because k6 **sums** counter samples across the test, counter metrics sourced
from franz-go hooks (which accumulate monotonic totals) SHALL be emitted as
per-flush **deltas** (the increase since the previous flush), so the summed
value reconstructs the true total. Emitting a running total each flush is
incorrect and MUST NOT be done. Trend metrics SHALL emit one sample per observed
value (buffered by the hooks, drained at flush), not an aggregate.

Writer metrics: `kafka_writer_write_count`, `kafka_writer_message_count`,
`kafka_writer_message_bytes`, `kafka_writer_error_count`,
`kafka_writer_dial_count` (counters); `kafka_writer_write_seconds`,
`kafka_writer_wait_seconds`, `kafka_writer_dial_seconds`,
`kafka_writer_batch_size`, `kafka_writer_batch_bytes` (trends).

Reader metrics: `kafka_reader_message_count`, `kafka_reader_message_bytes`,
`kafka_reader_fetches_count`, `kafka_reader_error_count`,
`kafka_reader_timeouts_count`, `kafka_reader_dial_count` (counters);
`kafka_reader_read_seconds`, `kafka_reader_wait_seconds`,
`kafka_reader_dial_seconds`, `kafka_reader_fetch_bytes`,
`kafka_reader_fetch_size`, `kafka_reader_offset`, `kafka_reader_lag` (trends).

#### Scenario: Hook-sourced counters emit deltas, not totals

- **WHEN** a metric backed by a franz-go hook (e.g. `kafka_writer_dial_count`,
  `kafka_reader_fetches_count`) is flushed on two successive produce/consume
  calls
- **THEN** each flush emits only the increase since the previous flush, so the
  summed counter equals the true total and is not inflated

### Requirement: Metrics are tagged by topic where a topic is meaningful

Topic-scoped metrics SHALL carry a `topic` tag identifying the topic, and a
single `produce` or `consume` call that spans multiple topics SHALL attribute
them per topic (one sample set per distinct topic), never to a single topic.
Topic-scoped metrics are the ones franz-go attributes to a topic: the
message-level metrics (`*_message_count`, `*_message_bytes`,
`kafka_reader_lag`, `kafka_reader_offset`) and the per-topic batch/fetch metrics
(`kafka_writer_write_count`, `kafka_writer_batch_size`,
`kafka_writer_batch_bytes`, `kafka_reader_fetches_count`,
`kafka_reader_fetch_size`, `kafka_reader_fetch_bytes`).

Metrics that are not topic-scoped SHALL be emitted without a `topic` tag rather
than attributed to an arbitrary topic. These are the broker-request-level timing
metrics — `*_write_seconds`, `*_read_seconds`, `*_wait_seconds`,
`*_dial_seconds`, `*_dial_count` (a single broker request batches many
topics/partitions, so no single topic applies) — and the call-level
`*_error_count` / `kafka_reader_timeouts_count`.

#### Scenario: Multi-topic produce attributes counts per topic

- **WHEN** one `writer.produce` call sends messages to topics `a` and `b`
- **THEN** `kafka_writer_message_count` and `kafka_writer_message_bytes` are
  emitted per topic, each sample tagged with its own topic, summing to the
  batch totals

#### Scenario: Multi-topic group consume attributes counts per topic

- **WHEN** one `reader.consume` call on a group subscribed to `groupTopics`
  returns messages from topics `a` and `b`
- **THEN** `kafka_reader_message_count`, `kafka_reader_message_bytes`,
  `kafka_reader_lag`, and `kafka_reader_offset` are emitted per topic, each
  tagged with the topic the message came from

### Requirement: Writer emits produce metrics

A `Writer` SHALL emit its metrics to the VU sample buffer at the end of each
`produce` call, on the VU goroutine.

#### Scenario: Successful produce records message counts and bytes

- **WHEN** `writer.produce` successfully writes N messages
- **THEN** `kafka_writer_message_count` increases by N and
  `kafka_writer_message_bytes` by the total serialized key+value bytes,
  attributed per topic

#### Scenario: Default-topic messages attributed to the writer's topic

- **WHEN** `writer.produce` sends a message with no explicit `topic` and the
  writer was configured with a default `topic`
- **THEN** that message's metrics are attributed to the writer's default topic,
  not to an empty topic

#### Scenario: Produce failure records an error, not a message count

- **WHEN** a `writer.produce` call fails (or partially fails)
- **THEN** `kafka_writer_error_count` increases by the number of failed records,
  and `kafka_writer_message_count` / `kafka_writer_message_bytes` count only the
  records that actually succeeded (none, on a total failure)

#### Scenario: Metrics require the VU context

- **WHEN** metrics would be emitted with no VU state (e.g. init context)
- **THEN** no sample is pushed and no panic occurs

### Requirement: Reader emits consume metrics

A `Reader` SHALL emit its metrics to the VU sample buffer at the end of each
`consume` call, on the VU goroutine.

#### Scenario: Successful consume records message counts and bytes

- **WHEN** `reader.consume` returns N messages
- **THEN** `kafka_reader_message_count` increases by N and
  `kafka_reader_message_bytes` by the total key+value bytes, attributed per
  topic

#### Scenario: Lag is derived per message

- **WHEN** a message is consumed at offset O from a partition with high
  watermark H
- **THEN** `kafka_reader_lag` records `max(0, H - O - 1)` and
  `kafka_reader_offset` records O, tagged with the message's topic

#### Scenario: Consume timeout records a timeout

- **WHEN** a `reader.consume` call times out before reaching its limit
- **THEN** `kafka_reader_timeouts_count` increases

#### Scenario: A consume that returns no messages does not count messages

- **WHEN** a `reader.consume` call returns `nil` with an error (fetch error,
  cancellation, or a non-`expectTimeout` timeout)
- **THEN** `kafka_reader_message_count` / `kafka_reader_message_bytes` are not
  emitted for that call; only the relevant error/timeout counter is

### Requirement: Pending metrics are flushed on close

`Writer.close` and `Reader.close` SHALL flush any metrics accumulated since the
last `produce` / `consume` before closing the underlying client, so events that
occur after the final call — late dials, in-flight fetch completions, retries,
a final rebalance — are not lost. This flush SHALL be a no-op when no VU state
is available.

#### Scenario: Close flushes late hook events

- **WHEN** dial, fetch, retry, or rebalance activity is recorded after the last
  `produce` / `consume` call, and `close` is then called in a VU context
- **THEN** the corresponding metric deltas and buffered trend values are emitted
  before the client is closed

### Requirement: Omitted community metrics are documented, not emitted

The extension MUST document, rather than emit, community metrics that have no
`twmb/franz-go` source. These are the `segmentio/kafka-go` stats-derived gauges
(`kafka_reader_queue_length`, `kafka_reader_queue_capacity`, config-echo gauges)
and the metrics for which franz-go exposes no hook: `kafka_writer_retries_count`,
`kafka_writer_batch_seconds`, and `kafka_reader_rebalance_count`. Such metrics
SHALL be absent from the summary (not emitted as zero or faked), and their
absence SHALL be recorded in this change's own user-facing docs (a README
metrics section), independent of any other change.

#### Scenario: Unsupported metrics are absent

- **WHEN** a test inspects the summary for `kafka_reader_queue_length`,
  `kafka_writer_retries_count`, or `kafka_reader_rebalance_count`
- **THEN** those metrics are absent (documented as unsupported), and their
  absence does not fail the run

