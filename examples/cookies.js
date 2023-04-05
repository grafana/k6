import http from "k6/http";
import { check, group } from "k6";

export let options = {
    maxRedirects: 3
};

export default function() {
    // VU cookie jar
    group("Simple cookies send with VU jar", function() {
        let cookies = {
            name: "value1",
            name2: "value2"
        };
        let res = http.get("http://httpbin.org/cookies", { cookies: cookies });
        check(res, {
            "status is 200": (r) => r.status === 200,
            "has cookie 'name'": (r) => r.json().cookies.name.length > 0,
            "has cookie 'name2'": (r) => r.json().cookies.name2.length > 0
        });

        // Since the cookies are set as "request cookies" they won't be added to VU cookie jar
        let vuJar = http.cookieJar();
        let cookiesForURL = vuJar.cookiesForURL(res.url);
        check(null, {
            "vu jar doesn't have cookie 'name'": () => cookiesForURL.name === undefined,
            "vu jar doesn't have cookie 'name2'": () => cookiesForURL.name2 === undefined
        });
    });

    group("Simple cookies set with VU jar", function() {
        // Since this request redirects the `res.cookies` property won't contain the cookies
        let res = http.get("http://httpbin.org/cookies/set?name3=value3&name4=value4");
        check(res, {
            "status is 200": (r) => r.status === 200
        });

        // Make sure cookies have been added to VU cookie jar
        let vuJar = http.cookieJar();
        let cookiesForURL = vuJar.cookiesForURL(res.url);
        check(null, {
            "vu jar has cookie 'name3'": () => cookiesForURL.name3.length > 0,
            "vu jar has cookie 'name4'": () => cookiesForURL.name4.length > 0
        });
    });

    // Local cookie jar
    group("Simple cookies send with local jar", function() {
        let jar = new http.CookieJar();
        let cookies = {
            name5: "value5",
            name6: "value6"
        };
        let res = http.get("http://httpbin.org/cookies", { cookies: cookies, jar: jar });
        check(res, {
            "status is 200": (r) => r.status === 200,
            "has cookie 'name5'": (r) => r.json().cookies.name5.length > 0,
            "has cookie 'name6'": (r) => r.json().cookies.name6.length > 0
        });

        // Since the cookies are set as "request cookies" they won't be added to VU cookie jar
        let cookiesForURL = jar.cookiesForURL(res.url);
        check(null, {
            "local jar doesn't have cookie 'name5'": () => cookiesForURL.name5 === undefined,
            "local jar doesn't have cookie 'name6'": () => cookiesForURL.name6 === undefined
        });

        // Make sure cookies have NOT been added to VU cookie jar
        let vuJar = http.cookieJar();
        cookiesForURL = vuJar.cookiesForURL(res.url);
        check(null, {
            "vu jar doesn't have cookie 'name5'": () => cookiesForURL.name === undefined,
            "vu jar doesn't have cookie 'name6'": () => cookiesForURL.name2 === undefined
        });
    });

    group("Advanced send with local jar", function() {
        let jar = new http.CookieJar();
        jar.set("http://httpbin.org/cookies", "name7", "value7");
        jar.set("http://httpbin.org/cookies", "name8", "value8");
        let res = http.get("http://httpbin.org/cookies", { jar: jar });
        let cookiesForURL = jar.cookiesForURL(res.url);
        check(res, {
            "status is 200": (r) => r.status === 200,
            "has cookie 'name7'": (r) => r.json().cookies.name7.length > 0,
            "has cookie 'name8'": (r) => r.json().cookies.name8.length > 0
        });

        cookiesForURL = jar.cookiesForURL(res.url);
        check(null, {
            "local jar has cookie 'name7'": () => cookiesForURL.name7.length > 0,
            "local jar has cookie 'name8'": () => cookiesForURL.name8.length > 0
        });

        // Make sure cookies have NOT been added to VU cookie jar
        let vuJar = http.cookieJar();
        cookiesForURL = vuJar.cookiesForURL(res.url);
        check(null, {
            "vu jar doesn't have cookie 'name7'": () => cookiesForURL.name7 === undefined,
            "vu jar doesn't have cookie 'name8'": () => cookiesForURL.name8 === undefined
        });
    });

    group("Advanced cookie attributes", function() {
        let jar = http.cookieJar();
        jar.set("http://httpbin.org/cookies", "name9", "value9", { domain: "httpbin.org", path: "/cookies" });

        let res = http.get("http://httpbin.org/cookies", { jar: jar });
        check(res, {
            "status is 200": (r) => r.status === 200,
            "has cookie 'name9'": (r) => r.json().cookies.name9 === "value9"
        });

        jar.set("http://httpbin.org/cookies", "name10", "value10", { domain: "example.com", path: "/" });
        res = http.get("http://httpbin.org/cookies", { jar: jar });
        check(res, {
            "status is 200": (r) => r.status === 200,
            "doesn't have cookie 'name10'": (r) => r.json().cookies.name10 === undefined
        });
    });
}
