// Run with:
// k6 run --features async-metric-context examples/async_metric_context.js

import { check } from 'k6';
import exec from 'k6/execution';
import { Counter } from 'k6/metrics';

const callbackPhases = new Counter('callback_phases');

export const options = {
  iterations: 1,
  thresholds: {
    callback_phases: ['count == 6'],
    checks: ['rate == 1'],
  },
};

function setContext(owner, phase) {
  exec.vu.metrics.tags.owner = owner;
  exec.vu.metrics.tags.phase = phase;
  exec.vu.metrics.metadata.trace = owner;
  exec.vu.metrics.metadata.step = phase;
}

function contextMatches(owner, phase) {
  return exec.vu.metrics.tags.owner === owner
    && exec.vu.metrics.tags.phase === phase
    && exec.vu.metrics.metadata.trace === owner
    && exec.vu.metrics.metadata.step === phase;
}

export default async function () {
  let releaseAlpha;
  let releaseBeta;
  const alphaGate = new Promise(resolve => { releaseAlpha = resolve; });
  const betaGate = new Promise(resolve => { releaseBeta = resolve; });

  const register = (owner, gate) => {
    setContext(owner, 'registered');
    return gate.then(async () => {
      for (const phase of ['first', 'second', 'third']) {
        setContext(owner, phase);
        await Promise.resolve();
        callbackPhases.add(1);
        check(null, {
          [`${owner}:${phase} kept its context`]: () => contextMatches(owner, phase),
        });
      }
    });
  };

  const alpha = register('alpha', alphaGate);
  const beta = register('beta', betaGate);

  setContext('root', 'waiting');
  releaseBeta();
  releaseAlpha();
  await Promise.all([alpha, beta]);

  check(null, {
    'the awaiting context was restored': () => contextMatches('root', 'waiting'),
  });
}
