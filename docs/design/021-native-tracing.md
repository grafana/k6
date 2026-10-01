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

## Trace topology

The process starts one `k6.run` span before loading and evaluating the test. Its lifetime includes
loading, but the initial implementation does not propagate its context into loader internals. It
ends after setup, execution, teardown, and output finalization, but before the trace provider shuts
down.

An earlier revision of this design made every iteration an independent trace, linked to `k6.run`
by an OpenTelemetry `Link` rather than nested under it, specifically to avoid one enormous trace for
a long or distributed load test. In practice that left a lone `k6.run` span with nothing under it,
which was not useful on its own. The default topology now nests a real hierarchy instead, and
`--traces-split` (see [Runtime configuration](#runtime-configuration)) narrows down to just the
iteration boundary for anyone who still wants independent iteration traces:

```text
k6.run
  `- k6.scenario (one per running scenario)
      `- k6.vu (one per VU activation)
          `- iteration
              `- http.request (and other opted-in protocol spans)
```

`k6.scenario` and `k6.vu` are always built this way, regardless of `--traces-split`. Only `iteration`
changes: under `--traces-split`, it becomes an independent root trace instead of a child of `k6.vu`,
carrying a `Link` to that `k6.vu` span (not to `k6.run` directly, though they share the same trace
ID, since only the split iteration ever gets a new one via `WithNewRoot`). Child operations, such as
an opted-in HTTP request, always use the iteration span as their parent, in either mode.

`k6.scenario` carries `test.scenario` and `k6.scenario.executor`. `k6.vu` carries `test.vu`,
`k6.vu.id_in_instance`, `k6.vu.id_in_test`, and `test.scenario` — the same identifying attributes
iterations carry, so they're queryable at whichever span level a backend surfaces best.

The iteration attributes are:

- `test.iteration.number`
- `test.vu`
- `test.scenario`
- `k6.run.id`, only under `--traces-split`
- `k6.vu.id_in_instance`
- `k6.vu.id_in_test`
- `k6.vu.iteration_in_scenario`
- `k6.scenario.iteration_in_instance`, when available
- `k6.scenario.iteration_in_test`, when available

`k6.run.id` is conditional because it would be redundant otherwise: when nested (the default), the
iteration's own trace ID already equals the run's, so a backend gets it for free. It's only added
under `--traces-split` because splitting deliberately gives the iteration a different trace ID,
severing that free correlation.

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

Tracing, trace export, and topology are separate concerns:

```text
--tracing <comma-separated modules>|all|none
--traces-split
--traces-output none|otel[=host:port]
--traces-parent <encoded-parent-context>
--traces-sampler <sampler-name>
--traces-sampler-arg <sampler-argument>
--traces-propagator tracecontext|jaeger
```

`--tracing` takes a comma-separated list of protocol module names (`http`, `grpc`, `websockets`,
`browser`), or the sentinels `all` (every registered module) or `none` (default: nothing enabled).
Naming a module makes it default-on for every VU; each protocol's per-call `tracing: true/false`
option (documented in that protocol's section below) always overrides the default in either
direction. Mixing `all`/`none` with a module name, or naming an unregistered module, is a startup
error. The module registry is extensible: any module, built-in or from an extension, can register
its own name as a valid `--tracing` target.

`--tracing` creates real run, scenario, VU, and iteration span contexts even when
`--traces-output=none`; those spans simply have no exporter. When `--tracing` is left entirely
unset and an output is configured, the resolved set is `{"browser"}` only — preserving the behavior
already released before `--tracing` existed, when browser operation tracing was unconditional
whenever a trace exporter was configured. Any explicit `--tracing` value, including `none`,
overrides this inference. Explicitly disabling tracing while configuring an output is still an
error.

`--traces-split` (default off) restores independent iteration traces (see
[Trace topology](#trace-topology)) for anyone who prefers one trace per iteration over the nested
default; it does not affect `--tracing`'s module-list semantics.

The corresponding environment variables are `K6_TRACING`, `K6_TRACES_SPLIT`, `K6_TRACES_OUTPUT`,
`K6_TRACES_PARENT`, `K6_TRACES_SAMPLER`, `K6_TRACES_SAMPLER_ARG`, and
`K6_TRACES_PROPAGATOR`.

The supported samplers follow the OpenTelemetry names `always_on`, `always_off`, `traceidratio`,
`parentbased_always_on`, `parentbased_always_off`, and `parentbased_traceidratio`. The default is
`parentbased_always_on`. Ratio samplers require an argument between zero and one. By default, the
sampling decision cascades down from `k6.run` through `k6.scenario` and `k6.vu` to `iteration` via
normal parent-based inheritance. Only under `--traces-split` is the decision made once per
iteration, independently of the run.

`--traces-parent` accepts the complete primary header value for the selected propagator, rather
than a bare span ID. For W3C Trace Context this is a `traceparent` value; for Jaeger it is an
`uber-trace-id` value. The extracted remote context parents `k6.run`. Iterations only remain
independent roots linked to their VU span under `--traces-split`; otherwise they inherit the
remote parent through the normal nested hierarchy.

The propagator controls extraction of the configured parent and injection by opted-in HTTP, gRPC,
and WebSocket instrumentation. It does not change OTLP export. Browser network traffic is owned by
Chromium and does not currently receive these injected headers.

## Browser opt-in

Enabled via `--tracing=browser` (or `all`); see [Runtime configuration](#runtime-configuration) for
the module-list mechanism. Unlike the other three protocols, browser has no per-operation override —
it is a whole-VU on/off switch, since browser contexts and event listeners are connected once at
`IterStart`.

Browser network traffic runs inside Chromium and is separate from `k6/http`, so enabling `http` does
not affect browser traffic, and vice versa.

## HTTP opt-in

Enabled via `--tracing=http` (or `all`); see [Runtime configuration](#runtime-configuration) for
the module-list mechanism. A request can override the VU default in either direction:

```javascript
export default function () {
  http.get('https://example.test/default-on');
  http.get('https://example.test/explicitly-off', { tracing: false });
  http.get('https://example.test/one-request', { tracing: true }); // opts in without --tracing=http
}
```

The per-request `tracing` boolean always overrides the VU default. The common request path means the
same option applies to synchronous, asynchronous, and batch HTTP requests.

An enabled request creates an `http.request` client span with OpenTelemetry semantic attributes for
the HTTP method, sanitized URL, server address, and response status. It injects W3C `traceparent`
or Jaeger `uber-trace-id`, according to the runtime configuration, and adds the trace ID to the HTTP
metric sample metadata so Cloud Insights can correlate a sample with its trace. Explicit user
headers can still be considered in later API work; the k6-owned span is authoritative.

## gRPC opt-in

Enabled via `--tracing=grpc` (or `all`); see [Runtime configuration](#runtime-configuration) for
the module-list mechanism. The gRPC module owns tracing for unary requests, health checks, and
streams; any of them can override the VU default in either direction:

```javascript
export default function () {
  client.invoke('example.Service/Call', {}, { tracing: false });
}
```

Unary and stream spans inherit the iteration context, inject the configured trace context into gRPC
metadata, record RPC service, method, and status attributes, and add the trace ID to metric metadata.
A stream uses one span for its lifetime rather than creating a span for every message.

## WebSocket opt-in

Enabled via `--tracing=websockets` (or `all`); see [Runtime configuration](#runtime-configuration)
for the module-list mechanism. A connection can override the VU default in either direction:

```javascript
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
- The `--tracing` module registry now lets any module, built-in or extension, register itself as
  an opt-in target; still open is a documented, stable contract for what a registered module
  should do with the resulting default and the `tracing: true/false` per-call convention.
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
