import http from 'k6/http';
import { check } from 'k6';

export const options = {
  http3: true,
  insecureSkipTLSVerify: true,
  tlsAuth: [
    {
      cert: open('/Users/valdemarpavesi/localhost.pem'),
      key: open('/Users/valdemarpavesi/localhost-key.pem'),
      domains: ['localhost'],
    },
  ],
  iterations: 3,
  vus: 1,
};

export default function () {
  const res = http.get('https://localhost:9100/metrics');

  console.log(`Proto: ${res.proto} | Status: ${res.status} | Body: ${res.body.length} bytes`);

  check(res, {
    'status is 200': (r) => r.status === 200,
    'protocol is HTTP/3': (r) => r.proto === 'HTTP/3.0',
    'body is not empty': (r) => r.body.length > 0,
  });
}
