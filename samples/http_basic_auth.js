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
}
