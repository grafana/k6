# Migration Guide

## v2 Constructor Parity

| Old API | New API | Status |
| --- | --- | --- |
| `new Writer(config)` | `new Producer(config)` | `Writer` is a deprecated alias in `v2.x` |
| `new Reader(config)` | `new Consumer(config)` | `Reader` is a deprecated alias in `v2.x` |
| `new Connection(config)` | `new AdminClient(config)` | `Connection` is a deprecated alias in `v2.x` |

## Method Parity

| Old API | New API | Notes |
| --- | --- | --- |
| `writer.produce({ messages })` | `producer.produce({ messages })` | Same payload shape |
| `reader.consume({ limit })` | `consumer.consume({ maxMessages })` | Both keys are accepted in `v2.x` |
| `connection.listTopics()` | `adminClient.listTopics()` | `Connection` returns `string[]`; `AdminClient` returns topic metadata objects |
| `connection.createTopic()` | `adminClient.createTopic()` | Same topic payload shape |
| `connection.deleteTopic()` | `adminClient.deleteTopic()` | Same topic-name argument |

## API Reference

- Legacy unversioned reference snapshot: [api-docs/docs/README.md](./api-docs/docs/README.md)
- Versioned v2 reference: [api-docs/v2/docs/README.md](./api-docs/v2/docs/README.md)
- Versioned v2 declarations: [api-docs/v2/index.d.ts](./api-docs/v2/index.d.ts)

## Config Notes

- `WriterConfig` and `ReaderConfig` remain the input shapes for `Producer` and `Consumer` in `v2.0.0`.
- `ConnectionConfig` now also accepts `brokers` for `AdminClient`. The legacy `Connection` constructor still accepts `address`.
- `ConsumeConfig.maxMessages` is the preferred v2 name. `ConsumeConfig.limit` is still accepted for compatibility.

## Known v2 Differences

- Custom writer balancer configuration is not supported on the Confluent compatibility path. Scripts that rely on `balancer` or a custom balancer callback should stay on the v1 surface until a replacement is implemented.
- `AdminClient.listTopics()` returns structured topic metadata. The deprecated `Connection.listTopics()` alias keeps the old `string[]` shape.
- `SCHEMA_TYPE_PROTOBUF` is implemented in `v2.1.0` for `SchemaRegistry.serialize()` and `SchemaRegistry.deserialize()` in both Schema Registry and standalone flows.
- New schema metadata for Protobuf: `messageName` and `dependencies` (standalone import map).
- New container metadata for Protobuf: `protobufFormat` with `"object"` (default) and `"bytes"` modes.

## Metric Compatibility Appendix

`v2.0.0` keeps the existing `kafka_writer_*` and `kafka_reader_*` metric names even when scripts move to `Producer` and `Consumer`. The runtime is now Confluent-backed, so some series are compatibility-derived from produce/consume operations or effective client config instead of `kafka-go`'s internal stats structs.

### Unchanged Metric Names

| Metric family | Metrics | Notes |
| --- | --- | --- |
| Reader metrics | `kafka_reader_dial_count`, `kafka_reader_fetches_count`, `kafka_reader_message_count`, `kafka_reader_message_bytes`, `kafka_reader_rebalance_count`, `kafka_reader_timeouts_count`, `kafka_reader_error_count`, `kafka_reader_dial_seconds`, `kafka_reader_read_seconds`, `kafka_reader_wait_seconds`, `kafka_reader_fetch_size`, `kafka_reader_fetch_bytes`, `kafka_reader_offset`, `kafka_reader_lag`, `kafka_reader_fetch_bytes_min`, `kafka_reader_fetch_bytes_max`, `kafka_reader_fetch_wait_max`, `kafka_reader_queue_length`, `kafka_reader_queue_capacity` | Names are unchanged in `v2.0.0`. |
| Writer metrics | `kafka_writer_write_count`, `kafka_writer_message_count`, `kafka_writer_message_bytes`, `kafka_writer_error_count`, `kafka_writer_batch_seconds`, `kafka_writer_batch_queue_seconds`, `kafka_writer_write_seconds`, `kafka_writer_wait_seconds`, `kafka_writer_retries_count`, `kafka_writer_batch_size`, `kafka_writer_batch_bytes`, `kafka_writer_attempts_max`, `kafka_writer_batch_max`, `kafka_writer_batch_timeout`, `kafka_writer_read_timeout`, `kafka_writer_write_timeout`, `kafka_writer_acks_required`, `kafka_writer_async` | Names are unchanged in `v2.0.0`. |

### Compatibility-Derived Semantics

| Metrics | Current v2.0.0 source |
| --- | --- |
| `kafka_reader_dial_count`, `kafka_reader_fetches_count`, `kafka_reader_message_count`, `kafka_reader_message_bytes`, `kafka_reader_read_seconds`, `kafka_reader_fetch_size`, `kafka_reader_fetch_bytes`, `kafka_reader_offset` | Derived from the Confluent-backed `Consumer.consume()` path and the messages returned by that poll cycle. |
| `kafka_reader_fetch_bytes_min`, `kafka_reader_fetch_bytes_max`, `kafka_reader_fetch_wait_max` | Compatibility gauges now reflect the effective Confluent consumer config (`fetch.min.bytes`, `fetch.max.bytes`, `fetch.wait.max.ms`) when available. |
| `kafka_reader_rebalance_count`, `kafka_reader_queue_length`, `kafka_reader_queue_capacity` | Retained for dashboard compatibility. They remain limited-signal placeholders on the Confluent path in `v2.0.0`. |
| `kafka_writer_write_count`, `kafka_writer_message_count`, `kafka_writer_message_bytes`, `kafka_writer_batch_seconds`, `kafka_writer_write_seconds`, `kafka_writer_batch_size`, `kafka_writer_batch_bytes` | Derived from the Confluent-backed `Producer.produce()` path and the resolved topics/messages in that call. |
| `kafka_writer_batch_timeout`, `kafka_writer_read_timeout`, `kafka_writer_write_timeout`, `kafka_writer_acks_required`, `kafka_writer_batch_max` | Compatibility gauges now reflect the effective Confluent producer config (`linger.ms`, `socket.timeout.ms`, `message.timeout.ms`, `acks`, `batch.num.messages`) when available. |
| `kafka_writer_retries_count`, `kafka_writer_attempts_max`, `kafka_writer_async` | Retained for dashboard compatibility. They remain compatibility placeholders or config-derived signals on the Confluent path in `v2.0.0`. |

### Renamed Metrics

None in `v2.0.0`.

### Removed Metrics

None in `v2.0.0`.

## Deprecation Policy

- `Writer`, `Reader`, and `Connection` remain available throughout `v2.x`.
- New examples, docs, and scripts should prefer `Producer`, `Consumer`, and `AdminClient`.
- Removal of the deprecated aliases is planned for the next major version only after replacement coverage and migration notes are complete.
