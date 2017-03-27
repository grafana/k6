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
        let r = http.get("http://httpbin.org/get?verb=get");
        let j = r.json();
        check(r, {
            "status is 200": (r) => r.status === 200,
            "is verb correct": (r) => j["args"]["verb"] === "get",
        });
    });

    // POST request
    group("POST", function() {
        let r = http.post("http://httpbin.org/post", { verb: "post" });
        let j = r.json();
        check(r, {
            "status is 200": (r) => r.status === 200,
            "is verb correct": (r) => j["form"]["verb"] === "post",
        });
    });

    // PUT request
    group("PUT", function() {
        let r = http.put("http://httpbin.org/put", JSON.stringify({ verb: "put" }), { headers: { "Content-Type": "application/json" }});
        let j = r.json();
        check(r, {
            "status is 200": (r) => r.status === 200,
            "is verb correct": (r) => j["json"]["verb"] === "put",
        });
    });

    // PATCH request
    group("PATCH", function() {
        let r = http.patch("http://httpbin.org/patch", JSON.stringify({ verb: "patch" }), { headers: { "Content-Type": "application/json" }});
        let j = r.json();
        check(r, {
            "status is 200": (r) => r.status === 200,
            "is verb correct": (r) => j["json"]["verb"] === "patch",
        });
    });

    // DELETE request
    group("DELETE", function() {
        let r = http.del("http://httpbin.org/delete?verb=delete");
        let j = r.json();
        check(r, {
            "status is 200": (r) => r.status === 200,
            "is verb correct": (r) => j["args"]["verb"] === "delete",
        });
    });
}
