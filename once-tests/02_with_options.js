import http from 'k6/http';
export const options = { vus: 10, duration: '30s' };
export default function () {
  console.log('RUNNING DEFAULT');
  http.get('https://test.k6.io/');
}
