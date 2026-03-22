// Single scenario with exec, env, tags. No default export.
export const options = {
  scenarios: {
    api: {
      executor: "per-vu-iterations",
      vus: 2,
      iterations: 2,
      exec: "api",
      env: { BASE_URL: "http://test.k6.io" },
      tags: { type: "api" },
    },
  },
};

export function api() {
  console.log("api ran, BASE_URL=" + __ENV.BASE_URL);
}
