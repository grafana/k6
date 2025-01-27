import http from "k6/http";
import { check } from "k6";

export const options = {
  vus: 10,
  iterations: 1000,

  thresholds: {
    "http_reqs{expected_response:false}": ["rate>10"],
  },
};

export default function () {
  const responses = http.batch([
    ['GET', 'https://httpstat.us/200', null, { tags: { type: 'ok' } }],
    ['GET', 'https://httpstat.us/400', null, { tags: { type: 'Bad Request' } }],
    ['GET', 'https://httpstat.us/404', null, { tags: { type: 'Not Found' } }],
    ['GET', 'https://httpstat.us/500', null, { tags: { type: 'Internal Server Error' } }],
    ['GET', 'https://httpstat.us/502', null, { tags: { type: 'Bad Gateway' } }],
    ['GET', 'https://httpstat.us/503', null, { tags: { type: 'Service Unavailable' } }],
    ['GET', 'https://httpstat.us/504', null, { tags: { type: 'Gateway Timeout' } }],


  ]);
  check(responses[0], {
    'main page status was 200': (res) => res.status === 200,
  });
  check(responses[1], {
    'main page status was 400': (res) => res.status === 400,
  });
  check(responses[6], {
    'main page status was 504': (res) => res.status === 504,
  });
}