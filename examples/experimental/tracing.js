import http from 'k6/http';
import { currentSpan } from 'k6/experimental/tracing';

// --tracing=http enables tracing for HTTP requests made by every VU. Run
// this example with:
// k6 run --tracing=http --traces-output=otel examples/experimental/tracing.js

export default function () {
  const span = currentSpan();
  span.setAttribute('example.user_type', 'registered');
  span.addEvent('example.started');

  // This request is traced because --tracing=http enabled the VU default.
  http.get('https://quickpizza.grafana.com');

  // A request can override the VU default in either direction. Without
  // --tracing=http, { tracing: true } opts in a single request instead.
  http.get('https://quickpizza.grafana.com/api/ratings', { tracing: false });
}
