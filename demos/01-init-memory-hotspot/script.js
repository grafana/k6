import { check, sleep } from "k6";

export const options = {
  vus: 10,
  duration: "10s",
};

function allocateBurst(iter) {
  const out = [];
  for (let i = 0; i < 200; i++) {
    out.push({
      iter,
      i,
      ts: Date.now(),
      payload: `value-${iter}-${i}`,
      nested: { a: i % 3, b: i % 7 },
    });
  }
  return out;
}

function cpuBurst() {
  let acc = 0;
  for (let i = 1; i < 50000; i++) {
    acc += Math.sqrt(i) / (i % 13 + 1);
  }
  return acc;
}

async function asyncBurst(iter) {
  const out = [];
  for (let i = 0; i < 120; i++) {
    await Promise.resolve(i + iter);
    out.push({ i, payload: `async-${iter}-${i}` });
  }
  return out.length;
}

export default async function () {
  const alloc = allocateBurst(__ITER);
  const cpu = cpuBurst();
  const asyncCount = await asyncBurst(__ITER);
  check(
    { alloc, cpu, asyncCount },
    {
      "cpu result is positive": (v) => v.cpu > 0,
      "async count is positive": (v) => v.asyncCount > 0,
    },
  );
  sleep(0.1);
}
