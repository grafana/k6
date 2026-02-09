import http from 'k6/http';
import { check } from 'k6';

const url = __ENV.K6_BENCH_URL || 'https://localhost:9100/metrics';

export const options = {
  http3: true,
  insecureSkipTLSVerify: true,
  tlsAuth: [
    {
      cert: open('/Users/valdemarpavesi/localhost.pem'),
      key: open('/Users/valdemarpavesi/localhost-key.pem'),
    },
  ],
  scenarios: {
    http3_bench: {
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
