## 1. Producer client

- [x] 1.1 Extend the shared client options with producer opts: default topic, compression, partitioner (balancer), requiredAcks, `maxAttempts` (both record-retries and unknown-topic-retries), `writeTimeout` (produce request timeout), `batchBytes`, `batchTimeout` (linger), `autoCreateTopic`
- [x] 1.2 Map `compression` (gzip/snappy/lz4/zstd) to franz-go codecs; unit-test
- [x] 1.3 Map `balancer` (round-robin, hash, murmur2, least-bytes) to partitioners; `BALANCER_CRC32` / custom fn fall back to the default; unit-test
- [x] 1.4 Map `requiredAcks` (-1/0/1); unit-test

## 2. Message marshaling

- [x] 2.1 Marshal a message to a record: `key`/`value` (string → UTF-8 bytes, `Uint8Array` → bytes), `headers` (object), per-message `topic`, `time`
- [x] 2.2 Unit-test marshaling (string and byte-array key/value, headers, topic override)

## 3. Writer

- [x] 3.1 Implement `new Writer(WriterConfig)` building the producer client; require `brokers` (error if empty); accepted-but-ignored options (`batchSize`, `readTimeout`, `connectLogger`, `BALANCER_CRC32`/custom balancer) do not error
- [x] 3.2 Implement `writer.produce({ messages })` via `ProduceSync` using the VU context; block until acknowledged; throw on error; reject calls from the init context
- [x] 3.3 Implement `writer.close()` (flush + close)
- [x] 3.4 Unit-test construction (incl. brokers-required), the close/method exposure

## 4. Integration

- [x] 4.1 Add an integration test (k6 script): construct a Writer with `autoCreateTopic`, produce messages, assert no error; skip when `KAFKA_BROKER` is unset
      (The deterministic produce-error integration is deferred to the admin change, where topic state is controllable; the broker default auto-creates topics.)

## 5. Validate

- [x] 5.1 `go test ./...`, `gosec ./...`, `make lint` (golangci-lint), `xk6 lint`, `xk6 build` (`CGO_ENABLED=0`), and `make it` (`xk6 test`; skips without `KAFKA_BROKER`) pass
- [x] 5.2 Run `openspec validate add-producer --strict` and fix any issues
