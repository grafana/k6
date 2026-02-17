# Research: Browser Module Tracing - Deep Technical Analysis

This document captures a deep investigation into three areas required for understanding how distributed tracing could work with the k6 browser module:

1. How `page.route` intercepts and amends browser requests
2. How the k6 HTTP instrumented tracing client works
3. The underlying W3C/Jaeger propagation technology

---

## 1. How Requests Are Amended Using `page.route`

### 1.1 Overview

The k6 browser module provides `page.route(url, handler)` which allows users to intercept HTTP requests made by the browser (Chromium) and modify them before they reach the server. Under the hood, this uses the **Chrome DevTools Protocol (CDP) Fetch domain** to pause, inspect, modify, and resume requests.

### 1.2 Route Registration Flow

When a user calls `page.route('**/*', handler)` from JavaScript, the following chain executes:

**Step 1: JS-to-Go bridge** (`browser/page_mapping.go:937-961`)

```go
func mapPageRoute(vu moduleVU, p *common.Page) func(sobek.Value, sobek.Callable) (*sobek.Promise, error) {
    return func(path sobek.Value, cb sobek.Callable) (*sobek.Promise, error) {
        ppath := parseStringOrRegex(path, false)
        tq := vu.get(ctx, p.TargetID())

        route := func(r *common.Route) error {
            _, err := queueTask(ctx, tq, func() (any, error) {
                return cb(sobek.Undefined(), vu.Runtime().ToValue(mapRoute(vu, r)))
            })()
            return err
        }

        return promise(vu, func() (any, error) {
            return nil, p.Route(ppath, route, newRegExMatcher(ctx, vu, tq))
        }), nil
    }
}
```

The URL pattern is parsed (quoted strings become literal matches, regex patterns become ECMAScript regex matches). The user's JavaScript callback is wrapped in a Go closure that queues it on the Sobek runtime's task queue.

**Step 2: Page.Route** (`common/page.go:1272-1295`)

```go
func (p *Page) Route(path string, cb RouteHandlerCallback, rm RegExMatcher) error {
    if !p.hasRoutes() {
        err := p.mainFrameSession.updateRequestInterception(true)
        // ...
    }
    matcher, err := newPatternMatcher(path, rm)
    routeHandler := NewRouteHandler(path, cb, matcher)
    // Prepend (newest routes match first)
    p.routes = append([]*RouteHandler{routeHandler}, p.routes...)
    return nil
}
```

Key details:
- **First route registered triggers CDP Fetch domain enablement** via `updateRequestInterception(true)`
- Routes are prepended (LIFO order: last registered = first tried)
- A `RouteHandler` stores the path, callback, and URL matcher function

**Step 3: Enabling CDP Fetch domain** (`common/network_manager.go:816-850`)

```go
func (m *NetworkManager) updateProtocolRequestInterception() error {
    actions := []Action{
        network.SetCacheDisabled(true),
        fetch.Enable().
            WithHandleAuthRequests(true).
            WithPatterns([]*fetch.RequestPattern{
                {URLPattern: "*", RequestStage: fetch.RequestStageRequest},
            }),
    }
    // Execute CDP commands...
}
```

This sends `Fetch.enable` to Chromium with a wildcard URL pattern, causing **every request** to pause and emit a `Fetch.requestPaused` CDP event.

### 1.3 Request Interception Flow

Once the Fetch domain is enabled, every browser request triggers this sequence:

```
Browser makes request
    |
    v
CDP: Fetch.requestPaused event
    |
    v
NetworkManager.onRequestPaused()  -- checks blocked hosts/IPs, stores event
    |
    v
NetworkManager.onRequestWillBeSent() -- pairs with paused event
    |
    v
NetworkManager.onRequest() -- creates Request object, stores it
    |
    v
FrameManager.requestStarted() -- matches against routes, invokes handler
    |
    v
Route handler callback (user's JS function)
    |
    +---> route.continue({headers: ...})  -- amend and forward request
    +---> route.abort('failed')            -- block the request
    +---> route.fulfill({status: 200, ...}) -- return a mock response
```

