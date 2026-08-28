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
the root k6 module's vendored dependency set. The initial binary was built and
run with msgpack and SSH; rebuilding the expanded composition requires the Go
checksum database entry for Kubernetes dependencies.

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

## Migrated

| Extension | Shape | Result |
| --- | --- | --- |
| `github.com/tango-tango/xk6-msgpack` | Per-VU module using Sobek `Runtime()` | Migrated. `pack()`/`unpack()` round-trip passes in the custom binary. |
| `github.com/grafana/xk6-ssh` | Raw shared Go default export | Migrated. The custom binary imports it and exposes `connect()`. |
| `github.com/grafana/xk6-faker` | Per-VU module using Sobek and `Environment` | Migrated. Its complete Go test suite passes. |
| `github.com/grafana/xk6-disruptor` | Per-VU module using context and Sobek | Migrated. Its complete Go test suite passes. |
| `github.com/grafana/xk6-kubernetes` | Per-VU module using context and Sobek | Migrated. Its complete Go test suite passes. |
| `github.com/grafana/xk6-sql` | Per-VU module using context and Sobek | Migrated. Production packages build; its legacy test harness still imports the old k6 module test API. |
| `github.com/grafana/xk6-sql-driver-{azuresql,clickhouse,mysql,postgres,sqlserver}` | SQL driver registration modules | Migrated. Each repository builds. MySQL now uses `crypto/tls` version constants. |

The custom binary test script imports every migrated module except disruptor
and directly exercises msgpack and SSH. Disruptor initializes its Kubernetes
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
| `xk6-redis` | Deferred | Needs a promise/event-loop bridge, active-vs-init state, and host dialer/TLS policy. |
| `xk6-tls` | Deferred | Needs promise settlement on the JS event loop and a host dialer. |
| `xk6-kafka` | Deferred | Needs metrics declaration/emission, current tags, built-in byte metrics, and active-vs-init state. |
| `xk6-dns` | Deferred | Needs promises, event-loop scheduling, DNS/dial hostname policy, and custom metrics. |
| `xk6-icmp` | Deferred | Needs promises, callback scheduling, resolver, logger, environment lookup, and metrics. |
| `xk6-mqtt` | Deferred | Needs promises/callback scheduling, resolver/TLS, logger, and metrics. |
| `xk6-tcp` | Deferred | Needs promises/callback scheduling, resolver/dialer/TLS, logger, and metrics/tags. |
| `xk6-loki` | Deferred | Needs a k6-aware HTTP executor, current tags, VU ID, logger, and metrics. |
| `xk6-client-prometheus-remote` | Deferred | Needs a k6-aware HTTP executor preserving transport, options, tags, and HTTP metrics. |
| `xk6-sse` | Deferred | Needs HTTP transport/options/cookies plus HTTP/custom metrics and tags. |
| `xk6-client-tracing` | Not currently actionable | V1-only catalog entry; its registered GitHub repository was unavailable during the inventory. Restore or identify its authoritative source before migration. |

## Deferred capability design

The following capabilities are intentionally not in the base API:

1. **JS scheduler and promises**: an API-owned, thread-safe way for a
   goroutine to settle a Sobek promise or enqueue JavaScript work on the owning
   event loop.
2. **Network policy**: resolver, cancellation-aware dialer, hostname policy,
   and optional TLS configuration supplied by the host.
3. **Metrics and tags**: portable metric definitions, immutable current-tag
   snapshots, and cancellation-aware sample emission. Concrete k6 metrics
   types must not cross the API boundary.
4. **HTTP execution**: a host executor with portable request/response types,
   preserving k6 transport settings and built-in HTTP metrics when k6 is the
   host.
5. **Small metadata services**: environment lookup, structured logging, VU
   identity, and active-vs-init execution state.

Each capability should be designed against at least one migrated extension and
one test fixture before being released. This keeps extensions independent from
k6's broad dependency graph while allowing later API versions to add
capabilities without breaking the small base contract.

## Temporary compatibility helper

`go.k6.io/k6-extension-api/common.Throw(runtime, err)` is available for the
small set of extensions that synchronously turn a Go error into a JavaScript
exception. It preserves a supplied Sobek exception and otherwise uses
`Runtime.NewGoError()`, retaining the JavaScript stack. The helper depends only
on Sobek, does not expose k6 types, and is deliberately separate from the base
module interface.
