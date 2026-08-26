import { check, sleep } from 'k6';
import remote from 'k6/x/remotewrite';

export let options = {
    vus: 10,
    duration: '10s',
};

const client = new remote.Client({
    url: "<remote-write-URL>"
});

export default function () {
    let res = client.store([{
        "labels": [
            { "name": "__name__", "value": `my_cool_metric_${__VU}` },
            { "name": "service", "value": "bar" }
        ],
        "samples": [
            { "value": Math.random() * 100, }
        ]
    }]);
    check(res, {
        'is status 2xx': (r) => r.status >= 200 && r.status < 300,
    });
    sleep(1)
}