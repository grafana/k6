import http from "k6/http";
import { check } from "k6";

/*
 * Thresholds are used to specify where a metric crosses into unacceptable
 * territory. If a threshold is crossed the test is considered a failure
 * and is marked as such by the program through a non-zero exit code.
 * 
 * Thresholds are specified as part of the options structure. It's a set of
 * key/value pairs where the name specifies the metric to watch (with optional 
 * tag filtering) and the values are JS expressions. Which could be a simple
 * number or involve a statistical aggregate like avg, max, percentiles etc.
 */

export let options = {
    thresholds: {
        // Declare a threshold over all HTTP response times,
        // the 95th percentile should not cross 500ms
        http_req_duration: ["p(95)<500"],

        // Declare a threshold over HTTP response times for all data points
        // where the URL tag is equal to "http://httpbin.org/post",
        // the max should not cross 1000ms
        "http_req_duration{name:http://httpbin.org/post}": ["max<1000"],
    }
};

export default function() {
    http.get("http://httpbin.org/");
    http.post("http://httpbin.org/post", {data: "some data"});
}
