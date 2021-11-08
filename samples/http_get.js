import http from 'k6/http';

export default function () {
  http.get('https://test-api.k6.io/');
};