**The critical pairing mechanism** (`common/network_manager.go:579-602, 609-688`):

CDP sends two events for each intercepted request: `Network.requestWillBeSent` and `Fetch.requestPaused`. These can arrive in **either order**. The NetworkManager handles this with two maps:

- `reqIDToRequestWillBeSentEvent` -- stores willBeSent events waiting for their paused partner
- `reqIDToRequestPausedEvent` -- stores paused events waiting for their willBeSent partner

When both events for a given `requestID` are available, `onRequest()` is called with both.

**Route matching** (`common/frame_manager.go:519-577`):

```go
func (m *FrameManager) requestStarted(req *Request) {
    if !m.page.hasRoutes() {
        return
    }
    route := NewRoute(m.logger, m.page.mainFrameSession.networkManager, req)
    for _, r := range m.page.routes {
        matched, err := r.urlMatcher(req.URL())
        if matched {
            r.handler(route)  // Invoke user's callback
            return            // First match wins
        }
    }
    // No match: auto-continue
    route.Continue(ContinueOptions{})
}
```

### 1.4 Amending Requests via Route

The user's route handler receives a `Route` object (`common/http.go:724-801`) with three methods:

#### `route.continue(options)` -- Modify and forward

```go
type ContinueOptions struct {
    Headers  []HTTPHeader  // Override request headers
    Method   string        // Override HTTP method
    PostData []byte        // Override request body
    URL      string        // Override request URL
}

func (r *Route) Continue(opts ContinueOptions) error {
    return r.networkManager.ContinueRequest(r.request.interceptionID, opts, r.request.HeadersArray())
}
```

This calls `Fetch.continueRequest` CDP command, which tells Chromium to resume the paused request with the optionally-modified fields. Headers are converted to CDP `HeaderEntry` format and sent via base64-encoded post data if modified.

**Headers specifically**: Users can pass new headers in their route handler:

```javascript
page.route('**/*', (route) => {
    route.continue({
        headers: {
            ...route.request().headers(),
            'traceparent': '00-abc123...-def456...-01'
        }
    });
});
```

The headers are parsed from the JS object (`browser/route_mapping.go:105-121`), converted to `[]common.HTTPHeader` (name-value pairs), then to `[]*fetch.HeaderEntry` for the CDP call (`common/network_manager.go:976-989`).

#### `route.abort(reason)` -- Block the request

Calls `Fetch.failRequest` with a network error reason (e.g., `"failed"`, `"aborted"`, `"connectionrefused"`).

#### `route.fulfill(options)` -- Return a mock response

```go
type FulfillOptions struct {
    Body        []byte
    ContentType string
    Headers     []HTTPHeader
    Status      int64
}
```

Calls `Fetch.fulfillRequest` to return a synthetic response without ever reaching the server.

#### `route.request()` -- Access the intercepted request

Returns the `Request` object containing the original URL, method, headers, post data, resource type, and frame information.

### 1.5 Important Implementation Details

1. **`onRequestPaused` handles blocked hosts/IPs** before route matching (`network_manager.go:659-688`). The request is checked against `state.Options.BlacklistIPs` and `state.Options.BlockedHostnames`. If blocked, `Fetch.failRequest` is sent regardless of routes.

2. **If no routes match but the Fetch domain is enabled**, the request is automatically continued unchanged (`frame_manager.go:573`).

3. **If no routes are registered at all**, the `!m.page.hasRoutes()` check in `onRequestPaused` causes `ContinueRequest` to be called immediately (`network_manager.go:650-656`), and `requestStarted` returns early.

4. **The Route is single-use**: `startHandling()` sets `handled = true` and returns an error if called twice, preventing double-handling of a request.

