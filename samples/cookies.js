import http from "k6/http";
import { check, group } from "k6";

export default function() {
    group("Simple cookies", function() {
        let cookies = {
            name: "value1",
            name2: "value2"
        };
        let r = http.get("http://httpbin.org/cookies", { cookies: cookies });
        check(r, {
            "status is 200": (r) => r.status === 200,
            "has cookie": (r) => r.cookies["name"].length > 0
        });
    });

    group("Advanced cookies", function() {
        let cookie = { name: "name3", value: "value3", domain: "httpbin.org", path: "/cookies" };
        let r = http.get("http://httpbin.org/cookies", { cookies: [cookie] });
        check(r, {
            "status is 200": (r) => r.status === 200,
            "has cookie": (r) => r.cookies["name3"].length > 0
        });
    });
}
