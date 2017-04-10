import http from "k6/http";
import { Trend } from "k6/metrics";
import { check } from "k6";

/*
 * Checks, custom metrics and requests can be tagged with any number of tags.
 *
 * Tags can be used for:
 * - Creating metric thresholds by filtering the metric data stream based on tags
 * - Aid result analysis by allowing for more precise filtering of metrics
 */

let myTrend = new Trend("my_trend");

export default function() {
    // Add tag to request metric data
    let res = http.get("http://httpbin.org/", { tags: { my_tag: "I'm a tag" } });

    // Add tag to check
    check(res, { "status is 200": (r) => r.status === 200 }, { my_tag: "I'm a tag" });

    // Add tag to custom metric
    myTrend.add(res.timings.connecting, { my_tag: "I'm a tag" });
}
