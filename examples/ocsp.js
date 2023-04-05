import http from "k6/http";
import { check } from "k6";

export default function() {
    let res = http.get("https://stackoverflow.com");
    check(res, {
        "is OCSP response good": (r) => r.ocsp.status === http.OCSP_STATUS_GOOD
    });
};
