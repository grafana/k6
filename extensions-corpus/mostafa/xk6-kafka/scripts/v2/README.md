# v2 Script Suite

These scripts exercise the `v2.0.0` JavaScript surface with the new constructors:

- `Producer`
- `Consumer`
- `AdminClient`

The suite is organized as:

- `smoke/`: fast single-run checks for core topic lifecycle and produce/consume flow
- `integration/`: richer flows for consumer groups and Schema Registry happy paths

Reference:

- v2 API docs: [`api-docs/v2/docs/README.md`](../../api-docs/v2/docs/README.md)

## Local Environment

These scripts assume a local Kafka + Schema Registry environment, such as the existing Lenses.io Fast Data Dev container.

Environment variables:

- `XK6_KAFKA_BROKERS`: comma-separated bootstrap servers. Default: `localhost:9092`
- `XK6_KAFKA_SCHEMA_REGISTRY_URL`: Schema Registry base URL. Default: `http://localhost:8081`
- `XK6_KAFKA_RUN_ID`: optional stable suffix for topic naming. Defaults to `Date.now()`

Examples:

```bash
./k6 run scripts/v2/smoke/basic.js
./k6 run scripts/v2/integration/consumer_group.js
./k6 run scripts/v2/integration/avro_schema_registry.js
./k6 run scripts/v2/integration/json_schema_registry.js
```

Each script creates a deterministic per-run topic name, validates the expected flow, and deletes the topic in `teardown()`.
