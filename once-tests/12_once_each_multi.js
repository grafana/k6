import http from 'k6/http';
export const options = {
  scenarios: {
    api: {
      executor: 'constant-vus',
      vus: 10,
      duration: '30s',
      exec: 'apiTest',
      tags: { test_type: 'api' },
    },
    write: {
      executor: 'shared-iterations',
      vus: 5,
      iterations: 100,
      exec: 'writeTest',
      tags: { test_type: 'write' },
    },
  },
};
export function apiTest() {
  console.log('RUNNING API_TEST');
  http.get('https://test.k6.io/');
}
export function writeTest() {
  console.log('RUNNING WRITE_TEST');
  http.get('https://test.k6.io/');
}
