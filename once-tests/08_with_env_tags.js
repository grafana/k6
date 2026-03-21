import http from 'k6/http';
export const options = {
  scenarios: {
    api: {
      executor: 'constant-vus',
      vus: 2,
      duration: '2s',
      exec: 'apiTest',
      env: { BASE_URL: 'https://test.k6.io', API_KEY: 'test123' },
      tags: { test_type: 'api', team: 'backend' },
    },
  },
};
export function apiTest() {
  console.log('RUNNING API_TEST');
  console.log('BASE_URL=' + __ENV.BASE_URL);
  console.log('API_KEY=' + __ENV.API_KEY);
  http.get(__ENV.BASE_URL + '/');
}
