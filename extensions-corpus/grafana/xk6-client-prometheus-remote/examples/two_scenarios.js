import { check, sleep } from 'k6';
import remote from 'k6/x/remotewrite';

export let options = {
    scenarios: {
        sendBatch: {
            executor: 'constant-vus',
            exec: 'sendBatch',
            vus: 10,
            duration: '1m',
        },
        increaseCounter: {
            executor: 'constant-vus',
            exec: 'increaseCounter',
            vus: 1,
            duration: '1m',
        },
    }
};

const client = new remote.Client({
    url: "<your-remote-write-url>",
    user_agent: "k6/advanced-script",
    tenant_name: "<my-name>",
    timeout: "20s"
});

export function increaseCounter() {
    let res = client.store([{
        "labels": [
            { "name": "__name__", "value": `my_happy_metric_total` },
        ],
        "samples": [
            { "value": __ITER, }
        ]
    },
    ]);
    check(res, {
        'is status 2xx': (r) => r.status >= 200 && r.status < 300,
    });
    sleep(10);
}
export function sendBatch() {
    let res = client.store([{
        "labels": [
            { "name": "__name__", "value": `my_cool_metric_${__VU}` },
            { "name": "service", "value": "bar" }
        ],
        "samples": [
            { "value": Math.random() * 100, }
        ]
    },
    {
        "labels": [
            { "name": "__name__", "value": `my_fancy_metric_${__VU}` },
        ],
        "samples": [
            { "value": Math.random() * 100 },
            { "value": Math.random() * 100 }
        ]
    }
    ]);
    check(res, {
        'is status 2xx': (r) => r.status >= 200 && r.status < 300,
    });
    sleep(1)
}