# xk6-kafka

> [!NOTE]
> **Pre-1.0 (`v0.x`).** The API targets the community [`mostafa/xk6-kafka`](https://github.com/mostafa/xk6-kafka) v1 surface and is stabilizing; minor changes are possible before `v1.0.0`.

`grafana/xk6-kafka` is the official, Grafana-owned, pure-Go [k6 extension](https://grafana.com/docs/k6/latest/extensions/) for load testing [Apache Kafka](https://kafka.apache.org): producing and consuming messages, managing topics, authenticating, and working with Schema Registry.

It is designed as a **100% pure-Go** extension (`CGO_ENABLED=0`): no C toolchain, no `confluentinc/librdkafka`, so it can run in lightweight containers, strict CI/CD pipelines, and Grafana Cloud k6.

## Goals

- **Official Grafana support** — owned and maintained by Grafana, with a clear path for security patches and releases.
- **Pure Go** — compiles without CGO, so it fits scratch containers, cross-compilation, and pure-Go build pipelines.
- **Familiar API** — aims to be a near-drop-in replacement for community [`mostafa/xk6-kafka`](https://github.com/mostafa/xk6-kafka) v1 scripts: same import and API shape, so common producer, consumer, admin, auth, and Schema Registry scripts run with little or no change.

See [RATIONALE.md](RATIONALE.md) for the problem, goals, and scope, the [migration guide](docs/MIGRATION.md) to port a community `mostafa/xk6-kafka` v1 script, and the [compatibility matrix](docs/COMPATIBILITY.md) for the full feature list.

## Build

Use [xk6](https://github.com/grafana/xk6) to build a k6 binary with the extension:

```bash
xk6 build --with github.com/grafana/xk6-kafka
```

This produces a `k6` binary in the current directory. No C toolchain required. This builds from the latest `main`; to pin a released version, append `@vX.Y.Z` (see [Releases](#releases)).

## Testing

Unit tests need no broker:

```bash
make test
```

The integration tests require a real Kafka broker. The easiest way is `make integration`, which starts a single-node Kafka (KRaft) via [`compose.yaml`](compose.yaml), runs the tests against it, and tears it down:

```bash
make integration
```

To keep the broker running between runs (e.g. while iterating), start it once and point the tests at it:

```bash
make broker-up
KAFKA_BROKER=localhost:9092 KAFKA_SASL_BROKER=localhost:9094 make it
make broker-down
```

`make it` fails if `KAFKA_BROKER` or `KAFKA_SASL_BROKER` is unset — the integration tests (including the SASL auth test) never skip silently. `compose.yaml` exposes a plaintext listener on `9092` and a SASL_PLAINTEXT listener on `9094`; the same broker is used in CI, so local and CI runs match.

## Usage

> [!NOTE]
> The example uses the v1-compatible API; minor changes are possible before `v1.0.0`.

```javascript
import { Writer, Reader } from "k6/x/kafka";

const brokers = ["localhost:9092"];
const topic = "my-topic";

const writer = new Writer({ brokers, topic, autoCreateTopic: true });
const reader = new Reader({ brokers, topic });

export default function () {
  writer.produce({
    // strings are sent as UTF-8 bytes; pass a Uint8Array for raw bytes
    messages: [{ key: "key", value: "value" }],
  });

  const messages = reader.consume({ limit: 10 });
  console.log(messages);
}

export function teardown() {
  writer.close();
  reader.close();
}
```

Run it with the binary you built:

```bash
./k6 run script.js
```

## Schema Registry

The extension supports [Confluent Schema Registry](https://docs.confluent.io/platform/current/schema-registry/) for schema management and serialization:

```javascript
import { Writer, SchemaRegistry, SCHEMA_TYPE_AVRO } from "k6/x/kafka";

const sr = new SchemaRegistry({ url: "http://localhost:8081" });

// Register or load a schema
const schema = sr.createSchema({
  subject: "my-topic-value",
  schema: '{"type":"record","name":"User","fields":[{"name":"id","type":"int"},{"name":"name","type":"string"}]}',
  schemaType: SCHEMA_TYPE_AVRO,
});

// Produce with schema
const writer = new Writer({ brokers: ["localhost:9092"], topic: "my-topic" });
writer.produce({
  messages: [{
    value: sr.serialize({
      data: { id: 1, name: "Alice" },
      schemaType: SCHEMA_TYPE_AVRO,
      schema: schema,
    }),
  }],
});
```

Supports Avro and JSON schemas via Confluent wire format. Standalone mode (no registry) is also supported for inline schemas. Protobuf serdes is not supported in v1 (see the limitations below).

## Metrics

The `Writer` and `Reader` emit custom k6 metrics into the end-of-test summary,
using the community `mostafa/xk6-kafka` v1 names, so you can assert on them in
`thresholds`. Metrics franz-go attributes to a topic (message counts/bytes, lag,
offset, and the per-topic batch/fetch metrics) carry a `topic` tag. The
broker-request-level timings (`*_dial_*`, `*_write_seconds`, `*_read_seconds`,
`*_wait_seconds`) are untagged, because one request batches many topics.

```javascript
export const options = {
  thresholds: {
    kafka_writer_error_count: ["count==0"],
    "kafka_reader_message_count{topic:my-topic}": ["count>0"],
  },
};
```

**Writer:** `kafka_writer_message_count`, `kafka_writer_message_bytes`,
`kafka_writer_write_count`, `kafka_writer_error_count`,
`kafka_writer_batch_size`, `kafka_writer_batch_bytes`,
`kafka_writer_write_seconds`, `kafka_writer_wait_seconds`,
`kafka_writer_dial_count`, `kafka_writer_dial_seconds`.

**Reader:** `kafka_reader_message_count`, `kafka_reader_message_bytes`,
`kafka_reader_fetches_count`, `kafka_reader_error_count`,
`kafka_reader_timeouts_count`, `kafka_reader_lag`, `kafka_reader_offset`,
`kafka_reader_fetch_size`, `kafka_reader_fetch_bytes`,
`kafka_reader_read_seconds`, `kafka_reader_wait_seconds`,
`kafka_reader_dial_count`, `kafka_reader_dial_seconds`.

Message counts, bytes, lag, and offset are exact (measured at produce/consume);
the hook-derived trends (batch/fetch sizes and the `*_seconds` timings) are
franz-go-derived and batch-granular, so they approximate — rather than exactly
reproduce — the community's `segmentio/kafka-go`-derived values.

**Not emitted** (no franz-go source; present in the community extension):
`kafka_writer_retries_count`, `kafka_writer_batch_seconds`,
`kafka_reader_rebalance_count`, `kafka_reader_queue_length`,
`kafka_reader_queue_capacity`.

## Compatibility

This extension aims for **familiarity, not a guarantee**: most community v1 scripts are expected to run with little or no change, but identical behavior is not promised. Some legacy tuning options have no pure-Go equivalent and are accepted but ignored, so behavior can differ in edge cases. Users who need behavior-identical, zero-change continuity should stay on `mostafa/xk6-kafka`.

See the **[compatibility matrix](docs/COMPATIBILITY.md)** for the full list of supported, divergent, and unsupported features, and the **[migration guide](docs/MIGRATION.md)** for porting a community v1 script. Highlights of the Schema Registry gaps:

### Schema Registry Limitations (v1)

The Schema Registry implementation focuses on the core serdes workflows and does not yet include:

- **Caching**: set `enableCaching: true` on the `SchemaRegistry` config to cache resolved schemas, so repeated `getSchema` of the same subject/version skip the registry (off by default). A cached `latest` is not refreshed for the client's lifetime — a schema evolved mid-run is not observed and can surface as a `deserialize` schema-id mismatch, so caching suits tests with stable schemas. Parsed Avro schemas are always reused (no re-parse per serdes call), regardless of the flag. The per-schema `Schema.enableCaching` field is accepted but ignored; caching is controlled at the client level.
- **Complex schema references**: multi-schema compositions (imports for Protobuf, `$ref` for JSON Schema) are not supported.
- **Protobuf**: only Avro and JSON are fully supported; Protobuf serdes is not yet implemented.
- **JSON validation**: only checks `required` fields; type mismatches and other schema violations may not be caught.
- **Subject-name strategies**: `TOPIC_NAME_STRATEGY` works for any serdes. `RECORD_NAME_STRATEGY` and `TOPIC_RECORD_NAME_STRATEGY` derive the record name from an **Avro** named schema (record/enum/fixed); JSON Schema and Protobuf record naming are not supported and return an error. An unknown strategy also errors. **Migration note:** earlier builds silently treated the record strategies as `TOPIC_NAME_STRATEGY` (`{topic}-{element}`); a script relying on that now gets the real record-name subject (or an error), which can change the registry subject it targets.

These may be added in future releases based on demand.

## Releases

Releases are published as [GitHub Releases](https://github.com/grafana/xk6-kafka/releases) with auto-generated notes; there is no `CHANGELOG` file. Notes are grouped by PR label (Features, Fixes, Security, …) where the merged PRs are labeled; unlabeled PRs are listed under "Other Changes". Tags follow [semantic versioning](https://semver.org) (`vX.Y.Z`); a pre-release suffix (e.g. `v1.2.3-rc.1`) is marked as a prerelease.

Once a release exists, pin it when building:

```bash
xk6 build --with github.com/grafana/xk6-kafka@v1.2.3
```

## Acknowledgments

This extension's API and design draw on the community [`mostafa/xk6-kafka`](https://github.com/mostafa/xk6-kafka) project by [Mostafa Moradian](https://github.com/mostafa). Thanks to Mostafa and its contributors for years of maintaining Kafka load testing in k6 and shaping the API that users know today.

This is not a takeover of the community extension, which continues independently. The aim is a separate, pure-Go reimplementation that Grafana can officially support and offer in Grafana Cloud k6. Users happy with `mostafa/xk6-kafka` can keep using it.

## License

Licensed under the [AGPL-3.0](LICENSE) license, the same license as [k6](https://github.com/grafana/k6).
