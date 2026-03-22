// Single scenario with exec, env, tags, and thresholds.
export const options = {
  scenarios: {
    api: {
      executor: "shared-iterations",
      vus: 2,
      iterations: 2,
      exec: "api",
      env: { BASE_URL: "http://test.k6.io" },
      tags: { type: "api" },
    },
  },
  thresholds: {
    iteration_duration: ["avg<5000"],
  },
};

export function api() {
  console.log("api ran, BASE_URL=" + __ENV.BASE_URL);
}
