## Why

`Writer` currently only constructs (scaffold). Producing messages is the core
load-testing capability and the first thing most users reach for. Building it on
the client/auth foundation (#9) makes the extension actually useful and unblocks
the producer compatibility scripts.

## What Changes

- `new Writer(WriterConfig)` builds a `twmb/franz-go` producer client via the
  shared client builder (brokers, SASL, TLS), with the default `topic`,
  `compression`, `balancer` partitioner, `requiredAcks`, `maxAttempts` (record
  and unknown-topic retries), `writeTimeout` (produce request timeout), batching
  (`batchBytes`, `batchTimeout`), and `autoCreateTopic`. `brokers` is required
  (construction errors when empty); `topic` is the default topic (optional when
  messages set their own).
- `writer.produce({ messages })` marshals each message — `key`/`value`
  (`string | Uint8Array` → bytes), `headers` (plain object), optional per-message
  `topic`, optional `time` — and produces them in the VU context (using the VU's
  context; rejected from init), returning after the batch is acknowledged (or
  erroring on failure).
- `writer.close()` flushes and closes the client.
- Accept-but-ignore / approximate options are honored as documented in
  `index.d.ts`: `batchSize` (ignored), `BALANCER_CRC32` and a custom balancer
  function (not honored yet), and `readTimeout` + `connectLogger` (ignored —
  franz-go manages read deadlines; the connection logger is not yet wired).
  `writeTimeout` maps to the produce request timeout (approximating the v1 socket
  write timeout).

## Capabilities

### New Capabilities

- `producer`: the `Writer` class producing messages to Kafka — client
  construction from `WriterConfig`, `produce`, and `close`.

### Modified Capabilities

<!-- None. `kafka-module` still holds (Writer exists and constructs); this adds
the producer behavior as a new capability. -->

## Impact

- New producer code in `pkg/kafka` (Writer client build, message marshaling,
  compression and balancer mapping) plus unit tests. Reuses the existing client
  builder from the auth change; no new dependencies.
- A produce-only integration test (and the existing integration CI job) exercise
  producing to a real broker. Full produce+consume compatibility scripts arrive
  with the consumer change.
- `index.d.ts`: doc remarks are clarified to match the franz-go behavior —
  `writeTimeout` (produce request timeout), `readTimeout` and `connectLogger`
  (accepted-but-ignored). `WriterConfig.topic` becomes optional (it is the
  default topic; per-message `topic` overrides it). Otherwise this implements
  part of the contract it already declares.