5. **Thread safety**: Route matching acquires `page.routesMu.RLock()`, but temporarily releases it while the handler executes (to allow the handler to register/unregister routes without deadlocking).

---

## 2. How the k6 HTTP Instrumented Tracing Client Works

### 2.1 Historical Context

The tracing functionality has gone through two implementations:

1. **Go-based module** (`k6/experimental/tracing`): Introduced in k6 v0.43.0, removed in v0.55.0. This was a native Go module that monkey-patched the `k6/http` module's methods.

2. **JavaScript library** (`http-instrumentation-tempo`): The current implementation, hosted at `https://jslib.k6.io/http-instrumentation-tempo/1.0.1/index.js`. This is a pure JS replacement that uses the same approach.

Both implementations share the same core design. The Go source code (recovered from git history) is authoritative for understanding the architecture.

### 2.2 What It Does

The instrumentation library wraps every HTTP request made by the standard `k6/http` module to:

1. **Generate a trace ID** with k6-specific structure
2. **Generate a span ID** for each request
3. **Inject trace context headers** into the outgoing request
4. **Set trace metadata** on the VU state for downstream collection by the cloud output

### 2.3 Propagation Formats

The library supports exactly **two** propagation formats (configured at init time):

#### W3C Trace Context (`propagator: "w3c"`)

Injects a single header:

```
traceparent: 00-{trace-id}-{parent-id}-{trace-flags}
```

Concrete example:
```
traceparent: 00-ab2901e3010eb0610133e0dac10a08a2-45f13d72b5e2fcf4-01
```

- `00` = W3C version (always "00")
- 32 hex chars = 16-byte trace ID (k6-structured, see below)
- 16 hex chars = 8-byte randomly generated span/parent ID
- `01` = sampled, `00` = not sampled

#### Jaeger (`propagator: "jaeger"`)

Injects a single header:

```
uber-trace-id: {trace-id}:{span-id}:{parent-span-id}:{flags}
```

Concrete example:
```
uber-trace-id: ab2901e3010eb0610133e0dac10a08a2:45f13d72:0:1
```

- Trace ID: same as W3C (32 hex chars)
- Span ID: 8 hex chars (randomly generated)
- Parent span ID: always `0` (root span)
- Flags: `1` = sampled, `0` = not sampled

### 2.4 What Is NOT Injected

- **No `tracestate` header** -- The W3C vendor-specific companion header is not emitted
- **No `baggage` header** -- The W3C baggage specification is explicitly unsupported (the Go module threw an error: `"baggage is not yet supported"`)
- **No OpenTelemetry Baggage** -- Despite the user's initial assumption, this is **not** baggage-based propagation. It uses W3C Trace Context propagation (or Jaeger), not the `baggage` header

### 2.5 Trace ID Structure

The trace ID is **not** a random UUID. It has k6-specific structure encoded in 16 bytes:

```
[k6 prefix (varint)] [code (varint)] [unix timestamp nanos (varint)] [random bytes]
```

- **k6 prefix**: `0o756` (octal) = `0x1EE` = the ASCII value of 'K' as a varint marker
- **Code**: `12` for cloud runs, `33` for local runs
- **Timestamp**: `time.Now().UnixNano()` as a varint
- **Random**: Remaining bytes are random

This structure allows Grafana Cloud/Tempo to identify k6-originated traces and correlate them with test run data.

### 2.6 Span ID Generation

Span IDs are generated as random hex strings using characters `123456789abcdef` (note: '0' is excluded):

- W3C: 16 hex characters (8 bytes equivalent)
- Jaeger: 8 hex characters (4 bytes equivalent)

A new span ID is generated for **each HTTP request**, meaning every request gets a unique span within the same trace.

### 2.7 Integration Flow

