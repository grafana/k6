// Two scenarios, one with exec, one without. No default export.
// All have env and tags. The scenario without exec targets default, but default doesn't exist.
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
    smoke: {
      executor: "constant-vus",
      vus: 1,
      duration: "2s",
      env: { SMOKE: "true" },
      tags: { type: "smoke" },
    },
  },
};

export function api() {
  console.log("api ran, BASE_URL=" + __ENV.BASE_URL);
}
