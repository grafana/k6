## 1. TLS

- [x] 1.1 Build a `*tls.Config` from `TLSConfig` (enable, `minVersion`, client cert/key PEM, server CA PEM, insecure skip-verify)
- [x] 1.2 Unit-test TLS config building (mutual TLS from PEM, min version, disabled case)

## 2. SASL

- [x] 2.1 Map `algorithm` to franz-go SASL mechanisms (plain, scram-256, scram-512; `sasl_ssl` → PLAIN and require TLS enabled, error otherwise; `none`/unset → none) using `username`/`password`. `sasl_aws_iam` is deferred — return a "not yet implemented" error (no AWS SDK dependency)
- [x] 2.2 Unit-test mechanism selection per `algorithm`, including the no-SASL case

## 3. JKS

- [x] 3.1 Implement `LoadJKS` as an init-context op: read the keystore via k6's `InitEnv().FileSystems["file"]` (not host `os`), parse it (pure Go), return `clientCertsPem`/`clientKeyPem`/`serverCaPem`; reject PKCS#12; error if called outside init context
- [x] 3.2 Unit-test `LoadJKS` against a test keystore fixture and the PKCS#12 rejection

## 4. Client builder

- [x] 4.1 Add the shared builder: brokers + `SASLConfig` + `TLSConfig` → franz-go client options
- [x] 4.2 Unit-test that brokers, SASL, and TLS options are applied (and omitted when unset)

## 5. Connection

- [x] 5.1 Make `new Connection(connectionConfig)` build a client via the builder (with configured `sasl`/`tls`) and verify connectivity (eager connect); fail on unreachable cluster
- [x] 5.2 Implement `Connection.close()` to close the client and release resources
- [x] 5.3 Add integration tests (k6 script via `xk6 test` / `make it`): connect plaintext and close; skip only when no broker address is configured (local/dev), and fail (not skip) when an address is configured but unreachable

## 6. Integration CI

- [x] 6.1 Add a CI workflow that starts a Kafka service container, configures the broker address to point at it, and runs the integration tests (`make it`) against it — failing the gate if the service is unreachable or the tests fail

## 7. Validate

- [x] 7.1 `go test ./...`, `make lint`, and `xk6 build` (`CGO_ENABLED=0`) pass
- [x] 7.2 Run `openspec validate add-client-and-auth --strict` and fix any issues
