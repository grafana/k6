import http from "k6/http";
import { check, group } from "k6";

/*
 * k6 supports all standard HTTP verbs/methods:
 * CONNECT, DELETE, GET, HEAD, OPTIONS, PATCH, POST, PUT and TRACE.
 * 
 * Below are examples showing how to use the most common of these.
 */

export default function() {
    // GET request
    group("GET", function() {
        let res = http.get("http://httpbin.org/get?verb=get");
        check(res, {
            "status is 200": (r) => r.status === 200,
            "is verb correct": (r) => r.json().args.verb === "get",
        });
    });

    // POST request
    group("POST", function() {
        let res = http.post("http://httpbin.org/post", { verb: "post" });
        check(res, {
            "status is 200": (r) => r.status === 200,
            "is verb correct": (r) => r.json().form.verb === "post",
        });
    });

    // PUT request
    group("PUT", function() {
        let res = http.put("http://httpbin.org/put", JSON.stringify({ verb: "put" }), { headers: { "Content-Type": "application/json" }});
        check(res, {
            "status is 200": (r) => r.status === 200,
            "is verb correct": (r) => r.json().json.verb === "put",
        });
    });

    // PATCH request
    group("PATCH", function() {
        let res = http.patch("http://httpbin.org/patch", JSON.stringify({ verb: "patch" }), { headers: { "Content-Type": "application/json" }});
        check(res, {
            "status is 200": (r) => r.status === 200,
            "is verb correct": (r) => r.json().json.verb === "patch",
        });
    });

    // DELETE request
    group("DELETE", function() {
        let res = http.del("http://httpbin.org/delete?verb=delete");
        check(res, {
            "status is 200": (r) => r.status === 200,
            "is verb correct": (r) => r.json().args.verb === "delete",
        });
    });
}
