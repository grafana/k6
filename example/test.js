import http from 'k6/http';

export const options = {
  vus: 10,
  duration: '10s',
  thresholds: {
    'http_reqs{expected_response:true}': ['rate>10'],
  },
  // Adding a tag to distinguish discrete test runs
  tags: {
    testid: `testrun-${Date.now()}`,
  }
};

export default function () {
  http.get('https://test.k6.io/');
}
