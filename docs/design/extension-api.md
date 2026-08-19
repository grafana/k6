# Standalone JavaScript extension API

## Status

This is an in-repository proof of concept for a future standalone Go module:
`go.k6.io/k6-extension-api`. The module imports only the Go standard library
and `github.com/grafana/sobek`; it does not import k6.

k6 owns the adapter from this API to its module resolver and legacy extension
registry. As a result, extensions use stable interfaces while k6 retains
ownership of its runtime, load-test state, metrics engine, network stack, and
automatic extension provisioning.

The current custom binary composition is in `cmd/k6-extension-api`. It declares
all migrated clones and is intentionally a nested module, so it does not alter
the root k6 module's vendored dependency set. It is built and run with an
isolated empty configuration. Its smoke test links every migrated extension and
directly exercises msgpack, SSH, Faker, TLS, Redis, and MQTT/TCP construction.

## Implemented v1 surface

```go
type Module interface {
	NewModuleInstance(VU) Instance
}

type Instance interface {
	Exports() Exports
}

type VU interface {
	Context() context.Context
	Runtime() *sobek.Runtime
}

type Exports struct {
	Default any
	Named   map[string]any
}

func Register(name string, module any)
```

`Register()` accepts either a per-runtime `Module` or a raw Go value exposed
as a shared default export. It requires the existing `k6/x/` import prefix.

The k6 adapter:

- adapts the small v1 `VU` to the legacy resolver's VU interface;
- resolves v1 modules as normal ESM/CommonJS Go modules;
- projects v1 registrations into `ext.Get()` and `ext.GetAll()`, which keeps
  usage reporting and automatic-extension-resolution aware of linked v1
  extensions;
- leaves legacy extensions unchanged.

The public `VU` interface will not be expanded with k6 types. New features
must be optional, host-neutral capability interfaces.

### Environment capability

`Environment` is the first optional capability:

```go
type Environment interface {
    LookupEnv(key string) (value string, ok bool)
}
```

An extension obtains it with `vu.(extensionapi.Environment)`. k6 adapts its
configured environment lookup into this capability without exposing `InitEnv`
or any other k6 type.

### Logger capability

`Logger` exposes the standard-library structured logger:

```go
type Logger interface {
    Logger() *slog.Logger
}
```

k6 adapts its current Logrus logger with a `slog.Handler`. Slog levels map to
the matching Logrus levels, and slog attributes and groups are flattened into
dot-separated Logrus fields. Extension authors therefore depend only on
`log/slog`, not Logrus.

### Network capability

`Network` is an optional capability for direct network extensions:

```go
type Network interface {
    DialContext(context.Context, network, address string) (net.Conn, error)
    LookupHost(context.Context, host string) ([]string, error)
}
```

k6 delegates both methods to its configured VU dialer and resolver. This
preserves k6 DNS behavior, hostname blocking, host overrides, and address
selection. The capability is unavailable in init context and returns
`ErrNetworkUnavailable`; extensions must not fall back to Go's default dialer
or resolver because that would bypass host policy.

### TLS capability

`TLS` is an optional capability for client-side TLS over a connection created
by `Network`:

```go
type TLS interface {
    TLSClient(context.Context, net.Conn, *tls.Config) (net.Conn, error)
}
```

k6 clones and retains ownership of its TLS configuration. The extension may
supply per-connection SNI (`ServerName`) and ALPN (`NextProtos`), custom root
certificates, and client certificates. Host policy remains authoritative for
verification, protocol versions, cipher suites, renegotiation, and key
logging. Custom extension roots replace the host root pool; client certificates
are additive. `TLSClient()` takes ownership of the raw connection and closes it
if the handshake fails or the context has already been cancelled.

### Promise and scheduler capabilities

`Scheduler` reserves one event-loop callback on the JavaScript runtime
goroutine and returns a thread-safe function that queues exactly one `Task`.
`Promises.NewPromise()` returns a Sobek promise with a resolver that safely
settles it from asynchronous Go code. The k6 adapter performs all Sobek promise
settlement on the owning event loop; extensions must never call Sobek from the
background goroutine that performs I/O.

### Metrics and tags capability