```
k6 test script calls http.get(url)
    |
    v
Instrumented wrapper intercepts the call
    |
    v
Generate new trace ID (or reuse per-VU trace ID)
Generate new span ID (unique per request)
    |
    v
Create propagation header:
  W3C:    traceparent: 00-{traceID}-{spanID}-{flags}
  Jaeger: uber-trace-id: {traceID}:{spanID}:0:{flags}
    |
    v
Inject header into request's params.headers
    |
    v
Set trace_id in VU metadata (for Cloud output collection)
    |
    v
Call original k6/http method with modified headers
    |
    v
Backend service receives request with traceparent header
Backend extracts trace context and creates child span
Trace is continued through the backend service chain
    |
    v
After request completes, clean up trace_id metadata
```

### 2.8 Cloud Integration

The trace ID flows through to Grafana Cloud via the insights collector (`internal/output/cloud/insights/collect.go`):

1. The instrumented HTTP client sets `trace_id` in VU metadata
2. When the request completes, `httpext.Trail` samples include this metadata
3. The Cloud insights `Collector` filters trails that have `trace_id` metadata
4. These are packaged as `insights.RequestMetadata` with trace ID, timing, and labels
5. They're flushed to Grafana Cloud Tempo for correlation with backend traces

Notably, the collector has a TODO comment: `// TODO(lukasz, other-proto-support): Support grpc/websocket trails.` -- confirming that only HTTP trails are currently supported.

### 2.9 Sampling

Configurable via the `sampling` option (0.0 to 1.0, default 1.0):

- `sampling: 1.0` uses `AlwaysOnSampler` (no randomness, always sampled)
- `sampling: 0.5` uses `ProbabilisticSampler` (50% chance each request is sampled)

The sampling decision affects only the trace flags byte (`01` vs `00`), not whether the header is injected. Headers are always sent; the flag tells the backend whether to record the trace.

---

## 3. Trace Propagation Technology Deep Dive

### 3.1 W3C Trace Context Specification

