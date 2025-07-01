import http from 'k6/http';
import { check, sleep } from 'k6';

// Test configuration
export const options = {
  stages: [
    { duration: '2m', target: 10 }, // Ramp up to 10 users
    { duration: '5m', target: 10 }, // Stay at 10 users
    { duration: '2m', target: 0 },  // Ramp down to 0 users
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests must complete below 500ms
    http_req_failed: ['rate<0.01'],   // Error rate must be below 1%
  },{{ if .ProjectID }}
  cloud: {
    projectID: {{ .ProjectID }},
    name: "{{ .ScriptName }}",
  },{{ end }}
};

// Base URL for the API
const BASE_URL = __ENV.BASE_URL || 'https://httpbin.org';

export default function() {
  // Test GET request
  let getResponse = http.get(`${BASE_URL}/get`);
  check(getResponse, {
    'GET status is 200': (r) => r.status === 200,
    'GET response time < 500ms': (r) => r.timings.duration < 500,
  });

  // Test POST request with JSON payload
  const payload = JSON.stringify({
    username: 'testuser',
    password: 'testpass123'
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
  };

  let postResponse = http.post(`${BASE_URL}/post`, payload, params);
  check(postResponse, {
    'POST status is 200': (r) => r.status === 200,
    'POST response time < 500ms': (r) => r.timings.duration < 500,
    'POST response contains data': (r) => r.json().data !== undefined,
  });

  // Test PUT request
  let putResponse = http.put(`${BASE_URL}/put`, payload, params);
  check(putResponse, {
    'PUT status is 200': (r) => r.status === 200,
  });

  // Test DELETE request
  let deleteResponse = http.del(`${BASE_URL}/delete`);
  check(deleteResponse, {
    'DELETE status is 200': (r) => r.status === 200,
  });

  // Brief pause between iterations
  sleep(1);
}

// Setup function - runs once before the test starts
export function setup() {
  console.log('Starting REST API test...');
  console.log(`Base URL: ${BASE_URL}`);
  
  // Verify the API is accessible
  let response = http.get(`${BASE_URL}/get`);
  if (response.status !== 200) {
    throw new Error(`Setup failed: API returned status ${response.status}`);
  }
  
  return { baseUrl: BASE_URL };
}

// Teardown function - runs once after the test ends
export function teardown(data) {
  console.log('REST API test completed');
  console.log(`Base URL used: ${data.baseUrl}`);
} 