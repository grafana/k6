import encoding from "k6/encoding";
import http from "k6/http";
import { check } from "k6";

export default function() {
    // Passing username and password as part of URL will authenticate using HTTP Basic Auth
    let res = http.get("http://user:passwd@httpbin.org/basic-auth/user/passwd");

    // Verify response
    check(res, {
        "status is 200": (r) => r.status === 200,
        "is authenticated": (r) => r.json().authenticated === true,
        "is correct user": (r) => r.json().user === "user"
    });

    // Alternatively you can create the header yourself to authenticate using HTTP Basic Auth
    res = http.get("http://httpbin.org/basic-auth/user/passwd", { headers: { "Authorization": "Basic " + encoding.b64encode("user:passwd") }});

    // Verify response
    check(res, {
        "status is 200": (r) => r.status === 200,
        "is authenticated": (r) => r.json().authenticated === true,
        "is correct user": (r) => r.json().user === "user"
    });
}
