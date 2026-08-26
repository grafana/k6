import { check, sleep } from 'k6';
import remote from 'k6/x/remotewrite';
import { randomIntBetween } from "https://jslib.k6.io/k6-utils/1.1.0/index.js";

// This example is similar to Promtool's default workload file: https://github.com/grafana/cortex-tools/blob/main/docs/benchtool.md#example-workload-file

export let options = {
    vus: 5,
    duration: '5m',
};

const client = new remote.Client({
    url: "<your-remote-write-url>"
});

export default function () {
    let res = client.store([{
        "labels": [
            { "name": "__name__", "value": `metric_gauge_random_01` },
            { "name": "label_01", "value": `label_value_01_${randomIntBetween(1, 5)}` },
            { "name": "label_02", "value": `label_value_02_${randomIntBetween(1, 20)}` },
            { "name": "replica", "value": __VU.toString() }
        ],
        "samples": generateRandomSamples(1000)
    }, {
        "labels": [
            { "name": "__name__", "value": `metric_gauge_zero_01` },
            { "name": "label_01", "value": `label_value_01_${randomIntBetween(1, 5)}` },
            { "name": "label_02", "value": `label_value_02_${randomIntBetween(1, 20)}` },
            { "name": "replica", "value": __VU.toString() }
        ],
        "samples": generateEmptySamples(1000)
    }]);
    check(res, {
        'is status 2xx': (r) => r.status >= 200 && r.status < 300,
    });
    sleep(0.15);
}

function generateRandomSamples(number) {
    let samples = []
    for (let i = 0; i < number; i++) {
        samples.push({ "value": Math.random(), })
    }
    return samples
}

function generateEmptySamples(number) {
    let samples = []
    for (let i = 0; i < number; i++) {
        samples.push({ "value": 0, })
    }
    return samples
}