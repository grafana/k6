import { Rate } from 'k6/metrics';
import { sleep, check } from 'k6';
import http from 'k6/http';
import k8s from 'k8s';

const degraded = new Rate('degraded');
const failed = new Rate('http_req_failed');

export const options = {
  threshold: {
    degraded: ['rate<=0'],
    http_req_duration: ['p(95)<500'],
    http_req_failed: ['rate<0.1']
  },
  scenarios: {
    attack: {
      executor: 'shared-iterations',
      startTime: '10s',
      exec: 'attack',
      vus: 1,
      iterations: 10,
    },
    probe: {
      executor: 'constant-arrival-rate',
      rate: 100,  // 200 RPS, since timeUnit is the default 1s
      duration: '1m',
      preAllocatedVUs: 50,
      maxVUs: 100,
      exec: 'probe'
    }
  },
  ext: {
    chaos: {
      hypothesis: `
        When we inject a pod failure into a deployment while under normal load,
        the performance of our system will not be degraded more than 10%.
      `
    }
  }
};

const sleepDuration = 1;
const namespace = 'default';

function killPodsByPattern(pods, regex) {
  const candidates = pods.filter(x =>  regex.test(x));
  const half_length = Math.floor(candidates.length / 3);
  const targets = candidates.slice(0,half_length);

  console.log(`Killing ${targets.length} pods out of ${candidates.length}`);

  targets.forEach(p => {
    k8s.kill(namespace, p);
  });
}

export function probe() {
  const r = http.get('http://10.7.10.201:8080')
  failed.add(r.status !== 200)
  sleep(1)
}

export function attack() {

  let pods = k8s.list(namespace);
  let count = pods.length;

  killPodsByPattern(pods, /^hello-world/);
  sleep(sleepDuration);

  pods = k8s.list(namespace);

  const recovered = count === pods.length;
  check(null, { 'deployment recovered': recovered });
  degraded.add(!recovered);
}
