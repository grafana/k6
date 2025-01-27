import http from "k6/http";
import { check } from "k6";
import { Counter, Gauge, Rate, Trend } from "k6/metrics";
let myCounter = new Counter("my_counter");
let myGauge = new Gauge("my_gauge");
let myRate = new Rate("my_rate");
let myTrend = new Trend("my_trend");

let maxResponseTime = 0.0;

export const options = {
  stages: [
    // Ramp-up from 1 to 5 VUs in 10s
    { duration: "10s", target: 5 },

    // Stay at rest on 5 VUs for 5s
    { duration: "5s", target: 5 },

    // Ramp-down from 5 to 0 VUs for 5s
    { duration: "5s", target: 0 }
  ],

};

export default function () {
  let res = http.get("https://httpbin.test.k6.io/");
 

//for test error on dashboard
  const responses = http.batch([
    "http://test.k6.io",
    "https://httpstat.us/500",
    "https://httpstat.us/404",
  ]);

  check(responses[0], {
    "main page 200": res => res.status === 200,
  });

  check(responses[1], {
    "pi page 200": res => res.status === 200,
    "pi page has right content": res => res.body === "2",
  });

}
