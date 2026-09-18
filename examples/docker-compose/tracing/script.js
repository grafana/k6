import { check } from 'k6';
import http from 'k6/http';
import { currentSpan } from 'k6/experimental/tracing';

http.enableTracing();

export const options = {
  scenarios: {
    tracing: {
      executor: 'shared-iterations',
      vus: 1,
      iterations: 3,
      startTime: '3s',
    },
  },
  thresholds: {
    checks: ['rate==1'],
  },
};

export default function () {
  const span = currentSpan();
  span.setAttribute('example.propagator', __ENV.K6_EXAMPLE_PROPAGATOR);
  span.addEvent('example.request.started');

  const response = http.get('http://target/');
  const expectedHeader = __ENV.K6_EXAMPLE_PROPAGATOR === 'jaeger'
    ? /Uber-Trace-Id: [0-9a-f]{32}:[0-9a-f]{16}:0:[0-9a-f]+/i
    : /Traceparent: 00-[0-9a-f]{32}-[0-9a-f]{16}-0[01]/i;

  check(response, {
    'target returned HTTP 200': (result) => result.status === 200,
    [`${__ENV.K6_EXAMPLE_PROPAGATOR} context was propagated`]: (result) =>
      expectedHeader.test(result.body),
  });

  console.log(`iteration trace ID: ${span.traceId}, sampled: ${span.sampled}`);
}
