import http from 'k6/http';
export default function () {
  console.log('RUNNING DEFAULT');
  http.get('https://test.k6.io/');
}
