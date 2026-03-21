import http from 'k6/http';
export const options = {
  scenarios: {
    api: {
      executor: 'constant-vus',
      vus: 10,
      duration: '30s',
      exec: 'apiTest',
    },
    smoke: {
      executor: 'per-vu-iterations',
      vus: 1,
      iterations: 1,
    },
  },
};
export function apiTest() {
  console.log('RUNNING API_TEST');
  http.get('https://test.k6.io/');
}
export default function () {
  console.log('RUNNING DEFAULT');
  http.get('https://test.k6.io/');
}
