# Standalone JavaScript extension API

## Status

This is an in-repository proof of concept for a future standalone Go module:
`go.k6.io/k6-extension-api`. The module imports only the Go standard library
and `github.com/grafana/sobek`; it does not import k6.

k6 owns the adapter from this API to its module resolver and legacy extension
registry. As a result, extensions use stable interfaces while k6 retains
ownership of its runtime, load-test state, metrics engine, network stack, and
automatic extension provisioning.

The current custom binary composition is in `cmd/k6-extension-api`. It links
the two migrated extension clones and is intentionally a nested module, so it
does not alter the root k6 module's vendored dependency set.

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

## Migrated and verified

| Extension | Shape | Result |
| --- | --- | --- |
| `github.com/tango-tango/xk6-msgpack` | Per-VU module using Sobek `Runtime()` | Migrated. `pack()`/`unpack()` round-trip passes in the custom binary. |
| `github.com/grafana/xk6-ssh` | Raw shared Go default export | Migrated. The custom binary imports it and exposes `connect()`. |

The custom binary test script imports both modules. It uses an isolated empty
config file so it does not depend on a user-level k6 configuration.

## Catalog migration assessment

The catalog was inspected from the current v1 and v2 registry files. The table
uses current default-branch source snapshots; a release migration should still
validate the release tag registered for that k6 catalog version.

| Extension | Assessment | Blocking capability or work |
| --- | --- | --- |
| `xk6-msgpack` | Migrated | None. |
| `xk6-ssh` | Migrated | None. Preserve its intentionally shared export. |
| `xk6-disruptor` | Can migrate now | Replace `common.Throw` with a Sobek-native error helper built on `Runtime()`. |
| `xk6-kubernetes` | Can migrate now | Same Sobek error conversion; all execution uses `Context()`. |
| `xk6-sql` | Can migrate now | Uses `Context()` plus direct Sobek values/symbols. |
| `xk6-sql-driver-{azuresql,clickhouse,mysql,postgres,sqlserver}` | Can migrate with `xk6-sql` | Drivers are registration shims. MySQL must replace four `netext` TLS-version constants. |
| `xk6-faker` | Deferred | Needs a host-neutral environment lookup capability for `XK6_FAKER_SEED`. |
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
