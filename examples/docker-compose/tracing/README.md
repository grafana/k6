# Native tracing with Jaeger

This example builds k6 from the current checkout, exports native traces to Jaeger, and sends traced
HTTP requests to an echo server. The script checks that the configured propagation header reaches
the echo server.

## Run the example

Start Jaeger and the target first, then run the one-shot k6 container:

```bash
docker compose up -d jaeger target
docker compose run --rm --build k6
```

The default run uses Jaeger `uber-trace-id` propagation and samples every iteration. Successful
output contains six passing checks and prints each iteration trace ID.

Open <http://localhost:16686>, select the `k6` service, and choose **Find Traces**. The results
include:

- a `k6.run` trace;
- three independent `iteration` traces linked to the run;
- an `http.request` child span in each iteration trace;
- the `example.propagator` attribute and `example.request.started` event on each iteration.

The target response is also available at <http://localhost:8080>.

## Try the configuration options

Use W3C Trace Context instead of Jaeger propagation:

```bash
K6_TRACES_PROPAGATOR=tracecontext docker compose run --rm --build k6
```

Attach `k6.run` to a remote Jaeger parent:

```bash
K6_TRACES_PARENT=4bf92f3577b34da6a3ce929d0e0e4736:00f067aa0ba902b7:0:1 \
  docker compose run --rm --build k6
```

Change the iteration sampling ratio:

```bash
K6_TRACES_SAMPLER_ARG=0.25 docker compose run --rm --build k6
```

To verify that tracing still creates and propagates real span contexts without an exporter, override
the service command. The checks continue to pass, but the new run does not appear in Jaeger:

```bash
docker compose run --rm --build k6 \
  run --tracing --traces-output=none --traces-propagator=jaeger /scripts/script.js
```

Stop and remove the example containers when finished:

```bash
docker compose down
```
