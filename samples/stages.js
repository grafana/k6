import http from "k6/http";
import { check } from "k6";
// for test stages on dashboard
export const options = {
  stages: [
    // Ramp-up from 1 to 5 VUs in 10s
    { duration: "10s", target: 5 },

    // Stay at rest on 5 VUs for 5s
    { duration: "5s", target: 5 },

    // Ramp-up from 5 to 10 VUs in 5s
    { duration: "5s", target: 10 },

    // Stay at rest on 10 VUs for 5s
    { duration: "10s", target: 10 },

    // Ramp-down from 5 to 0 VUs for 5s
    { duration: "5s", target: 0 }
  ],
  thresholds: {
    'http_reqs{expected_response:true}': ['rate>10'],
  },
};

export default function () {
  check(http.get("https://test-api.k6.io/"), {
    "status is 200": (r) => r.status == 200,
    "protocol is HTTP/2": (r) => r.proto == "HTTP/2.0",
  });
}
