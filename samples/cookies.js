import http from "k6/http";
import { check, group } from "k6";

export default function() {
    group("Simple cookies", function() {
        // Simple key/value cookies
        let cookies = {
            key: "value",
            key2: "value2"
        };

        // Send a request that will have the server set cookies in the response
        let r = http.get("http://httpbin.org/cookies", { cookies: cookies });

        // Verify response
        check(r, {
            "status is 200": (r) => r.status === 200,
            "cookie one is set": (r) => r.cookies["key"] === "value",
            "cookie two is set": (r) => r.cookies["key2"] === "value2",
        });
    });

    group("Advanced cookies", function() {
        // Set cookies with more advanced options
        let cookieJar = new http.CookieJar();
        cookieJar.set("key3", "value3", { domain: "httpbin.org", path: "/cookies" });
        cookieJar.set("key4", "value4", { domain: "httpbin.org", path: "/cookies", secure: true, httpOnly: true });

        // Send a request to cookie echoing service, not using TLS
        let r = http.get("http://httpbin.org/cookies", { cookies: cookieJar });

        // Verify response
        check(r, {
            "status is 200": (r) => r.status === 200,
            "cookie three is set": (r) => r.cookies["key3"] === "value3",
            "cookie four is not set": (r) => r.cookies["key4"] === undefined,  // since it's not "secure"!
        });

        // Send a request to cookie echoing service, using TLS
        r = http.get("https://httpbin.org/cookies", { cookies: cookieJar });

        // Verify response
        check(r, {
            "status is 200": (r) => r.status === 200,
            "cookie three is set": (r) => r.cookies["key3"] === "value3",
            "cookie four is set": (r) => r.cookies["key4"] === "value4",
        });
    });
}
