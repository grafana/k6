# OpenTelemetry Output

This example demonstrates how to integrate k6 performance testing with OpenTelemetry. The test metrics are sent to an OpenTelemetry collector (OTEL Collector), which forwards them to Prometheus for storage. These metrics can then be visualized using Grafana dashboards.

## Prerequisites

- Docker
- Docker Compose

## Run the example

```bash
docker-compose up -d
```

## Access the k6 performance test dashboard

Open the k6 performance test dashboard in your browser http://localhost:3000/d/demo-uid/k6-opentelemetry-prometheus

## Run the k6 test

This will run the test for 3 minutes with 10 virtual users and send the metrics to the OTEL Collector.

```bash
K6_OTEL_GRPC_EXPORTER_ENDPOINT=localhost:4317 \
K6_OTEL_GRPC_EXPORTER_INSECURE=true \
K6_OTEL_METRIC_PREFIX=k6_ \
k6 run --tag testid=1 -o experimental-opentelemetry script.js
```
