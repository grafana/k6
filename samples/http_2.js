import http from "k6/http";
import { check } from "k6";

export const options = {
  vus: 10,
  duration: '10s',
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
