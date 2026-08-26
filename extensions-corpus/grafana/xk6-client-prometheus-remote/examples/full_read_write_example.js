import { check, fail } from 'k6';
import exec from 'k6/execution';
import remote from 'k6/x/remotewrite';
import encoding from 'k6/encoding';
import { Httpx } from 'https://jslib.k6.io/httpx/0.0.6/index.js';
import { randomIntBetween } from "https://jslib.k6.io/k6-utils/1.1.0/index.js";

const PROMETHEUS_TOKEN = __ENV.PROMETHEUS_TOKEN || fail("provide PROMETHEUS_TOKEN when strting k6");
const PROMETHEUS_USERNAME = __ENV.PROMETHEUS_USERNAME || fail("provide PROMETHEUS_USERNAME when strting k6");
const BASE_URL = 'prometheus-prod-10-prod-us-central-0.grafana.net';
const RW_UNIQUE_METRICS = 50;
const RW_SERIES_PER_METRIC = 50;

let write_client = new remote.Client({
  url: `https://${PROMETHEUS_USERNAME}:${PROMETHEUS_TOKEN}@${BASE_URL}/api/prom/push`
});

let query_client = new Httpx({
  baseURL: `https://${BASE_URL}/api/prom/api/v1`,
  headers: {
    'User-Agent': 'k6-load-test',
    "Content-Type": 'application/x-www-form-urlencoded',
    "Authorization": `Basic ${encoding.b64encode(`${PROMETHEUS_USERNAME}:${PROMETHEUS_TOKEN}`)}`
  },
  timeout: 20000 // 20s timeout.
});


export let options = {
  thresholds: {
    'checks': [{ threshold: 'rate==1', abortOnFail: true }],
    'remote_write_req_duration': [{ threshold: 'p(95) < 1000', abortOnFail: false }],
    'http_req_failed': [{ threshold: 'rate==0.00', abortOnFail: true }],
    'http_req_duration{name:query}': [{ threshold: 'p(95)<1000', abortOnFail: true }],
    'http_req_duration{name:query_range}': [{ threshold: 'p(95)<1000', abortOnFail: true }],
    'http_req_duration{name:query_exemplars}': [{ threshold: 'p(95)<1000', abortOnFail: true }],
  },

  scenarios: {
    writing_metrics: {
      executor: 'constant-arrival-rate',
      rate: RW_UNIQUE_METRICS,
      timeUnit: '15s',

      duration: '1m',
      preAllocatedVUs: 10,
      maxVus: 100,
      exec: 'write_scenario'
    },
    reading_metrics: {
      executor: 'constant-arrival-rate',
      rate: RW_UNIQUE_METRICS,
      timeUnit: '15s',

      duration: '1m',
      preAllocatedVUs: 10,
      maxVus: 100,
      exec: 'read_scenario'
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

export function read_scenario() {
  let metric_name = `k6_generated_metric_${exec.scenario.iterationInTest % RW_UNIQUE_METRICS}`

  promql_query_metric(metric_name, 300);
  promql_query_range(metric_name, 300);
  promql_query_examplars(metric_name, 300)
}

// Helper functions below

function generate_metrics(metric_name, numberOfSeries) {
  let timestamp = Date.now();

  //  console.log(`Writing batch of ${numberOfSeries} series. Metric name ${metric_name}`);

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

function promql_query_metric(metric_name, seconds) {
  let query = `sum(${metric_name})`

  //  console.log(`query ${seconds}s of data for ${query}.`);

  let res = query_client.post('/query', {
    'query': query,
    'time': Math.ceil(Date.now() / 1000) - seconds,
  }, { tags: { name: `query` } })

  check(res, {
    'prom query worked': (r) => r.status === 200,
  }) || fail(JSON.stringify(res));
}

function promql_query_range(metric_name, seconds) {
  let query = `sum(${metric_name})`

  //  console.log(`query_range ${seconds}s of data for ${query}.`);

  let res = query_client.post('/query_range', {
    'query': query,
    'start': Math.ceil(Date.now() / 1000) - seconds,
    'end': Math.ceil(Date.now() / 1000),
    'step': 15
  }, { tags: { name: `query_range` } })

  check(res, {
    'prom query_range worked': (r) => r.status === 200,
  }) || fail(JSON.stringify(res));
}

function promql_query_examplars(metric_name, seconds) {
  let query = `sum(${metric_name})`

  // console.log(`query_exemplars ${seconds}s of data for ${query}.`);

  let res = query_client.post('/query_exemplars', {
    'query': query,
    'start': Math.ceil(Date.now() / 1000) - 300,
    'end': Math.ceil(Date.now() / 1000),
  }, { tags: { name: `query_exemplars` } })

  check(res, {
    'prom query_exemplars worked': (r) => r.status === 200,
  }) || fail(JSON.stringify(res));
}
