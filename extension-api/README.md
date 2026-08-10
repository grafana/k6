# k6 JavaScript extension API

This is the in-repository form of the standalone extension API. It is a
separate Go module with no dependency on k6; its only non-standard-library
dependency is Sobek.

The initial API supports two extension shapes:

- a per-JavaScript-runtime `Module`, which receives `Context()` and
  `Runtime() *sobek.Runtime` through `VU` and returns `Exports`;
- a raw Go value, registered directly as the default JavaScript export.

```go
extensionapi.Register("k6/x/example", &RootModule{})
```

The k6 repository contains the host adapter. It translates this API into the
current resolver without exposing k6 types to extensions. Promise scheduling,
network policy, metrics, HTTP execution, and execution metadata are not part
of this initial API; they will be introduced as optional, host-neutral
capabilities when their contracts are designed.

Until this module moves to its own repository and has releases, local example
extensions use a `replace go.k6.io/k6-extension-api => ../../../extension-api`
directive for development only.
