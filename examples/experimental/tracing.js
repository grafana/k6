import http from 'k6/http';
import { currentSpan } from 'k6/experimental/tracing';

// Enable tracing for HTTP requests made by every VU. Run this example with:
// k6 run --traces-output=otel examples/experimental/tracing.js
http.enableTracing();

export default function () {
  const span = currentSpan();
  span.setAttribute('example.user_type', 'registered');
  span.addEvent('example.started');

  // This request is traced because http.enableTracing() enabled the VU default.
  http.get('https://quickpizza.grafana.com');

  // A request can override the VU default in either direction. Without the
  // http.enableTracing() call above, { tracing: true } opts in a single request.
  http.get('https://quickpizza.grafana.com/api/ratings', { tracing: false });
}
