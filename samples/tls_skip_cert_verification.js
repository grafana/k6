import http from "k6/http";
import { check } from "k6";

export let options = {
    // When this option is enabled (set to true), all of the verifications
    // that would otherwise be done to establish trust in a server provided
    // TLS certificate will be ignored.
    insecureSkipTLSVerify: true
};

export default function() {
    let res = http.get("https://httpbin.org/");
    check(res, { "status is 200": (r) => r.status === 200 });
}
