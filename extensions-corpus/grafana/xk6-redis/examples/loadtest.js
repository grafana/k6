import { check } from "k6";
import http from "k6/http";
import redis from "k6/x/redis";
import exec from "k6/execution";
import { textSummary } from "https://jslib.k6.io/k6-summary/0.0.1/index.js";

export const options = {
  scenarios: {
    redisPerformance: {
      executor: "shared-iterations",
      vus: 10,
      iterations: 200,
      exec: "measureRedisPerformance",
    },
    usingRedisData: {
      executor: "shared-iterations",
      vus: 10,
      iterations: 200,
      exec: "measureUsingRedisData",
    },
  },
};

// Instantiate a new Redis client using a URL
const redisClient = new redis.Client(
  // URL in the form of redis[s]://[[username][:password]@][host][:port][/db-number
  __ENV.REDIS_URL || "redis://localhost:6379",
);

// Prepare an array of crocodile ids for later use
// in the context of the measureUsingRedisData function.
const crocodileIDs = new Array(0, 1, 2, 3, 4, 5, 6, 7, 8, 9);

export function measureRedisPerformance() {
  // VUs are executed in a parallel fashion,
  // thus, to ensure that parallel VUs are not
  // modifying the same key at the same time,
  // we use keys indexed by the VU id.
  const key = `foo-${exec.vu.idInTest}`;

  redisClient
    .set(`foo-${exec.vu.idInTest}`, 1)
    .then(() => redisClient.get(`foo-${exec.vu.idInTest}`))
    .then((value) => redisClient.incrBy(`foo-${exec.vu.idInTest}`, value))
    .then((_) => redisClient.del(`foo-${exec.vu.idInTest}`))
    .then((_) => redisClient.exists(`foo-${exec.vu.idInTest}`))
    .then((exists) => {
      if (exists !== 0) {
        throw new Error("foo should have been deleted");
      }
    });
}

export function setup() {
  redisClient.sadd("crocodile_ids", ...crocodileIDs);
}

export function measureUsingRedisData() {
  // Pick a random crocodile id from the dedicated redis set,
  // we have filled in setup().
  redisClient
    .srandmember("crocodile_ids")
    .then((randomID) => {
      const url = `https://test-api.k6.io/public/crocodiles/${randomID}`;
      const res = http.get(url);

      check(res, {
        "status is 200": (r) => r.status === 200,
        "content-type is application/json": (r) =>
          r.headers["content-type"] === "application/json",
      });

      return url;
    })
    .then((url) => redisClient.hincrby("k6_crocodile_fetched", url, 1));
}

export function teardown() {
  redisClient.del("crocodile_ids");
}

export function handleSummary(data) {
  redisClient
    .hgetall("k6_crocodile_fetched")
    .then((fetched) => Object.assign(data, { k6_crocodile_fetched: fetched }))
    .then((data) =>
      redisClient.set(`k6_report_${Date.now()}`, JSON.stringify(data))
    )
    .then(() => redisClient.del("k6_crocodile_fetched"));

  return {
    stdout: textSummary(data, { indent: "  ", enableColors: true }),
  };
}
