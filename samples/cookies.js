import http from "k6/http";
import { check, group } from "k6";

export default function() {
    group("Simple cookies", function() {
        let cookies = {
            name: "value1",
            name2: "value2"
        };
        let res = http.get("http://httpbin.org/cookies", { cookies: cookies });
        check(res, {
            "status is 200": (r) => r.status === 200,
            "has cookie 'name'": (r) => r.cookies.name.length > 0,
            "has cookie 'name2'": (r) => r.cookies.name2.length > 0
        });
    });

    group("Advanced cookies", function() {
        let cookie = { name: "name3", value: "value3", domain: "httpbin.org", path: "/cookies" };
        let res = http.get("http://httpbin.org/cookies", { cookies: [cookie] });
        check(res, {
            "status is 200": (r) => r.status === 200,
            "has cookie 'name3'": (r) => r.cookies.name3.length > 0
        });
    });
}