`Metrics` is an optional VU capability. It lets an extension declare a metric
with a portable kind and unit, obtain k6's `data_sent` and `data_received`
metrics, snapshot the VU's tags, and emit samples without importing k6's
metrics package:

```go
type Metrics interface {
    RegisterMetric(MetricSpec) (Metric, error)
    BuiltinMetric(BuiltinMetric) (Metric, bool)
    CurrentTags() Tags
    WithSystemTags(Tags, map[SystemTag]string) Tags
    Emit(context.Context, []Sample) error
}
```

`Tags` is immutable: the API copies both supplied and returned tag and metadata
maps. `WithSystemTags()` applies only system tags enabled by the host and keeps
non-indexable system tags as metadata. `Emit()` blocks only until the host
accepts the batch and returns `ctx.Err()` if cancelled, so background I/O can
stop without leaking a blocked goroutine. Metric handles are name-based but
the host rejects emission of a metric that has not been registered or supplied
as a supported built-in metric.

## Migrated

| Extension | Shape | Result |
| --- | --- | --- |
| `github.com/tango-tango/xk6-msgpack` | Per-VU module using Sobek `Runtime()` | Migrated. `pack()`/`unpack()` round-trip passes in the custom binary. |
| `github.com/grafana/xk6-ssh` | Raw shared Go default export | Migrated. The custom binary imports it and exposes `connect()`. |
| `github.com/grafana/xk6-faker` | Per-VU module using Sobek and `Environment` | Migrated. Its complete Go test suite passes. |
| `github.com/grafana/xk6-tls` | Per-VU module using `Network` and `Promises` | Migrated. Its complete Go test suite passes. |
| `github.com/grafana/xk6-disruptor` | Per-VU module using context and Sobek | Migrated. Its complete Go test suite passes. |
| `github.com/grafana/xk6-kubernetes` | Per-VU module using context and Sobek | Migrated. Its complete Go test suite passes. |
| `github.com/grafana/xk6-sql` | Per-VU module using context and Sobek | Migrated. Production packages build; its legacy test harness still imports the old k6 module test API. |
| `github.com/grafana/xk6-sql-driver-{azuresql,clickhouse,mysql,postgres,sqlserver}` | SQL driver registration modules | Migrated. Each repository builds. MySQL now uses `crypto/tls` version constants. |
| `github.com/grafana/xk6-mqtt` | Per-VU MQTT client using network, TLS, promises, logging, and metrics | Migrated for native MQTT/TLS brokers. Its complete Go test suite passes. MQTT over WebSocket remains deferred pending a host-aware WebSocket capability. |
| `github.com/grafana/xk6-tcp` | Per-VU TCP socket using network, TLS, promises, logging, and metrics | Migrated. Its complete Go test suite, including local integration tests, passes. |

The custom binary test script imports every migrated module except disruptor
and directly exercises msgpack, SSH, Faker, TLS, Redis, and MQTT/TCP module
construction. Disruptor initializes its Kubernetes
client at module-instantiation time and therefore requires a reachable cluster;
the binary links it but the no-external-service smoke test deliberately avoids
instantiating it. The script uses an isolated empty config file so it does not
depend on a user-level k6 configuration.

## Catalog migration assessment

The catalog was inspected from the current v1 and v2 registry files. The table
uses current default-branch source snapshots; a release migration should still
validate the release tag registered for that k6 catalog version.

