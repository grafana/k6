// Test: --once-each with thresholds
// Expected: all scenarios run once each; threshold breach causes exit code 99
import http from 'k6/http';

export const options = {
  scenarios: {
    api: {
      executor: 'constant-vus',
      vus: 10,
      duration: '30s',
      exec: 'apiTest',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<1'],  // intentionally impossible threshold
  },
};

export function apiTest() {
  console.log('RUNNING API_TEST');
  http.get('https://test.k6.io/');
}
