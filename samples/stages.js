import http from "k6/http";
import { check } from "k6";


/*
 * Stages (aka ramping) is how you, in code, specify the ramping of VUs.
 * That is, how many VUs should be active and generating traffic against
 * the target system at any specific point in time for the duration of
 * the test.
 * 
 * The following stages configuration will result in up-flat-down ramping
 * profile over a 20s total test duration.
 */ 

export let options = {
    stages: [
        // Ramp-up from 1 to 5 VUs in 10s
        { duration: "10s", target: 5 },

        // Stay at rest on 5 VUs for 5s
        { duration: "5s", target: 5 },

        // Ramp-down from 5 to 0 VUs for 5s
        { duration: "5s", target: 0 }
    ]
};

export default function() {
    let res = http.get("http://httpbin.org/");
    check(res, { "status is 200": (r) => r.status === 200 });
}
