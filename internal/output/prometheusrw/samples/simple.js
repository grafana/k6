import http from "k6/http";

export const options = {
  vus: 10,
  iterations: 1000,

  thresholds: {
    "http_reqs{expected_response:false}": ["rate>10"],
  },
};

export default function () {
  http.get("https://test.k6.io");
}
