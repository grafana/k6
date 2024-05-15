# xk6-output-opentelemetry

A work in progress k6 extension to output real-time test metrics in [OpenTelemetry metrics format](https://opentelemetry.io/docs/specs/otel/metrics/).

## Build

To build a `k6` binary with this extension, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git
- [xk6](https://github.com/grafana/xk6)

1. Build with `xk6`:

```bash
make build
```

This will result in a `k6` binary in the current directory.

2. Run with the just build `k6:

```bash
./k6 run -o xk6-opentelemetry <script.js>
```
