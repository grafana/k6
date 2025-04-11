
import { check } from 'k6';
import jsexttest from 'k6/x/jsexttest';

export let options = {
    iterations: 5,
    thresholds: {
        checks: ['rate===1'],
    }
};

export function handleSummary(data) {
    return {
        'summary-results.txt': data.metrics.foos.values.count.toString(),
    };
}


export default function () {
    check(null, {
        "foo is true": () => jsexttest.foo(__ITER),
    });
}