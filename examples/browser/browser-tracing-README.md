# Browser Tracing POC

Injects W3C Trace Context (`traceparent`) or Jaeger (`uber-trace-id`) headers into all HTTP requests made by the browser during a k6 test. This allows backend services to correlate browser-initiated requests with their distributed traces.

## Files

- `browser-tracing.js` — tracing library (`instrumentBrowser`, `uninstrumentBrowser`)
- `browser-tracing-test.js` — test script targeting QuickPizza
- `browser-tracing-benchmark-baseline.js` — benchmark without tracing
- `browser-tracing-benchmark-instrumented.js` — benchmark with tracing

## Usage

```javascript
import { browser } from 'k6/browser';
import { instrumentBrowser, uninstrumentBrowser } from './browser-tracing.js';

export default async function () {
  const page = await browser.newPage();

  await instrumentBrowser(page, {
    propagator: 'w3c',  // 'w3c' or 'jaeger'
    sampling: 1.0,       // 0.0 to 1.0
  });

  await page.goto('http://localhost:3333');
  // ... interact with the page ...

  await uninstrumentBrowser(page);
  await page.close();
}
```

The trace ID is logged to the console when `instrumentBrowser` is called. Use this to search for the trace in Grafana/Tempo.

## Running with QuickPizza

1. Clone and start QuickPizza with the local Grafana stack:
   ```bash
   git clone https://github.com/grafana/quickpizza
   cd quickpizza
   docker compose -f compose.grafana-local-stack.monolithic.yaml up
   ```

2. Build k6 and run the test:
   ```bash
   go build -o k6 .
   ./k6 run examples/browser/browser-tracing-test.js
   ```

3. Open Grafana at `http://localhost:3000`, go to Explore → Tempo, and search by the trace ID from the k6 console output.

## Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `propagator` | `string` | `'w3c'` | `'w3c'` for `traceparent` header, `'jaeger'` for `uber-trace-id` header |
| `sampling` | `number` | `1.0` | Sampling rate (0.0-1.0). Affects trace flags only — headers are always injected. |

## How it works

`instrumentBrowser` registers a `page.route(/./`)` handler that intercepts every browser request via the CDP Fetch domain. For each request, it generates a unique span ID, builds the trace header, and forwards the request with the added header using `route.continue()`. Note: k6 browser's `page.route()` requires ECMAScript regex patterns (not glob strings like Playwright's `'**/*'`).

## Known limitations

- **Performance overhead**: The CDP Fetch domain pauses every request for interception (pause → modify → resume via WebSocket round-trip).
- **Cache disabled**: The Fetch domain disables browser cache when active.
- **Route conflicts**: If you have your own `page.route()` handlers, only the first matching route fires (LIFO order). Register `instrumentBrowser` before your own routes. `uninstrumentBrowser` calls `page.unrouteAll()`, which removes all routes (including user-defined ones).
- **No cloud metadata**: Trace IDs are not sent to Grafana Cloud for correlation with k6 cloud test runs.
- **Simplified trace ID**: Uses random hex instead of the k6-specific encoded format.
