# OpenTelemetry Collector Go Exporter

[![PkgGoDev](https://pkg.go.dev/badge/go.opentelemetry.io/otel/exporters/otlp)](https://pkg.go.dev/go.opentelemetry.io/otel/exporters/otlp)

This exporter exports OpenTelemetry spans and metrics to the OpenTelemetry Collector.


## Installation and Setup

The exporter can be installed using standard `go` functionality.

```bash
$ go get -u go.opentelemetry.io/otel/exporters/otlp
```

A new exporter can be created using the `NewExporter` function.

```golang
package main

import (
	"context"
	"log"

	"go.opentelemetry.io/otel/sdk/metric/controller/push"
	"go.opentelemetry.io/otel/exporters/otlp"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	"go.opentelemetry.io/otel/sdk/metric/selector/simple"
	metricsdk "go.opentelemetry.io/otel/sdk/export/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	ctx := context.Background()
	exporter, err := otlp.NewExporter(ctx) // Configure as needed.
	if err != nil {
		log.Fatalf("failed to create exporter: %v", err)
	}
	defer func() {
		err := exporter.Shutdown(ctx)
		if err != nil {
			log.Fatalf("failed to stop exporter: %v", err)
		}
	}()

	// Note: The exporter can also be used as a Batcher. E.g.
	//   tracerProvider := sdktrace.NewTracerProvider(
	//   	sdktrace.WithBatcher(exporter,
	//   		sdktrace.WithBatchTimeout(time.Second*15),
	//   		sdktrace.WithMaxExportBatchSize(100),
	//   	),
	//   )
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	processor := processor.New(simple.NewWithInexpensiveDistribution(), metricsdk.StatelessExportKindSelector())
	pusher := push.New(processor, exporter)
	pusher.Start()
	metricProvider := pusher.MeterProvider()

	// Your code here ...
}
```

## Configuration

Configurations options can be specified when creating a new exporter (`NewExporter`).

### `WorkerCount(n uint)`

Sets the number of Goroutines to use when processing telemetry.


### `WithInsecure()`

Disables client transport security for the exporter's gRPC connection just like [`grpc.WithInsecure()`](https://pkg.go.dev/google.golang.org/grpc#WithInsecure) does.
By default, client security is required unless `WithInsecure` is used.

### `WithAddress(addr string)`

Sets the address that the exporter will connect to the collector on.
The default address the exporter connects to is `localhost:55680`.

### `WithReconnectionPeriod(rp time.Duration)`

Set the delay between connection attempts after failing to connect with the collector.

### `WithCompressor(compressor string)`

Set the compressor for the gRPC client to use when sending requests.
The compressor used needs to have been registered with `google.golang.org/grpc/encoding` prior to using here.
This can be done by `encoding.RegisterCompressor`.
Some compressors auto-register on import, such as gzip, which can be registered by calling `import _ "google.golang.org/grpc/encoding/gzip"`.

### `WithHeaders(headers map[string]string)`

Headers to send with gRPC requests.

### `WithTLSCredentials(creds "google.golang.org/grpc/credentials".TransportCredentials)`

TLS credentials to use when talking to the server.

### `WithGRPCServiceConfig(serviceConfig string)`

The default gRPC service config used when .

By default, the exporter is configured to support [retries](#retries).

```json
{
	"methodConfig":[{
		"name":[
			{ "service":"opentelemetry.proto.collector.metrics.v1.MetricsService" },
			{ "service":"opentelemetry.proto.collector.trace.v1.TraceService" }
		],
		"waitForReady": true,
		"retryPolicy":{
			"MaxAttempts":5,
			"InitialBackoff":"0.3s",
			"MaxBackoff":"5s",
			"BackoffMultiplier":2,
			"RetryableStatusCodes":[
				"UNAVAILABLE",
				"CANCELLED",
				"DEADLINE_EXCEEDED",
				"RESOURCE_EXHAUSTED",
				"ABORTED",
				"OUT_OF_RANGE",
				"UNAVAILABLE",
				"DATA_LOSS"
			]
		}
	}]
}
```

### `WithGRPCDialOption(opts ..."google.golang.org/grpc".DialOption)`

Additional `grpc.DialOption` to be used.

These options take precedence over any other set by other parts of the configuration.

## Retries

The exporter will not, by default, retry failed requests to the collector.
However, it is configured in a way that it can easily be enable.

To enable retries, the `GRPC_GO_RETRY` environment variable needs to be set to `on`. For example,

```
GRPC_GO_RETRY=on go run .
```

The [default service config](https://github.com/grpc/proposal/blob/master/A6-client-retries.md) used by default is defined to retry failed requests with exponential backoff (`0.3seconds * (2)^retry`) with [a max of `5` retries](https://github.com/open-telemetry/oteps/blob/be2a3fcbaa417ebbf5845cd485d34fdf0ab4a2a4/text/0035-opentelemetry-protocol.md#export-response)).

These retries are only attempted for reponses that are [deemed "retry-able" by the collector](https://github.com/grpc/proposal/blob/master/A6-client-retries.md#validation-of-retrypolicy).
