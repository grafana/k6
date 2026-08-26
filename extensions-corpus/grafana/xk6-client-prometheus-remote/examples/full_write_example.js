import { check, fail } from 'k6';
import exec from 'k6/execution';
import remote from 'k6/x/remotewrite';
import { randomIntBetween } from "https://jslib.k6.io/k6-utils/1.1.0/index.js";

const PROMETHEUS_TOKEN = __ENV.PROMETHEUS_TOKEN || fail("provide PROMETHEUS_TOKEN when strting k6");
const PROMETHEUS_USERNAME = __ENV.PROMETHEUS_USERNAME || fail("provide PROMETHEUS_USERNAME when strting k6");
const BASE_URL = 'prometheus-prod-10-prod-us-central-0.grafana.net';
const RW_UNIQUE_METRICS = 50;
const RW_SERIES_PER_METRIC = 20;

const remote_write_url = `https://${PROMETHEUS_USERNAME}:${PROMETHEUS_TOKEN}@${BASE_URL}/api/prom/push`;

let write_client = new remote.Client({ url: remote_write_url });

export let options = {
  thresholds: {
    'remote_write_req_duration': [{ threshold: 'p(95) < 1000', abortOnFail: false }],
    'checks': [{ threshold: 'rate==1', abortOnFail: true }],
    'http_req_failed': [{ threshold: 'rate==0.00', abortOnFail: true }],
  },
  scenarios: {
    writing_metrics: {
      executor: 'constant-arrival-rate',
      rate: RW_UNIQUE_METRICS,
      timeUnit: '15s',

      duration: '5m',
      preAllocatedVUs: 1,
      maxVus: 200,
      exec: 'write_scenario'
    },
  },
};

export function write_scenario() {
  let metric_name = `k6_generated_metric_${exec.scenario.iterationInTest % RW_UNIQUE_METRICS}`

  let res = write_client.store(
    generate_metrics(metric_name, RW_SERIES_PER_METRIC)
  );

  check(res, {
    'write worked': (r) => r.status >= 200 && r.status < 300,
  }) || fail(JSON.stringify(res));
}

function generate_metrics(metric_name, numberOfSeries) {
  let timestamp = Date.now();

  console.log(`Writing batch of ${numberOfSeries} series. Metric name ${metric_name}`);

  let metrics = [];

  for (let series_id = 0; series_id < numberOfSeries; series_id++) {
    metrics.push({
      "labels": [
        { "name": "__name__", "value": metric_name },
        { "name": "host", "value": "test-host" },
        { "name": "series_id", "value": series_id.toString() },
        { "name": "service", "value": "bar" },
      ],
      "samples": [
        { "value": randomIntBetween(1, 100), 'timestamp': timestamp },
      ]
    })
  }
  return metrics
}

