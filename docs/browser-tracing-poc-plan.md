# POC Plan: Adding Distributed Tracing to the k6 Browser Module

## Goal

Inject W3C Trace Context (`traceparent`) or Jaeger (`uber-trace-id`) headers into HTTP requests made by the browser (Chromium), so that backend services already instrumented with distributed tracing can correlate browser-initiated requests with their downstream spans. This lets users see the full trace of a browser workflow (login, page navigation, XHR/fetch calls) end-to-end.

---

## Key Design Considerations

### Browser vs HTTP request model

The k6 HTTP instrumentation is 1:1 — each `http.get()` call maps to exactly one HTTP request, and gets exactly one trace ID + span ID.

The browser model is 1:N — a single `page.goto('https://app.example.com')` triggers **many** HTTP requests: the HTML document, CSS, JS bundles, images, fonts, XHR/fetch calls, etc. This means:

- **Trace ID scope**: Should be shared across all sub-requests of a navigation/action (so Tempo groups them together)
- **Span ID scope**: Should be unique per request (so each sub-request appears as a distinct span in the trace)
- **Trace lifecycle**: A new trace ID should be generated per navigation or per VU iteration (TBD)

### Two CDP mechanisms for header injection

| Mechanism | CDP Command | Per-request? | Cache impact | Performance |
|-----------|-------------|-------------|-------------|-------------|
| `setExtraHTTPHeaders` | `Network.setExtraHTTPHeaders` | No (same headers for all requests) | None | Minimal |
| `page.route` / Fetch domain | `Fetch.continueRequest` | Yes (unique headers per request) | Disabled | Overhead per request (pause/resume via CDP) |

### Existing browser tracing (not the same thing)

The browser module already has internal OTEL tracing (`internal/js/modules/k6/browser/trace/trace.go`) that traces **k6's own operations** (e.g., how long `page.goto` takes to execute). This is for observing k6 internals. What we're adding is different: injecting trace context into the **browser's outgoing HTTP requests** so the backend can participate in the trace.

---

## Proposals

### Proposal A: Pure JS — local library file using `page.route` under the hood

**Approach**: Create a local JS file (`examples/browser/browser-tracing.js`) containing the tracing helpers. The test script imports from this file. It uses `page.route(/./, handler)` internally to intercept every browser request and inject trace context headers.

**User-facing API**:

```javascript
import { browser } from 'k6/browser';
import { instrumentBrowser, uninstrumentBrowser } from './browser-tracing.js';

export const options = {
  scenarios: {
    ui: {
      executor: 'shared-iterations',
      options: { browser: { type: 'chromium' } },
    },
  },
};

export default async function () {
  const page = await browser.newPage();

  // Enable tracing — registers a page.route(/./) handler internally
  instrumentBrowser(page, {
    propagator: 'w3c',    // or 'jaeger'
    sampling: 1.0,         // sampling rate
  });

  // All subsequent browser requests will carry traceparent headers
  await page.goto('https://app.example.com/login');
  await page.locator('#username').fill('testuser');
  await page.locator('#password').fill('password');
  await page.locator('#submit').click();

  // Cleanup: remove the route before closing
  uninstrumentBrowser(page);

  await page.close();
}
```

**How it works internally**:

```
instrumentBrowser(page, opts)
    |
    v
Generate trace ID (k6-format: prefix + code + timestamp + random)
    |
    v
page.route(/./, handler)   <-- enables CDP Fetch domain
    |
    v
For each intercepted request:
    1. Generate unique span ID (random hex, 16 chars)
    2. Build traceparent: 00-{traceID}-{spanID}-{flags}
    3. route.continue({
         headers: {
           ...route.request().headers(),
           'traceparent': traceparent
         }
       })
```

**Trace ID lifecycle**: One trace ID per `instrumentBrowser()` call (effectively per VU iteration). All requests across all navigations within the page's lifetime share one trace ID, giving full end-to-end workflow visibility. Each individual request gets a unique span ID for differentiation within the trace.

**Pros**:
- Zero Go code changes required
- Ships as a standalone JS file — no k6 release cycle dependency
- Consistent with existing HTTP instrumentation approach
- User has full control and visibility
- Can iterate quickly on the JS side

