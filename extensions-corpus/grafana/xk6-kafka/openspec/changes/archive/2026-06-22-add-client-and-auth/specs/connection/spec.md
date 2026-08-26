## ADDED Requirements

### Requirement: Connection establishes an authenticated connection

`new Connection(connectionConfig)` SHALL build a `twmb/franz-go` client from the
config's `address`, `sasl`, and `tls` (via the shared client builder) and
establish connectivity to the cluster. Construction SHALL fail with an error if
the cluster is unreachable. (Authentication and TLS errors also surface as
construction errors at runtime, but verifying that against a real broker is a
follow-up integration case; this change verifies the connectivity path and that
the configured auth is passed to the client.)

#### Scenario: Connects with valid config

- **WHEN** `new Connection({ address: "<broker>" })` runs against a reachable broker
- **THEN** the connection is established without error

#### Scenario: Configured auth is passed to the client

- **WHEN** `new Connection({ address, sasl, tls })` is given SASL and TLS settings
- **THEN** the underlying franz-go client is built (via the shared client
  builder) with those SASL and TLS settings applied

  Note: the SASL/TLS mapping itself is verified by the `auth` unit tests; a live
  authenticated/TLS handshake against a broker is a follow-up integration case.

### Requirement: Connection close releases resources

`Connection.close()` SHALL close the underlying client and release its
connections. After close, the connection is no longer usable.

#### Scenario: Close releases the client

- **WHEN** `close()` is called on a connection
- **THEN** the underlying franz-go client is closed and its resources released
