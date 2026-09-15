# Native tracing in k6

|                |                                                    |
| :------------- | :------------------------------------------------- |
| **status**     | 🔧 proposal                                        |
| **references** | [#1879], [#2128], [#2853], [#2854], [#2855], [#3029], [#3369], [#3445], [#3854], [#3855], [#5673], [#6298] |

## Problem

k6 has an OpenTelemetry trace provider and browser tracing, but it does not create a trace that
represents a complete test run or every VU iteration. Scripts also cannot annotate the active span,
and protocol modules cannot consistently attach their work to a k6-owned iteration span.

Earlier work established several important constraints:

- [#1879] explored native tracing and showed the value of spans created by k6 itself.
- [#2128], [#2853], [#2854], and [#2855] focused on trace propagation. Propagation is useful, but it
  does not provide a k6-owned execution trace by itself.
- [#3029] identified the missing root span problem.
- [#3369] proposed automatically tracing groups and HTTP requests. Its discussion also highlighted
  that manual start/end APIs are easy to misuse and that high-cardinality tests need explicit
  controls.
- [#3445] introduced the trace provider that browser tracing currently uses.
- [#3854] and [#3855] removed the old propagation-oriented `k6/experimental/tracing` API while
  reserving the module name for an API that works with actual traces and spans.
- [#5673] tracks the async JavaScript context needed before span scopes can safely survive arbitrary
  asynchronous script code.
- [#6298] demonstrates why trace provider shutdown must happen after all span producers finish.

## Goals

- Represent the test process and each VU iteration with native OpenTelemetry spans.
- Let k6 code, protocol modules, browser code, and scripts use the same active iteration span.
- Keep protocol-level instrumentation opt-in because it can generate a large volume of spans.
- Preserve useful browser tracing behavior and attribute names.
- Propagate an opted-in protocol operation's configured trace context to the system under test.
- Preserve correct provider shutdown and span export ordering.

## Non-goals

- Automatically tracing every protocol operation.
- Providing arbitrary script-created span scopes before async context propagation is solved.
- Defining final sampling, exporter, or distributed-run configuration.
- Making all iteration spans children of a single long-lived run span.

## Trace topology

The process starts one `k6.run` span before loading and evaluating the test. Its lifetime includes
loading, but the initial implementation does not propagate its context into loader internals. It
ends after setup, execution, teardown, and output finalization, but before the trace provider shuts
down.

Every iteration starts an independent root span named `iteration`. The iteration span contains a
link to the `k6.run` span rather than using it as its parent:

```text
k6.run
  <- link -- iteration trace 1
  <- link -- iteration trace 2
  <- link -- iteration trace N

iteration
  `- http.request
```

Independent iteration traces avoid one enormous trace for a long or distributed load test. They
can be sampled, queried, and retained independently while the link preserves run correlation.
Child operations, such as an opted-in HTTP request, use the iteration span as their parent.

The initial iteration attributes are:

- `test.iteration.number`
- `test.vu`
- `test.scenario`
- `k6.run.id`
- `k6.vu.id_in_instance`
- `k6.vu.id_in_test`
- `k6.vu.iteration_in_scenario`
- `k6.scenario.iteration_in_instance`, when available
- `k6.scenario.iteration_in_test`, when available

The first three names remain compatible with the existing browser iteration spans.

## Context propagation inside k6

The runner installs the iteration span in the VU context before `IterStart` and removes it after
`IterEnd`. Code participating in the iteration reads the span from that context instead of keeping
a separate global current-span stack.

When enabled by the browser module, its trace registry reuses the core iteration span and adds its
browser metadata. It retains a fallback root span only for isolated callers that do not run through
the core iteration runner. This preserves the trace structure introduced by [xk6-browser#1100].

Asynchronous protocol implementations must capture the current VU context before starting a
goroutine. General script-created child-span scopes are deferred until [#5673] supplies reliable
async JavaScript context propagation.

## Script API

`k6/experimental/tracing` only exposes and manipulates the active span. Protocol modules own their
instrumentation policy and APIs; the tracing module does not enable another module's tracing.

The tracing module gives scripts access to the active span without ownership of its lifetime:

```javascript
import { currentSpan } from 'k6/experimental/tracing';

export default function () {
  const span = currentSpan();
  span.setAttribute('test.user_type', 'registered');
  span.addEvent('checkout.started');
}
```

`currentSpan()` exposes `traceId`, `spanId`, `sampled`, and `recording`. Setting attributes accepts
primitive JavaScript values. When tracing is disabled, the returned handle is a safe no-op.

This deliberately avoids an explicit `startSpan()`/`endSpan()` pair. The lifecycle remains owned by
k6, which prevents leaked spans and matches the automatic-context direction discussed in [#3369].

## Runtime configuration

Tracing and trace export are separate concerns:

```text
--tracing[=true|false]
--traces-output none|otel[=host:port]
--traces-parent <encoded-parent-context>
--traces-sampler <sampler-name>
--traces-sampler-arg <sampler-argument>
--traces-propagator tracecontext|jaeger
```

`--tracing` creates real run and iteration span contexts even when `--traces-output=none`; those
spans simply have no exporter. Configuring an output continues to imply tracing when `--tracing`
was not explicitly set, preserving the behavior of `--traces-output`. Explicitly disabling tracing
while configuring an output is an error.

The corresponding environment variables are `K6_TRACING`, `K6_TRACES_OUTPUT`,
`K6_TRACES_PARENT`, `K6_TRACES_SAMPLER`, `K6_TRACES_SAMPLER_ARG`, and
`K6_TRACES_PROPAGATOR`.

The supported samplers follow the OpenTelemetry names `always_on`, `always_off`, `traceidratio`,
`parentbased_always_on`, `parentbased_always_off`, and `parentbased_traceidratio`. The default is
`parentbased_always_on`. Ratio samplers require an argument between zero and one. Because every
iteration is an independent root, the ratio decision is made once per iteration and inherited by
its protocol child spans.

`--traces-parent` accepts the complete primary header value for the selected propagator, rather
than a bare span ID. For W3C Trace Context this is a `traceparent` value; for Jaeger it is an
`uber-trace-id` value. The extracted remote context parents `k6.run`. Iterations remain independent
roots linked to the run span.

The propagator controls extraction of the configured parent and injection by opted-in HTTP, gRPC,
and WebSocket instrumentation. It does not change OTLP export. Browser network traffic is owned by
Chromium and does not currently receive these injected headers.

## Browser opt-in

Browser operation tracing is disabled by default and enabled through the browser module during the
init context:

```javascript
import { browser } from 'k6/browser';

browser.enableTracing();

export default async function () {
  const page = await browser.newPage();
  await page.goto('https://example.test');
}
```

Browser instrumentation must be enabled during init because the tracer, browser contexts, and event
listeners are connected at `IterStart`. Browser network traffic runs inside Chromium and is separate
from `k6/http`, so `http.enableTracing()` and its request override do not affect browser traffic.

## HTTP opt-in

HTTP request tracing is disabled by default. A script can enable the default for its VUs during init
or for the current VU while it is executing:

```javascript
import http from 'k6/http';

http.enableTracing();

export default function () {
  http.get('https://example.test/default-on');
  http.get('https://example.test/explicitly-off', { tracing: false });
}
```

A request can opt in without changing the VU default:

```javascript
http.get('https://example.test/one-request', { tracing: true });
```

The per-request `tracing` boolean always overrides the VU default. The common request path means the
same option applies to synchronous, asynchronous, and batch HTTP requests.

An enabled request creates an `http.request` client span with OpenTelemetry semantic attributes for
the HTTP method, sanitized URL, server address, and response status. It injects W3C `traceparent`
or Jaeger `uber-trace-id`, according to the runtime configuration, and adds the trace ID to the HTTP
metric sample metadata so Cloud Insights can correlate a sample with its trace. Explicit user
headers can still be considered in later API work; the k6-owned span is authoritative.

## gRPC opt-in

The gRPC module owns tracing for unary requests, health checks, and streams:

```javascript
import grpc from 'k6/net/grpc';

grpc.enableTracing();

export default function () {
  client.invoke('example.Service/Call', {}, { tracing: false });
}
```

Without module-level enablement, an individual invocation, health check, or stream can use
`{ tracing: true }`.
Unary and stream spans inherit the iteration context, inject the configured trace context into gRPC
metadata, record RPC service, method, and status attributes, and add the trace ID to metric metadata.
A stream uses one span for its lifetime rather than creating a span for every message.

## WebSocket opt-in

The `k6/websockets` module owns its enablement and exposes a named module function:

```javascript
import { WebSocket, enableTracing } from 'k6/websockets';

enableTracing();

export default function () {
  new WebSocket('wss://example.test/socket', null, { tracing: false });
}
```

Legacy `k6/ws` is intentionally not instrumented.

Each connection produces one `websocket.session` client span covering the session lifetime. Its
context is injected into the HTTP upgrade headers and its trace ID is added to all session metric
metadata. Messages remain activity within the session instead of producing high-volume child spans.

## Lifecycle and failures

The run span begins before test loading so load or evaluation failures can be recorded. Iteration and
HTTP spans end through deferred cleanup and record errors. The run span ends only after outputs have
finished, and the provider shuts down last so all completed spans can be flushed. This ordering
follows the shutdown concerns documented in [#6298].

## Future work

- Define resource attributes for local, cloud, and distributed runs.
- Stabilize names and attributes against current OpenTelemetry semantic conventions.
- Add group and user-created child spans after async context propagation is available.
- Define a supported opt-in contract for extension protocol modules.
- Define distributed run identity and cross-instance run correlation.
- Evaluate additional propagation formats without reviving the removed propagation-only API.
- Add cardinality and span-volume guardrails before moving the module out of experimental status.

[#1879]: https://github.com/grafana/k6/pull/1879
[#2128]: https://github.com/grafana/k6/issues/2128
[#2853]: https://github.com/grafana/k6/issues/2853
[#2854]: https://github.com/grafana/k6/pull/2854
[#2855]: https://github.com/grafana/k6/pull/2855
[#3029]: https://github.com/grafana/k6/issues/3029
[#3369]: https://github.com/grafana/k6/issues/3369
[#3445]: https://github.com/grafana/k6/pull/3445
[#3854]: https://github.com/grafana/k6/issues/3854
[#3855]: https://github.com/grafana/k6/pull/3855
[#5673]: https://github.com/grafana/k6/issues/5673
[#6298]: https://github.com/grafana/k6/pull/6298
[xk6-browser#1100]: https://github.com/grafana/xk6-browser/pull/1100
