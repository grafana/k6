import http from 'k6/http';
import { check } from 'k6';

const url = __ENV.K6_BENCH_URL || 'https://localhost:9100/metrics';

export const options = {
  insecureSkipTLSVerify: true,
  tlsAuth: [
    {
      cert: open('./localhost.pem'),
      key: open('./localhost-key.pem'),
    },
  ],
  scenarios: {
    http2_bench: {
      executor: 'constant-vus',
      vus: 20,
      duration: '30s',
    },
  },
};

export default function () {
  const res = http.get(url);

  check(res, {
    'status is 200': (r) => r.status === 200,
  });
}