**Cons**:
- **Performance overhead**: CDP Fetch domain pauses every request (pause → inspect → modify → resume via WebSocket round-trip). This is the main concern for a load testing tool.
- **Cache disabled**: CDP Fetch domain disables browser cache when active
- **Trace ID generation in JS**: Need to replicate k6's trace ID format in JavaScript (prefix + code + timestamp + random) or use a simpler format. The k6-specific format is needed for Grafana Cloud/Tempo correlation.
- **Cloud metadata integration**: JS can't set VU metadata for cloud insights collection the way the HTTP instrumentation does (it needs `vu.State().Tags.Modify(...)` which is Go-side)

**Risks**:
- Fetch domain performance impact may be unacceptable for load testing scenarios with many concurrent VUs
- Interaction with user-defined `page.route` handlers (only one handler can match — if user has their own routes, the tracing route needs to cooperate)

---

### Proposal B: Go-based — automatic `setExtraHTTPHeaders` injection

**Approach**: Add Go code to the browser module that automatically calls `Network.setExtraHTTPHeaders` with a `traceparent` header when tracing is enabled. The trace ID is generated Go-side using the existing k6 trace ID format. Enabled via a browser context option or an explicit JS API call.

**User-facing API — Option B1 (browser context option)**:

```javascript
import { browser } from 'k6/browser';

export default async function () {
  const context = await browser.newContext({
    tracing: {
      propagator: 'w3c',
      sampling: 1.0,
    },
  });
  const page = await context.newPage();

  // All browser requests automatically carry traceparent headers
  await page.goto('https://app.example.com/login');
  // ...
  await page.close();
  await context.close();
}
```

**User-facing API — Option B2 (explicit method)**:

```javascript
import { browser } from 'k6/browser';

export default async function () {
  const page = await browser.newPage();

  // Enable tracing on the page — calls Network.setExtraHTTPHeaders internally
  await page.enableTracing({ propagator: 'w3c' });

  await page.goto('https://app.example.com/login');
  // ...

  // Optionally update trace ID for next navigation
  await page.updateTraceID();

  await page.close();
}
```

**How it works internally**:

```
page.enableTracing(opts)  OR  context option triggers it
    |
    v
Go code generates trace ID (k6-format, using existing encoding.go logic)
    |
    v
Go code generates span ID
    |
    v
Builds traceparent: 00-{traceID}-{spanID}-{flags}
    |
    v
networkManager.SetExtraHTTPHeaders({
    ...existingHeaders,
    "traceparent": traceparent
})
    |
    v
CDP: Network.setExtraHTTPHeaders  <-- applies to ALL requests
    |
    v
Optionally: on navigation events, update the span ID portion
```

**Trace ID lifecycle**:
- Generated once when tracing is enabled
- Can be refreshed per navigation via Go-side event listeners (listen for `Network.requestWillBeSent` with `type=Document` and update headers)
- Or user explicitly calls `page.updateTraceID()` before navigating

**Pros**:
- **No performance overhead**: `Network.setExtraHTTPHeaders` is a one-time CDP call, not per-request
- **Cache still works**: No Fetch domain = no cache penalty
- **k6 trace ID format**: Generated Go-side using existing code, guaranteed Grafana Cloud compatibility
- **Cloud metadata integration**: Go code can set VU metadata for cloud insights collection
- **Simple implementation**: Small, focused Go changes

**Cons**:
- **Same trace context for ALL requests**: Every request (document, CSS, JS, images, XHR) gets the same `traceparent` with the same span ID. Backend sees them all as the same "span" rather than distinct operations.
- **No per-request span differentiation**: The main value of distributed tracing (seeing individual request spans) is lost
- **Go release cycle**: Changes ship with k6 releases, not independently
- **Less user control**: Users can't customize which requests get traced

**Mitigation for the "same span ID" issue**:
The Go code could listen for navigation events and periodically update `setExtraHTTPHeaders` with a new span ID. But there's no way to give each concurrent request its own span ID via this mechanism.

---

### Proposal C: Go-based — automatic internal route handler (Fetch domain)

