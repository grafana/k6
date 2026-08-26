# Compatibility matrix

How `grafana/xk6-kafka` compares to the community
[`mostafa/xk6-kafka`](https://github.com/mostafa/xk6-kafka) **v1** API it targets.
The goal is **familiarity, not a guarantee**: most v1 scripts run with little or
no change, but identical behavior is not promised. See [MIGRATION.md](MIGRATION.md)
for how to port a script.

Legend: ✅ supported · ⚠️ supported with a documented difference · 🅸 accepted but
ignored (no-op, never errors) · ❌ not supported (errors or absent).

## Classes & API surface

| Feature | Status | Notes |
|---|---|---|
| `Writer` (`produce`, `close`) | ✅ | |
| `Reader` (`consume`, `close`) | ✅ | Consumer group and direct-partition modes |
| `Connection` (`createTopic`, `deleteTopic`, `listTopics`, `close`) | ✅ | |
| `SchemaRegistry` (`serialize`, `deserialize`, `getSchema`, `createSchema`, `getSubjectName`) | ✅ | Standalone and registry-backed |
| `LoadJKS` | ⚠️ | JKS keystores only (not PKCS#12) |
| v2 classes: `Producer` / `Consumer` / `AdminClient` | ❌ | v1 API only; the v2 surface is out of scope |

## Producer (`Writer`)

| Option | Status | Notes |
|---|---|---|
| `brokers`, `topic`, `autoCreateTopic` | ✅ | |
| `compression` (`gzip`/`snappy`/`lz4`/`zstd`) | ✅ | |
| `requiredAcks` (`-1`/`0`/`1`) | ✅ | Non-`all` acks disable the idempotent producer |
| `balancer` (`round_robin`/`least_bytes`/`hash`/`murmur2`) | ✅ | |
| `balancer` (`crc32`, or a custom function) | ⚠️ | Falls back to franz-go's default (murmur2-compatible) partitioner |
| `batchBytes`, `batchTimeout`, `maxAttempts`, `writeTimeout` | ✅ | Durations are nanoseconds |
| `batchSize`, `readTimeout`, `connectLogger` | 🅸 | No franz-go equivalent |

## Consumer (`Reader`)

| Option | Status | Notes |
|---|---|---|
| `brokers`, `topic`, `partition`, `offset` | ✅ | |
| `groupID`, `groupTopics` | ✅ | Offsets are committed on `close` |
| `startOffset`, `minBytes`, `maxBytes`, `maxWait`, `maxAttempts`, `isolationLevel` | ✅ | |
| `groupBalancers` (`range`, `round_robin`) | ✅ | |
| `groupBalancers` (`rack_affinity`) | 🅸 | No franz-go equivalent; caller falls back to `range` |
| `heartbeatInterval`, `sessionTimeout`, `rebalanceTimeout`, `commitInterval` | ✅ | |
| `consume({ limit, expectTimeout, nanoPrecision })` | ✅ | `consume` returns `Uint8Array` key/value; `time` is an RFC3339 string; `highWaterMark` present |

## Auth

| Feature | Status | Notes |
|---|---|---|
| SASL `PLAIN`, `SCRAM-SHA-256`, `SCRAM-SHA-512` | ✅ | |
| SASL `SSL` (PLAIN over TLS) | ✅ | Requires TLS enabled |
| SASL `AWS_IAM` | ❌ | Deferred; errors "not yet implemented" (tracked separately) |
| SASL `AZURE_ENTRA` | ❌ | v2-only mechanism; not present |
| TLS (`enableTls`, `minVersion`, `clientCertPem`, `clientKeyPem`, `serverCaPem`, `insecureSkipTlsVerify`) | ✅ | Kafka client; also used for the Schema Registry HTTPS client |
| JKS via `LoadJKS` → PEM → TLS | ⚠️ | JKS only; end-to-end against a TLS broker not yet in CI |

## Serdes & Schema Registry

| Feature | Status | Notes |
|---|---|---|
| STRING, BYTES serdes | ✅ | Schemaless (no schema object) |
| AVRO serdes (standalone + registry) | ✅ | Confluent wire format when `schema.id` is set |
| JSON serdes | ⚠️ | **Requires a schema object** — schemaless JSON is not supported (community allows it). Validation is required-fields (+ basic) only |
| PROTOBUF serdes | ❌ | v2-only community feature; errors |
| Complex schema references (Protobuf `import`, JSON `$ref`) | ❌ | |
| Subject naming: `TOPIC_NAME_STRATEGY` | ✅ | Any serdes |
| Subject naming: `RECORD_NAME_STRATEGY`, `TOPIC_RECORD_NAME_STRATEGY` | ⚠️ | Avro named schemas only; JSON/Protobuf record naming errors |
| Schema caching (`enableCaching`, client-level) | ⚠️ | Opt-in; a cached `latest` is not refreshed mid-run. Parsed-Avro reuse is always on. Per-schema `Schema.enableCaching` is 🅸 |
| `serialize()` return type | ✅ | `Uint8Array` (matches `index.d.ts`) |

## Metrics

`kafka_writer_*` / `kafka_reader_*` are emitted into the summary (usable in
`thresholds`). See the README "Metrics" section for the full list and tagging.

| Metric group | Status | Notes |
|---|---|---|
| message counts/bytes, lag, offset, error/timeout counts | ✅ | Exact (measured at produce/consume) |
| batch/fetch sizes & bytes, dial/write/read/wait seconds | ⚠️ | franz-go-derived, batch-granular — approximate |
| `kafka_writer_retries_count`, `kafka_writer_batch_seconds`, `kafka_reader_rebalance_count`, `kafka_reader_queue_length`, `kafka_reader_queue_capacity` | ❌ | No franz-go source; not emitted |

## Constants

All v1 constants are exported (`CODEC_*`, `SASL_*`, `TLS_*`, `KEY`/`VALUE`,
`ISOLATION_LEVEL_*`, `START_OFFSETS_*` + `FIRST_OFFSET`/`LAST_OFFSET` aliases,
`*_NAME_STRATEGY`, `BALANCER_*`, `GROUP_BALANCER_*`, `SCHEMA_TYPE_*`, time units).
`SCHEMA_TYPE_PROTOBUF` and `SASL_AWS_IAM` are exported for source compatibility
but error at use (see above).
