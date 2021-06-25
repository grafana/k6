import http from "k6/http";
import { Counter, Gauge, Rate, Trend } from "k6/metrics";
import { check } from "k6";

/*
 * Custom metrics are useful when you want to track something that is not
 * provided out of the box.
 *
 * There are four types of custom metrics: Counter, Gauge, Rate and Trend.
 *
 * - Counter: a sum of all values added to the metric
 * - Gauge: a value that change to whatever you set it to
 * - Rate: rate of "truthiness", how many values out of total are !=0
 * - Trend: time series, all values are recorded, statistics can be calculated
 *          on it
 */

let myCounter = new Counter("my_counter");
let myGauge = new Gauge("my_gauge");
let myRate = new Rate("my_rate");
let myTrend = new Trend("my_trend");

let maxResponseTime = 0.0;

export default function () {
    let res = http.get("http://httpbin.org/");
    let passed = check(res, { "status is 200": (r) => r.status === 200 });

    // Add one for number of requests
    myCounter.add(1);
    console.log(myCounter.name, " is config ready")

    // Set max response time seen
    maxResponseTime = Math.max(maxResponseTime, res.timings.duration);
    myGauge.add(maxResponseTime);

    // Add check success or failure to keep track of rate
    myRate.add(passed);

    // Keep track of TCP-connecting and TLS handshaking part of the response time
    myTrend.add(res.timings.connecting + res.timings.tls_handshaking);
}