**Approach**: Add Go code that internally registers a route handler (similar to `page.route`) when tracing is enabled. The Go code handles trace ID generation, per-request span ID injection, and cloud metadata — but all via the Fetch domain so each request gets a unique span.

**User-facing API**:

```javascript
import { browser } from 'k6/browser';

export default async function () {
  const page = await browser.newPage();

  // Enable tracing — Go code internally enables Fetch domain and handles everything
  await page.enableTracing({ propagator: 'w3c', sampling: 1.0 });

  await page.goto('https://app.example.com/login');
  // ...

  // Disable tracing (disables Fetch domain, restores cache)
  await page.disableTracing();

  await page.close();
}
```

**How it works internally**:

```
page.enableTracing(opts)
    |
    v
Go code generates trace ID (k6-format)
    |
    v
Enables CDP Fetch domain (if not already enabled by user routes)
    |
    v
Registers an internal "tracing route" that runs BEFORE user routes
    |
    v
For each intercepted request:
    1. Go code generates unique span ID
    2. Builds traceparent header
    3. Adds header to request via ContinueRequest
    4. Sets trace metadata on VU state for cloud collection
    |
    v
If user has their own page.route handlers:
    - Tracing route runs first (adds headers)
    - Then user route runs (can modify further or abort)
```

**Key implementation detail**: This requires modifying `FrameManager.requestStarted()` to support an internal "pre-route" handler that always runs before user routes and doesn't consume the route match (i.e., it calls `continue` with headers but allows the next route to also match).

**Pros**:
- **Per-request span IDs**: Each request gets a unique span, proper distributed tracing
- **k6 trace ID format**: Generated Go-side, cloud-compatible
- **Cloud metadata integration**: Full VU metadata support
- **Transparent to user**: Simple enable/disable API
- **Coexists with user routes**: Internal handler doesn't block user-defined routes

**Cons**:
- **Same Fetch domain overhead as Proposal A**: Every request paused/resumed
- **Cache disabled**: Same as Proposal A
- **More complex Go changes**: Need to modify the route matching flow to support internal pre-route handlers
- **Go release cycle**: Ships with k6
- **Route interaction complexity**: Must handle the case where user routes call `abort` or `fulfill` (tracing handler already continued the request)

**This is the most architecturally complex proposal** but gives the best tracing fidelity with the least user effort.

---

### Proposal D: Hybrid — Go trace generation + JS route handler

**Approach**: Go code exposes a trace context generation API to JavaScript. A JS library uses this API to get properly-formatted trace IDs and span IDs, then uses `page.route` to inject them. Best of both worlds: k6-compatible trace IDs + JS flexibility.

**User-facing API**:

```javascript
import { browser } from 'k6/browser';
import { instrumentBrowser } from './browser-tracing.js';

export default async function () {
  const page = await browser.newPage();

  // instrumentBrowser calls a new Go-exposed API to get trace context,
  // then uses page.route internally
  await instrumentBrowser(page, { propagator: 'w3c' });

  await page.goto('https://app.example.com/login');
  await page.close();
}
```

**New Go API exposed to JS**:

```javascript
// New module or extension to browser module
import { generateTraceContext } from 'k6/browser/tracing';  // hypothetical

const ctx = generateTraceContext({ propagator: 'w3c' });
// ctx.traceID = "ab2901e3010eb0610133e0dac10a08a2"
// ctx.newSpanID() = "45f13d72b5e2fcf4" (new each call)
// ctx.header() = "traceparent: 00-ab290...-45f13...-01"
```

**Pros**:
- k6-compatible trace IDs (generated Go-side)
- JS flexibility for routing logic
- Cloud metadata could be set Go-side when `generateTraceContext` is called
- JS file can iterate independently on the routing logic

**Cons**:
- Requires both Go changes (new API) AND JS library
- Still has Fetch domain overhead (uses page.route)
- More moving parts than other proposals
- API surface to design and maintain

---

## Comparison Matrix

