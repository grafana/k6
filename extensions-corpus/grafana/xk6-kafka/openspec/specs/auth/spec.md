# auth Specification

## Purpose
TBD - created by archiving change add-client-and-auth. Update Purpose after archive.
## Requirements
### Requirement: SASL mechanism selection

The client builder SHALL select the `twmb/franz-go` SASL mechanism that matches
`SASLConfig.algorithm`: `sasl_plain` → PLAIN, `sasl_scram_sha256` → SCRAM-SHA-256,
`sasl_scram_sha512` → SCRAM-SHA-512. `sasl_aws_iam` is deferred to a dedicated
change and SHALL return a "not yet implemented" error for now. `sasl_ssl` SHALL
use the PLAIN mechanism with the configured
credentials and SHALL require TLS to be enabled — building a client with
`sasl_ssl` while TLS is not enabled SHALL fail with an error (matching v1
behavior). When the algorithm is `none` or the SASL config is absent, no SASL
mechanism SHALL be configured. `username` and `password` from the config SHALL be
used as the mechanism credentials where applicable.

#### Scenario: SCRAM mechanism is selected

- **WHEN** a client is built with `algorithm` `sasl_scram_sha512`, a username, and a password
- **THEN** the client is configured with the SCRAM-SHA-512 mechanism carrying those credentials

#### Scenario: sasl_ssl uses PLAIN and requires TLS

- **WHEN** a client is built with `algorithm` `sasl_ssl`
- **THEN** the PLAIN mechanism is used, and the build fails with an error if TLS is not enabled

#### Scenario: No SASL when unset

- **WHEN** a client is built with no SASL config (or `algorithm` `none`)
- **THEN** the client is configured without any SASL mechanism

#### Scenario: AWS IAM is deferred

- **WHEN** a client is built with `algorithm` `sasl_aws_iam`
- **THEN** it returns a "not yet implemented" error (AWS IAM lands in a dedicated change)

### Requirement: TLS configuration

The client builder SHALL build a `*tls.Config` from `TLSConfig` when TLS is
enabled: honoring `minVersion`, loading the client certificate and key PEM for
mutual TLS, trusting the server CA PEM, and setting insecure skip-verify when
`insecureSkipTlsVerify` is true. When TLS is not enabled, no TLS SHALL be
configured.

#### Scenario: Mutual TLS from PEM

- **WHEN** a client is built with `enableTls` true and client cert/key and server CA PEM
- **THEN** the resulting TLS config presents the client certificate and trusts the server CA

#### Scenario: Minimum version is applied

- **WHEN** `minVersion` is `tlsv1.3`
- **THEN** the TLS config's minimum version is TLS 1.3

#### Scenario: No TLS when disabled

- **WHEN** TLS is not enabled
- **THEN** the client is configured without TLS

### Requirement: JKS loading

`LoadJKS` SHALL load a Java KeyStore from the path in `JKSConfig` and return the
client certificate chain, client key, and server CA as PEM strings. Only the JKS
format is supported; a PKCS#12 keystore SHALL be rejected with an error.

`LoadJKS` is an init-context operation: it SHALL read the keystore through k6's
init-environment filesystem (`InitEnv().FileSystems["file"]`), not the host OS
filesystem directly, so paths resolve correctly inside k6 archives and bundles.
It SHALL error if called outside the init context.

#### Scenario: Load returns PEM material

- **WHEN** `LoadJKS` is called in the init context with a valid JKS path, password, and aliases
- **THEN** it reads the keystore via the init-environment filesystem and returns `clientCertsPem`, `clientKeyPem`, and `serverCaPem` in PEM format

#### Scenario: Errors outside init context

- **WHEN** `LoadJKS` is called from VU (non-init) context
- **THEN** it fails with an error rather than reading from the host filesystem

#### Scenario: PKCS#12 is rejected

- **WHEN** `LoadJKS` is called on a PKCS#12 keystore
- **THEN** it fails with an error rather than returning material

### Requirement: Shared client builder

The extension SHALL provide a single internal helper that turns broker
addresses plus optional `SASLConfig` and `TLSConfig` into `twmb/franz-go` client
options, so producer, consumer, and admin construct clients consistently.

#### Scenario: Brokers are configured

- **WHEN** a client is built with a list of broker addresses
- **THEN** the franz-go client targets those seed brokers

