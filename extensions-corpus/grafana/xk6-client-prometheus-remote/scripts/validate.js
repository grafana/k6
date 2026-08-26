// End-to-end validation script for the xk6-client-prometheus-remote extension.
// Writes a uniquely named metric to a Prometheus remote write endpoint and
// asserts the store call succeeds. scripts/validate.sh then queries Prometheus
// to confirm the data landed. Run via scripts/validate.sh, not directly.
import remote from 'k6/x/remotewrite';
import { check } from 'k6';

const URL = __ENV.REMOTE_WRITE_URL || 'http://localhost:9090/api/v1/write';

export const options = {
    vus: 2,
    iterations: 10,
    thresholds: {
        checks: ['rate==1.0'],
    },
};

const client = new remote.Client({ url: URL });

export default function () {
    const res = client.store([
        {
            labels: [
                { name: '__name__', value: 'xk6_validate_metric' },
                { name: 'service', value: 'validation' },
                { name: 'vu', value: `${__VU}` },
            ],
            samples: [{ value: __VU * 100 + __ITER, timestamp: Date.now() }],
        },
    ]);
    check(res, {
        // Prometheus remote write receiver returns 204 on success.
        'store status 2xx': (r) => r.status >= 200 && r.status < 300,
    });
}