| Criteria | A: Pure JS | B: Go setExtraHTTPHeaders | C: Go internal route | D: Hybrid |
|----------|-----------|--------------------------|---------------------|-----------|
| **Per-request span IDs** | Yes | No | Yes | Yes |
| **Performance overhead** | High (Fetch) | None | High (Fetch) | High (Fetch) |
| **Browser cache** | Disabled | Works | Disabled | Disabled |
| **k6 trace ID format** | Needs JS replication | Native Go | Native Go | Native Go |
| **Cloud metadata** | Not possible from JS | Full support | Full support | Partial |
| **Go code changes** | None | Small | Medium | Small |
| **User effort** | Import + call | Option or method | Method call | Import + call |
| **Coexists with user routes** | Needs care | N/A | Needs implementation | Needs care |
| **Ship independently** | Yes (standalone JS) | No (k6 release) | No (k6 release) | Partially |
| **Complexity** | Low | Low | High | Medium |

---

## Decision: Proposal A (Pure JS) for the POC

**Confirmed: Proposal A** for the following reasons:

1. **Zero Go changes** means fastest time to a working POC
2. **Validates the concept** — does injecting traceparent into browser requests actually produce useful traces in Tempo?
3. **Validates the performance impact** — how much does the Fetch domain overhead matter in practice?
4. **Identifies edge cases** — what happens with redirects, service workers, WebSocket upgrades, etc.?

**Accepted POC limitations**:
- Simplified trace ID format (random UUID) instead of k6-specific encoding — good enough to validate the concept
- No Grafana Cloud metadata integration — traces appear in Tempo but won't correlate with k6 cloud test runs
- Fetch domain performance overhead — measure it, but don't optimize yet
- No special handling for user-defined routes — address in Go if moving to production
- Manual cleanup before `page.close()` required
- One `instrumentBrowser()` call per page

**If the POC validates the concept**, the route interaction and performance concerns can be addressed by moving to Go (Proposal B or C).

---

## POC Implementation Steps (Proposal A)

### Step 1: Create the JS library skeleton

Create a new JS file that exports `instrumentBrowser(page, options)`.

### Step 2: Implement trace ID and span ID generation in JS

- Trace ID: 32 hex chars (simplified: random for POC, k6-format later)
- Span ID: 16 hex chars (random per request)
- Sampling flag: from options

### Step 3: Register the page.route handler

```javascript
function instrumentBrowser(page, options) {
  const traceID = generateTraceID();
  const propagator = options.propagator || 'w3c';
  const sampling = options.sampling ?? 1.0;

  page.route(/./, (route) => {
    const spanID = generateSpanID();
    const header = buildHeader(propagator, traceID, spanID, sampling);
    const headerName = propagator === 'w3c' ? 'traceparent' : 'uber-trace-id';

    route.continue({
      headers: {
        ...route.request().headers(),
        [headerName]: header,
      },
    });
  });
}
```

### Step 4: Add cleanup function

Export `uninstrumentBrowser(page)` that calls `page.unrouteAll()` to remove the tracing route and restore normal request flow. User calls this manually before `page.close()`.

### Step 5: Test with a traced backend

- Set up a simple backend with OTEL instrumentation (e.g., a Go/Node service exporting to Jaeger or Tempo)
- Run k6 browser test with the instrumentation enabled
- Verify traces appear in Tempo/Jaeger with correct parent-child relationships
- Verify the full workflow (login → navigate → interact) appears as one trace with many spans

### Step 6: Measure performance impact

- Run the same browser test with and without instrumentation
- Compare: page load times, total test duration, CDP message volume
- Document the overhead to inform the production approach decision

---

## Resolved Questions

1. **Trace ID per what?** **Per VU iteration** (in practice, per `instrumentBrowser()` call). One trace ID covers the entire user workflow (login → dashboard → actions), giving full end-to-end visibility. Individual requests are differentiated by unique span IDs. Per-navigation would fragment the workflow into separate traces, losing the ability to see the full flow.

