## ADDED Requirements

### Requirement: Writer construction

`new Writer(writerConfig)` SHALL build a `twmb/franz-go` producer client via the
shared client builder (brokers, SASL, TLS) and apply the producer options from
`WriterConfig`: default `topic`, `compression`, `balancer` partitioner,
`requiredAcks`, `maxAttempts` (record and unknown-topic retries), `writeTimeout`
(produce request timeout), batching (`batchBytes`, `batchTimeout`), and
`autoCreateTopic`. `brokers` is required; `topic` is the default produce topic
(optional when every message sets its own `topic`). `requiredAcks`, when set,
MUST be `-1`, `0`, or `1`. `maxAttempts`, when set, MUST be `>= 0` and is applied
exactly (including `0` to disable retries) rather than left at the franz-go
default. Construction SHALL fail with an error when `brokers` is empty,
`requiredAcks` is out of range, or `maxAttempts` is negative.

#### Scenario: Construct with brokers and topic

- **WHEN** a Writer is constructed with a broker list and a topic
- **THEN** a Writer instance is returned without error

#### Scenario: Construction fails without brokers

- **WHEN** a Writer is constructed with no `brokers`
- **THEN** construction throws an error

#### Scenario: Construction fails on invalid requiredAcks

- **WHEN** a Writer is constructed with `requiredAcks` set to a value other than `-1`, `0`, or `1`
- **THEN** construction throws an error

#### Scenario: Construction fails on negative maxAttempts

- **WHEN** a Writer is constructed with a negative `maxAttempts`
- **THEN** construction throws an error

#### Scenario: requiredAcks maps to the client

- **WHEN** a Writer is built with `requiredAcks` `-1`, `0`, or `1`
- **THEN** the client is configured to wait for all in-sync replicas, no
  acknowledgement, or the leader only, respectively

#### Scenario: compression maps to the client

- **WHEN** a Writer is built with `compression` set to a {@link COMPRESSION_CODECS} value
- **THEN** the client produces with that compression codec

#### Scenario: maxAttempts and writeTimeout map to the client

- **WHEN** a Writer is built with `maxAttempts` and `writeTimeout` set
- **THEN** the client uses `maxAttempts` for both its record-retry and
  unknown-topic-retry budgets, and `writeTimeout` as the produce request timeout

### Requirement: Message marshaling

`Writer` SHALL marshal each `Message` to a `twmb/franz-go` record: `key` and
`value` accept a string (sent as UTF-8 bytes) or a `Uint8Array`; `headers` (a
plain object) become record headers; the per-message `topic` overrides the
writer's default topic when set; and `time` (a `Date`) sets the record
timestamp.

#### Scenario: String key/value become UTF-8 bytes

- **WHEN** a message with a string `key` and `value` is marshaled
- **THEN** the record's key and value are the UTF-8 bytes of those strings

#### Scenario: Headers and per-message topic are carried on the record

- **WHEN** a message sets `headers` and its own `topic` is marshaled
- **THEN** the record carries those headers and is targeted at that topic (not the default)

### Requirement: Producing messages

`writer.produce(produceConfig)` SHALL produce every message in
`produceConfig.messages`, returning after the broker acknowledges the batch and
throwing on a produce error. It SHALL run in the VU context (default / setup /
teardown), using the VU's context so it aborts when the VU stops, and SHALL
throw if called from the init context.

#### Scenario: Produce succeeds against a broker

- **WHEN** a Writer for the broker configured via `KAFKA_BROKER` produces messages to an auto-created topic
- **THEN** `produce` returns without error

#### Scenario: Produce error surfaces

- **WHEN** a produce request is rejected by the broker (the franz-go produce result carries an error)
- **THEN** `produce` throws

#### Scenario: Produce in init context is rejected

- **WHEN** `produce` is called from the init context (no VU state)
- **THEN** it throws rather than producing

<!-- Reading produced records back (verifying headers/topic/value end to end on
the consume side) is covered by the consumer change. A deterministic broker-side
error case — e.g. producing to a missing topic with auto-create disabled —
requires controlling topic state and is added with the admin change (the broker
default auto-creates topics, so it is not deterministic here). -->

### Requirement: Accepted-but-ignored producer options

`Writer` SHALL accept the options documented in `index.d.ts` as accepted-but-
ignored or approximate without error: `batchSize` is ignored (franz-go batches
by size/time), `BALANCER_CRC32` and a custom `BalancerFunction` are not honored
yet (a supported default partitioner is used), and `readTimeout` and
`connectLogger` are ignored (franz-go manages socket read deadlines internally,
and the connection logger is not yet wired). `writeTimeout` maps to the produce
request timeout.

#### Scenario: Ignored option does not error

- **WHEN** a Writer is built with `batchSize` set
- **THEN** construction succeeds and the option has no effect

### Requirement: Closing the writer

`writer.close()` SHALL flush any buffered messages and close the underlying
client, releasing its connections.

#### Scenario: Close flushes and releases

- **WHEN** `close()` is called on a writer
- **THEN** buffered messages are flushed and the client is closed
