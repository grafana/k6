# xk6-redis

k6 extension providing a Redis client for load tests. Wraps go-redis v9 and supports single-node, cluster, and sentinel deployments. All commands return JS promises.

## Architecture

The extension follows k6's standard module pattern: a root module creates per-VU instances, each owning a Redis client.

**Lazy connection**: The client stores options at construction time but does not connect until the first command executes inside a VU context. This satisfies k6's convention of no IO during init. Every command method checks `connect()` before proceeding.

**Promise pattern**: Every Redis command creates a JS promise, validates arguments, spawns a goroutine for the actual Redis call, and resolves or rejects the promise from that goroutine. This pattern is uniform across all 40+ command methods.

**Options parsing**: The constructor accepts either a URL string (single-node) or a JS object. The object form determines the client type: if `masterName` is present, sentinel/failover is used; if `cluster` is present, a cluster client is used; otherwise, single-node. In cluster mode, all nodes must share consistent options (credentials, timeouts, pool sizes) or construction fails.

**TLS merging**: When TLS is configured, the extension merges the user-provided TLS config with k6's VU-level TLS settings (cipher suites, min/max version, client certificates, renegotiation, key logging). It also replaces the default dialer with k6's network dialer to preserve blocked-host and DNS resolution features, manually upgrading the connection to TLS. Root CA merging is explicitly marked as a TODO and not yet implemented.

**Type validation**: Before sending values to Redis, the client validates that arguments are of supported Go types (string, numeric, bool). Validation errors report the argument's position in the outer command call using a position offset.

## Gotchas

- Cluster node option consistency is enforced for most fields, but TLS config is only taken from the first node that provides it. If different nodes specify different TLS configs, only the first one wins silently.

- Tests use a custom in-process RESP protocol stub server over TCP, not a real Redis. The stub auto-ignores CLIENT commands. If your test relies on CLIENT command behavior, you must register a handler explicitly.

- The linter config is not committed. It is downloaded from k6 core's master branch on first lint run. Do not commit it.

- The underlying go-redis client is instantiated once per VU on first use. go-redis manages its own connection pool and reconnection internally, but the k6-level client wrapper has no mechanism to re-apply VU-level config changes (TLS, dialer) after initial setup.
