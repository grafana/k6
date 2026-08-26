import { check, fail } from 'k6';
import loki from 'k6/x/loki';

/*
 * Host name with port
 * @constant {string}
 */
const HOST = "xxx.grafana.net:3100";

/**
 * Name of the Loki tenant
 * @constant {string}
 */
const TENANT_ID = __ENV.LOKI_TENANT_ID || fail("provide LOKI_TENANT_ID when starting k6");

/**
 * Access token of the Loki tenant with logs:write and logs:read permissions
 * @constant {string}
 */
const ACCESS_TOKEN = __ENV.LOKI_ACCESS_TOKEN || fail("provide LOKI_ACCESS_TOKEN when starting k6");

/**
 * URL used for push and query requests
 * Path is automatically appended by the client
 * @constant {string}
 */
const BASE_URL = `https://${TENANT_ID}:${ACCESS_TOKEN}@${HOST}`;

/**
 * Minimum amount of virtual users (VUs)
 * @constant {number}
 */
const MIN_VUS = 10

/**
 * Maximum amount of virtual users (VUs)
 * @constant {number}
 */
const MAX_VUS = 100;

/**
 * Constants for byte values
 * @constant {number}
 */
const KB = 1024;
const MB = KB * KB;

/**
 * Definition of test scenario
 */
export const options = {
  thresholds: {
    'http_req_failed': [{ threshold: 'rate<=0.01', abortOnFail: true }],
  },
  scenarios: {
    write: {
      executor: 'ramping-vus',
      exec: 'write',
      startVUs: MIN_VUS,
      stages: [
        { duration: '5m', target: MAX_VUS },
        { duration: '30m', target: MAX_VUS },
      ],
      gracefulRampDown: '1m',
    },
  },
};

const labelCardinality = {
  "app": 5,
  "namespace": 1,
  "pod": 10,
};
const conf = new loki.Config(BASE_URL, 10000, 0.9, labelCardinality);
const client = new loki.Client(conf);

/**
 * Entrypoint for write scenario
 */
export function write() {
  let streams = randomInt(4, 8);
  let res = client.pushParameterized(streams, 800 * KB, 1 * MB);
  check(res,
    {
      'successful write': (res) => {
        let success = res.status === 204;
        if (!success) console.log(res.status, res.body);
        return success;
      },
    }
  );
}

/**
 * Return a random integer between min and max including min and max
 */
function randomInt(min, max) {
  return Math.floor(Math.random() * (max - min + 1) + min);
}
