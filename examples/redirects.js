import http from "k6/http";
import {check} from "k6";

export let options = {
    // Max redirects to follow (default is 10)
    maxRedirects: 5
};

export default function() {
    // If redirecting more than options.maxRedirects times, the last response will be returned
    let res = http.get("https://httpbin.org/redirect/6");
    check(res, {
        "is status 302": (r) => r.status === 302
    });

    // The number of redirects to follow can be controlled on a per-request level as well
    res = http.get("https://httpbin.org/redirect/1", {redirects: 1});
    console.log(res.status);
    check(res, {
        "is status 200": (r) => r.status === 200,
        "url is correct": (r) => r.url === "https://httpbin.org/get"
    });
}
