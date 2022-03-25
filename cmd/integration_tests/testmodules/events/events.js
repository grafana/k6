import exec from 'k6/execution';
import { setTimeout, clearTimeout, setInterval, clearInterval } from 'k6/events'


export let options = {
    scenarios: {
        'foo': {
            executor: 'constant-vus',
            vus: 1,
            duration: '3.8s',
            gracefulStop: '0s',
        }
    }
};

function debug(arg) {
    let t = String((new Date()) - exec.scenario.startTime).padStart(6, ' ')
    console.log(`[${t}ms, iter=${exec.scenario.iterationInTest}] ${arg}`);
}

export default function () {
    debug('default start');

    let tickCount = 1;
    let f0 = (arg) => {
        debug(`${arg} ${tickCount++}`);
    }
    let t0 = setInterval(f0, 500, 'tick')

    let f1 = (arg) => {
        debug(arg);
        clearInterval(t0);
    }
    let t1 = setTimeout(f1, 2000, 'third');

    let t2 = setTimeout(debug, 1500, 'never happening');

    let f3 = (arg) => {
        debug(arg);
        clearTimeout(t2);
        setTimeout(debug, 600, 'second');
    }
    let t3 = setTimeout(f3, 1000, 'first');

    debug('default end');
    if (exec.scenario.iterationInTest == 1) {
        debug(`expected last iter, the interval ID is ${t0}, we also expect timer ${t1} to be interrupted`)
    }
}
