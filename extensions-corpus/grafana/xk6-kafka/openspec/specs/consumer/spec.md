# consumer Specification

## Purpose
TBD - created by archiving change add-consumer. Update Purpose after archive.
## Requirements
### Requirement: Reader construction

`new Reader(readerConfig)` SHALL build a `twmb/franz-go` consumer client via the
shared client builder (brokers, SASL, TLS). When `groupID` is set it SHALL
consume as a member of that consumer group over `groupTopics` (or `topic`),
applying `groupBalancers`, the heartbeat / session / rebalance timeouts, and the
commit interval. Otherwise (direct mode) it SHALL consume a single partition of
`topic` via direct assignment — `partition` defaults to `0` when omitted —
starting at `offset` if set, otherwise `startOffset`; multi-partition
consumption uses a consumer group. When `groupBalancers` is unset, the `range`
balancer is used (the v1 default), not franz-go's cooperative-sticky default. It
SHALL also map `minBytes`, `maxBytes`, `maxWait`, `isolationLevel`, and
`maxAttempts` (which, when set, MUST be `>= 0`). `brokers` is required;
construction SHALL fail when `brokers` is empty, when neither a group target nor
a direct `topic` is given, when a direct `offset` is below `-1`, or when
`maxAttempts` is negative.

#### Scenario: Construct a group consumer

- **WHEN** a Reader is constructed with `brokers`, a `groupID`, and a topic
- **THEN** a Reader instance is returned without error

#### Scenario: Construct a direct-partition consumer

- **WHEN** a Reader is constructed with `brokers`, a `topic`, and a `partition`
- **THEN** a Reader instance is returned without error

#### Scenario: Construction fails without brokers

- **WHEN** a Reader is constructed with no `brokers`
- **THEN** construction throws an error

#### Scenario: Construction fails on an offset below -1

- **WHEN** a direct Reader is constructed with `offset` less than `-1`
- **THEN** construction throws an error

#### Scenario: startOffset selects the starting point

- **WHEN** a Reader is built with `startOffset` {@link START_OFFSETS_FIRST_OFFSET} or {@link START_OFFSETS_LAST_OFFSET}
- **THEN** the consumer resets to the earliest or latest offset respectively when it has no committed offset

### Requirement: Consuming messages

`reader.consume(consumeConfig)` SHALL poll and return up to `limit` messages
(timeout handling per the Timeout behavior requirement). It SHALL run in the VU context (using the
VU's context so it aborts when the VU stops) and SHALL throw if called from the
init context or after `close`. Each returned message SHALL carry `topic`,
`partition`, `offset`, `highWaterMark`, `key` and `value` as `Uint8Array`,
`headers` as a plain object, and `time` as an RFC3339 string.

#### Scenario: Consume returns messages

- **WHEN** `consume({ limit: 10 })` is called on a Reader for the broker configured via `KAFKA_BROKER` and messages are available
- **THEN** it returns up to 10 messages, each with key, value, headers, and metadata

#### Scenario: Consume in init context is rejected

- **WHEN** `consume` is called from the init context (no VU state)
- **THEN** it throws rather than consuming

### Requirement: Timeout behavior

`consume` SHALL collect messages until it has `limit` or `maxWait` elapses. When
`maxWait` elapses with fewer than `limit` messages, behavior depends on
`expectTimeout`: when `true`, it returns whatever it has collected so far
(possibly an empty array); when `false` (the default), it throws a timeout error.
(This matches the v1 contract and community behavior.) A canceled VU context
(the VU stopping) is distinct from a `maxWait` timeout: it SHALL surface as a
cancellation error, regardless of `expectTimeout`.

#### Scenario: expectTimeout returns a partial batch

- **WHEN** `consume({ limit: 10, expectTimeout: true })` reaches `maxWait` with fewer than 10 messages
- **THEN** it returns the messages collected so far (possibly empty), without throwing

#### Scenario: Default consume throws on timeout

- **WHEN** `consume({ limit: 10 })` (expectTimeout unset/false) reaches `maxWait` before 10 messages arrive
- **THEN** it throws a timeout error

### Requirement: Timestamp precision

When `nanoPrecision` is true, returned message timestamps SHALL retain
nanosecond precision (RFC3339 with nanoseconds); otherwise second-level RFC3339
is used.

#### Scenario: Nanosecond timestamps

- **WHEN** `consume({ limit: 1, nanoPrecision: true })` returns a message
- **THEN** its `time` string includes sub-second (nanosecond) precision

### Requirement: Accepted-but-ignored consumer options

`Reader` SHALL accept the options documented in `index.d.ts` as accepted-but-
ignored without error: `queueCapacity`, `readBatchTimeout`, `readLagInterval`,
`partitionWatchInterval`, `watchPartitionChanges`, `joinGroupBackoff`,
`retentionTime`, `readBackoffMin`/`readBackoffMax`, `connectLogger`, and the
`GROUP_BALANCER_RACK_AFFINITY` value (no rack-affinity balancer exists in
franz-go; it is ignored, and the group uses the `range` default).

#### Scenario: Ignored option does not error

- **WHEN** a Reader is built with `queueCapacity` set
- **THEN** construction succeeds and the option has no effect

### Requirement: Closing the reader

`reader.close()` SHALL close the underlying client, releasing its connections
(and, for a group consumer, leaving the group).

#### Scenario: Close releases the client

- **WHEN** `close()` is called on a reader
- **THEN** the underlying client is closed and its resources released

