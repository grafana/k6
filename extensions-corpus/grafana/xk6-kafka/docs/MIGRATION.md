# Migrating from `mostafa/xk6-kafka`

`grafana/xk6-kafka` reimplements the community
[`mostafa/xk6-kafka`](https://github.com/mostafa/xk6-kafka) **v1** API in pure Go
(`CGO_ENABLED=0`, no `librdkafka`). Most v1 scripts run with little or no change.
This guide covers what to change; see [COMPATIBILITY.md](COMPATIBILITY.md) for the
full feature matrix.

## 1. Build with this module

```bash
# was: xk6 build --with github.com/mostafa/xk6-kafka
xk6 build --with github.com/grafana/xk6-kafka
```

The import stays the same:

```javascript
import { Writer, Reader, Connection, SchemaRegistry } from "k6/x/kafka";
```

## 2. Runs unchanged

- Produce / consume (string, bytes, Avro), including consumer groups and
  direct-partition reads.
- Topic admin via `Connection` (create / list / delete).
- SASL `PLAIN` / `SCRAM-SHA-256` / `SCRAM-SHA-512`, `SASL_SSL`, and TLS.
- Avro serdes, standalone and Schema-Registry-backed, incl. `TOPIC_NAME_STRATEGY`
  / `RECORD_NAME_STRATEGY` subject naming.
- The `kafka_writer_*` / `kafka_reader_*` metric names used in `thresholds`
  (with the exceptions listed below).

## 3. Changes you may need

### JSON serdes require a schema

The community extension allows **schemaless** JSON serdes. Here, JSON serdes
require a schema object (used for validation; standalone = a schema whose `id`
is absent). Add one:

```javascript
// was (community): sr.serialize({ data, schemaType: SCHEMA_TYPE_JSON })
const schema = {
  schema: JSON.stringify({ type: "object", properties: { name: { type: "string" } }, required: ["name"] }),
  schemaType: SCHEMA_TYPE_JSON,
};
sr.serialize({ data: { name: "x" }, schema, schemaType: SCHEMA_TYPE_JSON });
sr.deserialize({ data: msg.value, schema, schemaType: SCHEMA_TYPE_JSON });
```

STRING and BYTES serdes remain schemaless.

### Field names: use the v1 names

This extension follows the **v1** field names. If you are copying from current
community (v2) docs, rename:

| current community (v2) | here (v1) |
|---|---|
| `groupId` | `groupID` |
| `maxMessages` | `limit` |

### Unsupported — remove or replace

- **Protobuf serdes** (`SCHEMA_TYPE_PROTOBUF`) — not supported; use Avro or JSON.
- **`SASL_AWS_IAM`** — not implemented yet (deferred).
- **`SASL_AZURE_ENTRA`** and the **v2 classes** (`Producer`/`Consumer`/`AdminClient`) — not present.

### Accepted but ignored (no-ops — safe to leave, but they do nothing)

`batchSize`, `readTimeout`, `connectLogger`; `GROUP_BALANCER_RACK_AFFINITY`;
`BALANCER_CRC32` and custom balancer functions (fall back to the default
partitioner); the per-schema `Schema.enableCaching` field (caching is
client-level via `SchemaRegistryConfig.enableCaching`).

## 4. Behavior differences to know

- **Metrics not emitted** (no franz-go source): `kafka_writer_retries_count`,
  `kafka_writer_batch_seconds`, `kafka_reader_rebalance_count`,
  `kafka_reader_queue_length`, `kafka_reader_queue_capacity`. Batch/fetch size
  and `*_seconds` trends are franz-go-derived and approximate.
- **Subject-name record strategies** now return the real Avro record name (or an
  error for non-Avro schemas). If a script previously relied on the record
  strategies silently behaving like `TOPIC_NAME_STRATEGY`, the target subject
  changes.
- **Schema caching** (`enableCaching`) does not refresh a cached `latest` for the
  client's lifetime — suited to tests with stable schemas.
- **`serialize()` returns a `Uint8Array`** (pass it straight to `produce`).

## Staying on the community extension

If you need behavior-identical, zero-change continuity, or the v2 surface,
`mostafa/xk6-kafka` continues independently — keep using it.