2. **Interaction with user routes**: Not a concern for the POC. If a production solution is warranted, this will be addressed in Go code (e.g., middleware-style routes that don't consume the match).

3. **Resource filtering**: Trace ALL requests for the POC (document, CSS, JS, images, XHR, fetch). Evaluate whether filtering is needed based on trace volume.

4. **Cloud integration**: Not in scope for the POC. This is separate from Grafana Cloud correlation. Pure JS is sufficient.

5. **Cleanup**: User manually calls cleanup before `page.close()` for the POC.

6. **Multiple pages**: One `instrumentBrowser()` call per page. No context-level API needed for the POC.

---

## Detailed TODO List (Proposal A — Pure JS POC)

### Phase 1: Core Library Implementation

The JS library that handles trace context generation and request interception.

- [x] **1.1 Create the library file**
  - Created `examples/browser/browser-tracing.js`
  - Exports: `instrumentBrowser`, `uninstrumentBrowser`

- [x] **1.2 Implement trace ID generation**
  - `generateTraceID()` produces 32 lowercase hex chars, guaranteed not all zeros

- [x] **1.3 Implement span ID generation**
  - `generateSpanID()` produces 16 lowercase hex chars, guaranteed not all zeros

- [x] **1.4 Implement W3C traceparent header builder**
  - `buildW3CHeader(traceID, spanID, sampled)` → `00-{traceID}-{spanID}-{flags}`

- [x] **1.5 Implement Jaeger uber-trace-id header builder**
  - `buildJaegerHeader(traceID, spanID, sampled)` → `{traceID}:{spanID}:0:{flags}`

- [x] **1.6 Implement sampling decision**
  - `shouldSample(rate)` — decided once per `instrumentBrowser()` call

### Phase 2: page.route Integration

Wire the trace context generation into browser request interception.

- [x] **2.1 Implement `instrumentBrowser(page, options)`**
  - Validates propagator and sampling rate, generates trace ID, makes sampling decision, registers route

- [x] **2.2 Implement the route handler**
  - Generates unique span ID per request, builds header, spreads existing headers, calls `route.continue()`

- [x] **2.3 Implement `uninstrumentBrowser(page)`**
  - Calls `page.unrouteAll()` to remove all routes

- [x] **2.4 Handle edge cases in the route handler**
  - Preserves existing headers, falls back to empty object if null, try/catch around `route.continue()`

### Phase 3: Test Script

A k6 browser test script that uses the library against a traced backend.

- [x] **3.1 Create a k6 browser test script**
  - Created `examples/browser/browser-tracing-test.js`
  - Imports library, targets QuickPizza via `QUICKPIZZA_URL` env var (default `http://localhost:3333`)
  - Workflow: load homepage → click "Pizza, Please!" → wait for result
  - `instrumentBrowser` logs trace ID to console for Tempo lookup

- [x] **3.2 Deploy QuickPizza locally with tracing stack**
  - Cloned `https://github.com/grafana/quickpizza` to `/tmp/quickpizza`
  - Ran: `docker compose -f compose.grafana-local-stack.monolithic.yaml up -d`
  - All containers running: QuickPizza (port 3333), Grafana (port 3000), Tempo, Alloy, Prometheus, Loki, Pyroscope
  - `QUICKPIZZA_TRUST_CLIENT_TRACEID: 1` is set — QuickPizza honors injected trace IDs

- [x] **3.3 Verify tracing works before k6 integration**
  - Verified QuickPizza generates its own traces (trace IDs in server logs)
  - Verified traces appear in Tempo via `GET /api/traces/{traceID}` API
  - Confirmed pipeline: QuickPizza → Alloy (OTLP) → Tempo → Grafana

### Phase 4: Validation

Verify traces are correct and useful.

- [x] **4.1 Verify trace header injection**
  - Ran `./k6 run examples/browser/browser-tracing-test.js` — 27 requests, 0 failures
  - Console output showed trace ID
  - QuickPizza server logs confirmed `traceparent` headers on all incoming requests
  - **Key fix**: k6 browser's `page.route()` requires regex patterns (not glob strings). Changed `'**/*'` to `/./`

- [x] **4.2 Verify traces appear in Grafana/Tempo**
  - Queried Tempo API: `GET /api/traces/{traceID}` returned trace data with spans
  - Trace contains QuickPizza server-side spans with correct trace ID from k6 injection

- [x] **4.3 Verify multi-step workflow visibility**
  - 37 QuickPizza requests all carry the same trace ID
  - Includes: static assets, `/api/config`, `/api/quotes`, `/api/pizza`, `/api/users/token/authenticate`, `/api/ingredients/*`, `/api/internal/recommendations`, `/api/tools`, `/api/doughs`, `/api/adjectives`, `/api/names`
  - Full pizza ordering workflow visible in one trace

- [x] **4.4 Test with both propagators**
  - W3C (`traceparent`): Verified end-to-end with QuickPizza
  - Jaeger (`uber-trace-id`): Runs without errors. QuickPizza only supports W3C, so Jaeger header not consumed by server (expected)

- [x] **4.5 Test sampling**
  - `sampling: 0.0` → all 3 iterations showed `sampled=false` (trace flags `00`)
  - `sampling: 1.0` → all iterations showed `sampled=true` (trace flags `01`)
  - Headers are always injected regardless of sampling (only trace flags differ)

### Phase 5: Performance Measurement

Quantify the overhead of the Fetch domain interception.

- [x] **5.1 Create a baseline benchmark script**
  - Created `examples/browser/browser-tracing-benchmark-baseline.js` (5 iterations, no instrumentation)

- [x] **5.2 Create an instrumented benchmark script**
  - Created `examples/browser/browser-tracing-benchmark-instrumented.js` (5 iterations, with instrumentation)

- [x] **5.3 Run both benchmarks and compare**
  - Baseline (5 iterations, no tracing):
    - `iteration_duration` avg: 2.51s
    - `browser_http_req_duration` avg: 13.63ms, p90: 20.81ms
    - FCP avg: 124.8ms, TTFB avg: 2.31ms
  - Instrumented (5 iterations, with tracing):
    - `iteration_duration` avg: 2.53s (+0.8%)
    - `browser_http_req_duration` avg: 23.35ms (+71%), p90: 48.78ms (+135%)
    - FCP avg: 184.8ms (+48%), TTFB avg: 6.08ms (+163%)
  - **Conclusion**: Per-request overhead is significant (CDP round-trip per request), but overall iteration duration barely affected (~0.8%). FCP/TTFB increase due to Fetch interception on initial page load.

- [x] **5.4 Measure CDP message volume**
  - Trace logging shows `Fetch.requestPaused` + `Fetch.continueRequest` pair for every request
  - ~27 requests per iteration = ~54 CDP messages for tracing (27 paused + 27 continue)
  - Additional messages: `Fetch.enable` (once) + `Fetch.disable` (once on unrouteAll)

### Phase 6: Documentation and Findings

Record what was learned from the POC.

- [x] **6.1 Document usage instructions**
  - Created `examples/browser/browser-tracing-README.md` with usage, options, setup instructions, and known limitations

- [x] **6.2 Document POC findings**
  - See "POC Findings" section below

- [x] **6.3 Recommend next steps**
  - See "Recommended Next Steps" section below

---

## Files That Would Change (by proposal)

### Proposal A (Pure JS — POC)
- **New**: `examples/browser/browser-tracing.js` — the tracing library (local file, imported by test script)
- **New**: `examples/browser/browser-tracing-test.js` — the test script that imports the library
- **None** in the k6 Go codebase

### Proposal B (Go setExtraHTTPHeaders)
- `internal/js/modules/k6/browser/common/page.go` — add `EnableTracing()` method
- `internal/js/modules/k6/browser/browser/page_mapping.go` — map `enableTracing` to JS
- `internal/js/modules/k6/browser/common/browser_context_options.go` — add tracing option (if using context option approach)
- New file: `internal/js/modules/k6/browser/common/trace_propagation.go` — trace ID/span ID generation and header building

### Proposal C (Go internal route)
- All files from Proposal B, plus:
- `internal/js/modules/k6/browser/common/frame_manager.go` — modify `requestStarted` to support internal pre-route handlers
- `internal/js/modules/k6/browser/common/network_manager.go` — internal route registration

### Proposal D (Hybrid)
- New file: `internal/js/modules/k6/browser/common/trace_propagation.go` — trace context API exposed to JS
- `internal/js/modules/k6/browser/browser/page_mapping.go` — map new API
- **New**: `examples/browser/browser-tracing.js` (local file)

---

## POC Findings

### The concept works

Injecting `traceparent` headers via `page.route` + `route.continue()` successfully propagates trace context from k6 browser tests to backend services. QuickPizza (with `QUICKPIZZA_TRUST_CLIENT_TRACEID: 1`) accepted the injected trace IDs, and full traces appeared in Tempo with server-side spans correctly parented under the browser-injected trace ID.

### Full workflow visibility achieved

A single trace ID covers the entire user workflow: page load (HTML, CSS, JS, images) → user interaction (click "Pizza, Please!") → API calls (`/api/pizza`, `/api/users/token/authenticate`, `/api/ingredients/*`, `/api/internal/recommendations`). All 37 requests in the QuickPizza flow appeared under one trace.

### Performance overhead

| Metric | Baseline | Instrumented | Overhead |
|--------|----------|--------------|----------|
| iteration_duration avg | 2.51s | 2.53s | **+0.8%** |
| browser_http_req_duration avg | 13.63ms | 23.35ms | **+71%** |
| browser_http_req_duration p90 | 20.81ms | 48.78ms | **+135%** |
| FCP avg | 124.8ms | 184.8ms | **+48%** |
| TTFB avg | 2.31ms | 6.08ms | **+163%** |

Per-request overhead is significant (~10ms per request from CDP interception round-trip), but total iteration duration is barely affected because request latency is a small fraction of the 2s `waitForTimeout` and page interaction time. For real load tests with many concurrent VUs, the CDP WebSocket traffic volume (54 extra messages per iteration) could become a concern.

### Edge cases discovered

1. **k6 browser's `page.route()` requires regex patterns, not glob strings**. The Playwright-style `'**/*'` glob pattern silently matches nothing in k6 — it's treated as a literal string match. The correct catch-all pattern is `/./` (ECMAScript regex). This is a documentation gap in k6's browser module.

2. **`page.unroute(pattern)` requires the same string representation**. Since regex patterns are converted to strings internally, using `page.unrouteAll()` is simpler and more reliable for cleanup.

3. **Browser cache is disabled** when Fetch interception is active. This is inherent to the CDP Fetch domain and cannot be avoided with this approach.

4. **No Fetch interception errors observed**. All 27 requests per iteration were successfully intercepted and continued with headers. No `Invalid InterceptionId` errors or context cancellation issues.

### What the POC does NOT cover

- k6-specific trace ID format (uses random hex instead)
- Grafana Cloud metadata integration
- WebSocket, Service Worker, or iframe request interception
- Multiple concurrent pages or browser contexts
- User-defined route conflict resolution

---

## Recommended Next Steps

### Short-term: Ship the JS library as-is

The pure JS approach (Proposal A) is sufficient for users who want browser tracing today. It works, it's zero-Go-changes, and the performance overhead is acceptable for most use cases. Consider:

1. Move `examples/browser/browser-tracing.js` to a jslib or official example
2. Document the regex pattern requirement (`/./` not `'**/*'`)
3. Add a note about cache being disabled during tracing

### Medium-term: Go-based implementation (Proposal B + C hybrid)

For production-quality browser tracing:

1. **Start with Proposal B** (`Network.setExtraHTTPHeaders`): Zero per-request overhead, cache works, k6-format trace IDs. Downside: all requests share the same span ID. This is acceptable if the primary goal is trace correlation (linking browser tests to backend traces) rather than per-request span visibility.

2. **If per-request spans are required**, add Proposal C as an opt-in mode: internal route handler in Go that runs before user routes, generates unique span IDs per request, and sets cloud metadata. This has the same Fetch domain overhead as the POC but with proper k6 trace IDs and cloud integration.

3. **API recommendation**: `page.enableTracing({ propagator: 'w3c', mode: 'lightweight' | 'detailed' })` where `lightweight` uses `setExtraHTTPHeaders` (Proposal B) and `detailed` uses internal Fetch interception (Proposal C).

### Decision criteria for Go implementation

- If browser tracing demand is low → ship the JS library, iterate based on feedback
- If cloud correlation is needed → Proposal B (Go, setExtraHTTPHeaders)
- If per-request span visibility is essential → Proposal C (Go, internal route handler)
