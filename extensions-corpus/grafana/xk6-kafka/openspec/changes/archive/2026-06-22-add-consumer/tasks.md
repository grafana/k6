## 1. Reader client

- [x] 1.1 Build consumer options for group mode (`groupID`, `groupTopics`/`topic`, balancers, heartbeat/session/rebalance timeouts, commit interval) and direct mode (single partition of `topic`; `partition` defaults to 0; `offset`/`startOffset`)
- [x] 1.2 Map `minBytes`, `maxBytes`, `maxWait`, `isolationLevel`, `maxAttempts`; require `brokers` (error if empty)
- [x] 1.3 Map `groupBalancers` (range, round-robin); `GROUP_BALANCER_RACK_AFFINITY` falls back to a supported balancer
- [x] 1.4 Unit-test option/balancer/offset mapping and brokers validation

## 2. Message decode

- [x] 2.1 Decode a record to a `Message`: `topic`, `partition`, `offset`, `highWaterMark`, `key`/`value` (`Uint8Array`), `headers` (object), `time` (RFC3339; RFC3339Nano when `nanoPrecision`)
- [x] 2.2 Unit-test decoding (key/value/headers, time formatting with and without nanoPrecision)

## 3. Reader

- [x] 3.1 Implement `new Reader(ReaderConfig)`; accepted-but-ignored options (`queueCapacity`, `readBatchTimeout`, `readLagInterval`, `partitionWatchInterval`, `watchPartitionChanges`, `joinGroupBackoff`, `retentionTime`, `readBackoffMin`/`Max`, `connectLogger`) do not error
- [x] 3.2 Implement `reader.consume({ limit, nanoPrecision, expectTimeout })` via `PollRecords` in the VU context, bounded by `maxWait`; reject init context and calls after close
- [x] 3.3 Implement `reader.close()`
- [x] 3.4 Unit-test construction, method exposure, and the init/closed guards

## 4. Integration

- [x] 4.1 Add a produce + consume round-trip integration test (k6 script): produce a string value, consume it, assert the consumed value matches; skip when `KAFKA_BROKER` is unset. (Community compat-script ports use Schema Registry serdes and land with the schema-registry change.)

## 5. Contract

- [x] 5.1 Update `index.d.ts` docs: `Reader.consume` → "VU context" + timeout semantics (default throws on `maxWait`; `expectTimeout` returns partial/empty); `topic` may also be the group's topic when `groupID` is set; `startOffset` applies to direct readers too (and `offset` takes precedence); `readBackoffMin`/`readBackoffMax` reclassified accepted-but-ignored; `connectLogger` annotated accepted-but-ignored

## 6. Validate

- [x] 6.1 `go test ./...`, `gosec ./...`, `make lint` (golangci-lint), `xk6 lint`, `xk6 build` (`CGO_ENABLED=0`), and `make it` (`xk6 test`; skips without `KAFKA_BROKER`) pass
- [x] 6.2 Run `openspec validate add-consumer --strict` and fix any issues
