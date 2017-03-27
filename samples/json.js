import http from "k6/http";
import { check } from "k6";

export default function() {
    // Send a JSON encoded POST request
    let body = JSON.stringify({ key: "value" });
    let r = http.post("http://httpbin.org/post", body, { headers: { "Content-Type": "application/json" }});

    // Use JSON.parse to deserialize the JSON (instead of using the r.json() method)
    let j = JSON.parse(r.body);

    // Verify response
    check(r, {
        "status is 200": (r) => r.status === 200,
        "is key correct": (r) => j["json"]["key"] === "value",
    });
}
