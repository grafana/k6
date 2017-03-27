import http from "k6/http";
import { check } from "k6";

export let options = {
    // When this option is enabled (set to true), mismatches in hostname
    // between target system and TLS certificate will be ignored
    insecureSkipTLSVerify: true
};

export default function() {
    let r = http.get("https://httpbin.org/");
    check(r, { "status is 200": (r) => r.status === 200 });
}
