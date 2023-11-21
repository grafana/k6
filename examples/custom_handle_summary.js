import http from "k6/http";
import { check } from "k6";

export default function () {
    let res = http.get("http://httpbin.org/");
    check(res, { "status is 200": (r) => r.status === 200 });
}

/*
 * With handleSummary(), you can completely customize your
 * end-of-test summary.
 *
 * k6 expects handleSummary() to return a {key1: value1, key2: value2, ...}
 * map that represents the summary metrics.
 *
 * The keys must be strings. They determine where k6 displays or saves
 * the content:
 *
 * - stdout for standard output
 * - stderr for standard error,
 * - any relative or absolute path to a file on the system (this operation
 * overwrites existing files).
 *
 * You can return multiple summary outputs in a script.
 * In this example, we return statement sends a report and writes the data
 * object to different JSON files.
 */

export function handleSummary(data) {
    return {
        "./data/summary.json": JSON.stringify(data),
        "summary2.json": JSON.stringify(data)
    };
}