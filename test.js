import { Rate } from 'k6/metrics';
import { sleep, check } from 'k6';
import k8s from 'k8s';


const degraded = new Rate('degraded');

export const options = {
  iterations: 1,
  threshold: {
    degraded: ['rate<=0'],
    htt_req_duration: ['p(95)<500'],
  },
};

const sleepDuration = 1;
const namespace = 'default';

export default function () {

  let pods = k8s.list(namespace)
  let count = pods.length

  console.log(`Pod count is ${count}.`)

  let podsToKill = pods.filter(x => x.indexOf('webserver') !== -1)

  console.log(`Killing ${podsToKill.length} pods in total`)
  podsToKill.forEach(p => {
    k8s.kill(namespace, p)
  })

  console.log(`Sleeping for ${sleepDuration} seconds`)
  sleep(sleepDuration)

  pods = k8s.list(namespace);
  console.log(`Pod count is ${count}.`)

  check(null, {
    'deployment recovered': count === pods.length
  });
  degraded.add(count === k8s.list(namespace));
}
