# xk6-output-opentelemetry

A work in progress k6 extension to output real-time test metrics in [OpenTelemetry metrics format](https://opentelemetry.io/docs/specs/otel/metrics/).

> [!WARNING]  
> It's work in progress implementation and not ready for production use.

Configuration options (currently environment variables only):

* `K6_OTEL_RECEIVER_TYPE` - OpenTelemetry receiver type, currently only `grpc` is supported. Default is `grpc`.
* `K6_OTEL_RECEIVER_ENDPOINT` - OpenTelemetry receiver endpoint. Default is `localhost:4317`.
* `K6_OTEL_METRIC_PREFIX` - Metric prefix. Default is empty.
* `K6_OTEL_FLUSH_INTERVAL` - How frequently to flush metrics to the receiver from k6. Default is `1s`.
* `K6_OTEL_PUSH_INTERVAL` - How frequently to push metrics to the receiver from k6. Default is `1s`.

## Build

To build a `k6` binary with this extension, first ensure you have the prerequisites:

- [Go toolchain](https://go101.org/article/go-toolchain.html)
- Git
- [xk6](https://github.com/grafana/xk6)

```bash
make build
```

This will result in a `k6` binary in the current directory.

## Local usage

1. You could run a local environment with OpenTelemetry collector ([Grafana Alloy](https://github.com/grafana/alloy)), Prometheus backend and Grafana (http://localhost:3000/):

```bash
docker-compose up -d
```

2. Run with the just build `k6:

```bash
K6_OTEL_METRIC_PREFIX=k6_ ./k6 run -o xk6-opentelemetry examples/script.js
```
