import http from 'k6/http';
export const options = {
  scenarios: {
    api: {
      executor: 'constant-vus',
      vus: 10,
      duration: '30s',
      exec: 'apiTest',
      env: { BASE_URL: 'https://test.k6.io' },
      tags: { test_type: 'api' },
    },
  },
};
export function apiTest() {
  console.log('RUNNING API_TEST, BASE_URL=' + __ENV.BASE_URL);
  http.get(__ENV.BASE_URL + '/');
}