| Extension | Assessment | Blocking capability or work |
| --- | --- | --- |
| `xk6-msgpack` | Migrated | None. |
| `xk6-ssh` | Migrated | None. Preserve its intentionally shared export. |
| `xk6-disruptor` | Migrated | Uses the standalone `common.Throw` helper. |
| `xk6-kubernetes` | Migrated | Uses the standalone `common.Throw` helper. |
| `xk6-sql` | Migrated | Its production code no longer needs k6; legacy tests need standalone test helpers. |
| `xk6-sql-driver-{azuresql,clickhouse,mysql,postgres,sqlserver}` | Migrated | Drivers are registration shims; MySQL uses `crypto/tls` constants. |
| `xk6-faker` | Migrated | Uses the optional `Environment` capability for `XK6_FAKER_SEED`. |
| `xk6-redis` | Migrated | Uses `Network`, `TLS`, and `Promises`; its tests use the standalone API test host. |
| `xk6-tls` | Migrated | Uses `Network` and `Promises`; no k6 dependency remains. |
| `xk6-kafka` | Can migrate after execution-state capability | Metrics/tags and built-in byte metrics are available; it still needs an active-vs-init state capability. |
| `xk6-dns` | Can migrate | Use `NetworkPolicy.CheckHost()` for the queried logical name and own its DNS packets, nameserver choice, and response parsing. |
| `xk6-icmp` | Can migrate | Use policy-aware `Network.LookupHost()` to resolve the target, then own ICMP packet-socket selection, privileges, and echo handling. |
| `xk6-mqtt` | Migrated for native MQTT/TLS | Uses network, TLS, promises, logger, and metrics/tags. MQTT-over-WebSocket awaits a host-aware WebSocket capability. |
| `xk6-tcp` | Migrated | Uses network, TLS, promises, logger, and metrics/tags. |
| `xk6-loki` | Deferred | Logger is available; needs a k6-aware HTTP executor, current tags, VU ID, and metrics. |
| `xk6-client-prometheus-remote` | Deferred | Needs a k6-aware HTTP executor preserving transport, options, tags, and HTTP metrics. |
| `xk6-sse` | Deferred | Needs HTTP transport/options/cookies plus HTTP/custom metrics and tags. |
| `xk6-client-tracing` | Not currently actionable | V1-only catalog entry; its registered GitHub repository was unavailable during the inventory. Restore or identify its authoritative source before migration. |

## Deferred capability design

The following capabilities are intentionally not in the base API:

1. **HTTP execution**: a host executor with portable request/response types,
   preserving k6 transport settings and built-in HTTP metrics when k6 is the
   host.
2. **Small metadata services**: VU identity and active-vs-init execution
   state. Environment lookup and structured logging are now available.

Each capability should be designed against at least one migrated extension and
one test fixture before being released. This keeps extensions independent from
k6's broad dependency graph while allowing later API versions to add
capabilities without breaking the small base contract.

## Next capability proposal

The execution-context capability below remains a proposal. It should be added
only after its host adapter and standalone test-host implementation have been
reviewed.

### Execution context (Kafka)

Kafka only needs to distinguish an executable VU context from module
initialization. It must not receive the host's scenario, VU, or executor
objects. The proposed optional capability is:

```go
type ExecutionContext uint8

const (
    ExecutionContextInit ExecutionContext = iota
    ExecutionContextVU
)

type Execution interface {
    ExecutionContext() ExecutionContext
}
```

Kafka uses this before its synchronous compatibility `produce()` and
`consume()` calls, replacing its `vu.State() == nil` check. Its existing
network, TLS, promise, logger, and metrics requirements are already covered.

## Protocol networking building block

### Advisory network policy (DNS and protocol-specific extensions)

Extensions sometimes need to perform an operation whose logical target is not
the address they connect to. For example, a DNS extension queries an explicit
nameserver but must check the queried hostname first. The optional
`NetworkPolicy` capability supplies that building block without giving an
extension any k6 network types:

```go
type NetworkPolicy interface {
    CheckHost(context.Context, host string) error
}
```

`CheckHost()` checks only the host's hostname-blocking policy: it does not
resolve the name or open a connection. It is intentionally advisory. An
extension owns its protocol implementation and is responsible for applying the
check before its operation; a host cannot reliably enforce every operation an
extension can implement directly.

`Network.LookupHost()` remains the appropriate building block when an
extension needs a host-policy-aware selected address. `xk6-icmp` can use it
before its own `icmp.ListenPacket()` and packet handling, retaining the
extension's platform-specific privileged/unprivileged fallback. `xk6-dns` can
call `CheckHost()` for the DNS question, then retain ownership of DNS packet
construction, transport, nameserver choice, retries, and response parsing.

## Temporary compatibility helper

`go.k6.io/k6-extension-api/common.Throw(runtime, err)` is available for the
small set of extensions that synchronously turn a Go error into a JavaScript
exception. It preserves a supplied Sobek exception and otherwise uses
`Runtime.NewGoError()`, retaining the JavaScript stack. The helper depends only
on Sobek, does not expose k6 types, and is deliberately separate from the base
module interface.