The W3C Trace Context is the industry standard for distributed trace propagation, defined in [W3C Trace Context](https://www.w3.org/TR/trace-context/). It uses two HTTP headers:

#### `traceparent` Header

Format: `{version}-{trace-id}-{parent-id}-{trace-flags}`

```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
             ^^                                  ^^^^^^^^^^^^^^^^  ^^
             version                             parent-id         flags
                ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
                trace-id
```

| Field | Size | Format | Description |
|-------|------|--------|-------------|
| `version` | 1 byte (2 hex chars) | Always `00` | Protocol version. Max allowed: `fe` (254). `ff` is invalid. |
| `trace-id` | 16 bytes (32 hex chars) | Lowercase hex | Globally unique trace identifier. Must not be all zeros. Shared by all spans in one trace. |
| `parent-id` | 8 bytes (16 hex chars) | Lowercase hex | The span ID of the calling span. Must not be all zeros. Each service generates a new one for its own span. |
| `trace-flags` | 1 byte (2 hex chars) | Bit field | Bit 0 (LSB) = sampled flag. `01` = sampled, `00` = not sampled. Other bits reserved. |

The OpenTelemetry Go SDK implementation lives in the k6 vendor tree at `vendor/go.opentelemetry.io/otel/propagation/trace_context.go`:

```go
// Injection (lines 39-64):
func (TraceContext) Inject(ctx context.Context, carrier TextMapCarrier) {
    sc := trace.SpanContextFromContext(ctx)
    // Build: version + "-" + traceID + "-" + spanID + "-" + flags
    carrier.Set(traceparentHeader, sb.String())
    carrier.Set(tracestateHeader, sc.TraceState().String())
}

// Extraction (lines 80-128):
func (tc TraceContext) extract(carrier TextMapCarrier) trace.SpanContext {
    // Parse traceparent: split on "-", decode hex fields, validate
    // Parse tracestate (failure MUST NOT affect traceparent parsing)
}
```

#### `tracestate` Header

Format: `vendor1=value1,vendor2=value2`

Purpose: Carries **vendor-specific trace data** alongside the vendor-neutral `traceparent`. Multiple tracing systems can coexist in the same trace.

Implementation in `vendor/go.opentelemetry.io/otel/trace/tracestate.go`:
- Maximum **32 members**
- Key format: `[a-z][a-z0-9_\-*/]{0,255}` or multi-tenant `tenant@system`
- Value format: 1-256 printable ASCII chars (no `,` or `=`)
- No duplicate keys allowed

Example:
```
tracestate: rojo=00f067aa0ba902b7,congo=t61rcWkgMzE
```

### 3.2 W3C Baggage Specification

The `baggage` header (specified in [W3C Baggage](https://www.w3.org/TR/baggage/)) is a **separate** propagation mechanism from Trace Context.

Format: `key1=value1;property1,key2=value2;property2=propvalue`

Implementation in `vendor/go.opentelemetry.io/otel/baggage/baggage.go`:

```go
const (
    maxMembers               = 180
    maxBytesPerMembers       = 4096
    maxBytesPerBaggageString = 8192
    listDelimiter     = ","
    keyValueDelimiter = "="
    propertyDelimiter = ";"
)
```

| Aspect | Trace Context (`traceparent`) | Baggage (`baggage`) |
|--------|-------------------------------|---------------------|
| Purpose | Trace identity (who am I in the trace?) | Application context (arbitrary key-value pairs) |
| Consumed by | Tracing infrastructure | Application code |
| Auto-managed | Yes (tracing library updates span IDs) | No (developer explicitly sets/reads) |
| Sensitive data | Never put sensitive data | Propagated to ALL downstream services |
| Example use | Correlating spans across services | Passing user ID, tenant, feature flags |

**Key insight**: Baggage is NOT used by the k6 HTTP instrumentation library. The library uses Trace Context (`traceparent`) for propagation.

### 3.3 Jaeger Propagation Format

The Jaeger format uses a single header:

```
uber-trace-id: {trace-id}:{span-id}:{parent-span-id}:{flags}
```

| Aspect | W3C Trace Context | Jaeger |
|--------|-------------------|--------|
| Header name | `traceparent` | `uber-trace-id` |
| Delimiter | `-` (hyphen) | `:` (colon) |
| Version field | Yes (`00`) | No |
| Parent span in header | No (only current span-id) | Yes (explicit parent-span-id) |
| Trace ID size | Always 128-bit (32 hex) | 64-bit or 128-bit |
| Vendor data | Separate `tracestate` header | Not in the trace header |
| Baggage | W3C `baggage` header | Individual `uberctx-*` headers |
| Debug flag | Not defined | Bit 1 = debug flag |

### 3.4 How Propagation Works End-to-End

```
k6 VU (Service A)  -->  API Gateway (Service B)  -->  Backend (Service C)
```

1. **k6 generates trace context** for each HTTP request:
   ```
   traceparent: 00-ab2901e3010eb0610133e0dac10a08a2-45f13d72b5e2fcf4-01
   ```

2. **Service B receives the request**, extracts `traceparent`:
   - trace-id: `ab2901e3010eb0610133e0dac10a08a2` (same throughout)
   - parent-id: `45f13d72b5e2fcf4` (k6's span)
   - Creates its own span with a new span-id: `b7ad6b7169203331`
   - Records parent as `45f13d72b5e2fcf4`

3. **Service B makes request to Service C**, injects updated context:
   ```
   traceparent: 00-ab2901e3010eb0610133e0dac10a08a2-b7ad6b7169203331-01
   ```
   - trace-id: unchanged (same trace)
   - parent-id: now Service B's span-id
   - flags: preserved

4. **Trace backend** (Tempo, Jaeger) receives spans from all services, reconstructs the call tree using trace-id to group and parent-id to establish hierarchy.

### 3.5 How Browser Requests Could Carry Trace Context

The browser (Chromium) does **not** understand trace context headers. It treats them as opaque custom headers. There are two mechanisms to inject them:

#### Mechanism 1: `Network.setExtraHTTPHeaders` (CDP)

Sets headers on **all** requests from the browser session:

```go
// common/network_manager.go:991-1001
func (m *NetworkManager) SetExtraHTTPHeaders(headers network.Headers) error {
    return network.SetExtraHTTPHeaders(headers).Do(cdp.WithExecutor(m.ctx, m.session))
}
```

Exposed to JS via `page.setExtraHTTPHeaders()`. This sets the same headers on every request (navigations, XHR, fetch, resource loads).

**Limitation**: All requests get the same trace context. No per-request span ID differentiation.

#### Mechanism 2: `Fetch.continueRequest` (CDP, via `page.route`)

Modifies headers on a **per-request basis** via the Fetch domain interception:

```go
// common/network_manager.go:888-933
func (m *NetworkManager) ContinueRequest(requestID fetch.RequestID, opts ContinueOptions, ...) error {
    action := fetch.ContinueRequest(requestID)
    if len(opts.Headers) > 0 {
        action = action.WithHeaders(toFetchHeaders(opts.Headers))
    }
    return action.Do(cdp.WithExecutor(m.ctx, m.session))
}
```

This is the `page.route` approach, which allows **unique trace context per request** -- each intercepted request can get its own span ID in the `traceparent` header.

**Advantage**: Proper per-request tracing with unique span IDs.
**Overhead**: The Fetch domain has performance overhead (every request is paused and resumed through CDP).

---

## 4. Key Technical Facts Summary

| Area | Fact |
|------|------|
| **k6 HTTP tracing uses** | W3C Trace Context (`traceparent`) or Jaeger (`uber-trace-id`), NOT baggage |
| **Headers injected** | Only one header per propagator (no `tracestate`, no `baggage`) |
| **Trace ID format** | k6-specific: prefix + code + timestamp + random (16 bytes, 32 hex) |
| **Span ID format** | Random hex per request (no '0' char), 8 bytes for W3C, 4 bytes for Jaeger |
| **page.route mechanism** | CDP Fetch domain: pause all requests, match URL, invoke handler, continue/abort/fulfill |
| **Header amendment** | Via `route.continue({headers: {...}})` which calls `Fetch.continueRequest` |
| **Browser header injection** | Two options: `setExtraHTTPHeaders` (global, static) or `page.route` (per-request, dynamic) |
| **Current tracing limitation** | Only `httpext.Trail` is collected for cloud insights (TODO comment for gRPC/WebSocket) |
| **Sampling** | Configurable 0.0-1.0, affects trace-flags byte only |
| **Request interception overhead** | Enabling Fetch domain disables cache and adds round-trip per request through CDP |

---

## 5. Source File Reference

| File | Purpose |
|------|---------|
| `common/network_manager.go` | CDP network/fetch event handling, request interception, continue/abort/fulfill |
| `common/page.go` | Route registration, URL matching, event handlers |
| `common/frame_manager.go:519-577` | Route matching and handler invocation on `requestStarted` |
| `common/http.go:724-801` | Route struct with Continue/Abort/Fulfill methods |
| `common/helpers.go:245-280` | Pattern matcher (string literal or regex) |
| `browser/page_mapping.go:937-961` | JS-to-Go bridge for `page.route()` |
| `browser/route_mapping.go` | JS-to-Go bridge for Route object methods |
| `common/frame_session.go:1178-1185` | `updateRequestInterception` enabling/disabling Fetch domain |
| `internal/output/cloud/insights/collect.go` | Cloud trace ID collection from HTTP trails |
| `vendor/go.opentelemetry.io/otel/propagation/trace_context.go` | W3C traceparent inject/extract |
| `vendor/go.opentelemetry.io/otel/trace/tracestate.go` | W3C tracestate parsing |
| `vendor/go.opentelemetry.io/otel/baggage/baggage.go` | W3C baggage parsing |
