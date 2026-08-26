import { check, fail } from 'k6';
import exec from 'k6/execution';
import encoding from 'k6/encoding';
import { Httpx } from 'https://jslib.k6.io/httpx/0.0.6/index.js';

const PROMETHEUS_TOKEN = __ENV.PROMETHEUS_TOKEN || fail("provide -e PROMETHEUS_TOKEN when strting k6");
const PROMETHEUS_USERNAME =  __ENV.PROMETHEUS_USERNAME || fail("provide -e PROMETHEUS_USERNAME when strting k6");
const BASE_URL = 'prometheus-prod-10-prod-us-central-0.grafana.net';

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
    'checks': [{threshold: 'rate==1', abortOnFail: true}],
    'http_req_failed': [{threshold: 'rate==0.00', abortOnFail: true}],
    'http_req_duration{name:query}': [{threshold: 'p(95)<1000', abortOnFail: true}],
    'http_req_duration{name:query_range}': [{threshold: 'p(95)<1000', abortOnFail: true}],
    'http_req_duration{name:query_exemplars}': [{threshold: 'p(95)<1000', abortOnFail: true}],
  },
  scenarios: {
    reading_metrics: {
      executor: 'constant-arrival-rate',
      rate: 50, // 50 reads per 15s
      timeUnit: '15s',

      duration: '5m',
      preAllocatedVUs: 1,
      maxVus: 100,
      exec: 'read_scenario'
    },
  },
};

export function read_scenario() {
  let metric_id = exec.scenario.iterationInTest % 50;
  let metric_name = `k6_generated_metric_${metric_id}`

  promql_query_metric(metric_name, 300);
  promql_query_range(metric_name, 300);
  promql_query_examplars(metric_name, 300)
}

// Helper functions below

function promql_query_metric(metric_name, seconds){
  let query = `sum(${metric_name})`

  //console.log(`query ${seconds}s of data for ${query}.`);

  let res = query_client.post('/query', {
    'query': query,
    'time': Math.ceil(Date.now()/1000) - seconds,
  }, {tags: {name: `query`}})

  check(res, {
    'prom query worked': (r) => r.status === 200,
  }) || fail(JSON.stringify(res));
}

function promql_query_range(metric_name, seconds){
  let query = `sum(${metric_name})`

  //console.log(`query_range ${seconds}s of data for ${query}.`);

  let res = query_client.post('/query_range', {
    'query': query,
    'start': Math.ceil(Date.now()/1000) - seconds, 
    'end': Math.ceil(Date.now()/1000),
    'step': 15
  }, {tags: {name: `query_range`}})

  check(res, {
    'prom query_range worked': (r) => r.status === 200,
  }) || fail(JSON.stringify(res));
}

function promql_query_examplars(metric_name, seconds){
  let query = `sum(${metric_name})`

  //console.log(`query_exemplars ${seconds}s of data for ${query}.`);

  let res = query_client.post('/query_exemplars', {
    'query': query,
    'start': Math.ceil(Date.now()/1000) - 300,
    'end': Math.ceil(Date.now()/1000),
  }, {tags: {name: `query_exemplars`}})

  check(res, {
    'prom query_exemplars worked': (r) => r.status === 200,
  }) || fail(JSON.stringify(res));
}

