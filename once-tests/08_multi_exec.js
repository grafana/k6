// Two scenarios, both with exec, env, tags. No default export.
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
    ui: {
      executor: "constant-vus",
      vus: 2,
      duration: "2s",
      exec: "ui",
      env: { HEADLESS: "true" },
      tags: { type: "ui" },
      options: { browser: { type: "chromium" } },
    },
  },
};

export function api() {
  console.log("api ran, BASE_URL=" + __ENV.BASE_URL);
}

export function ui() {
  console.log("ui ran, HEADLESS=" + __ENV.HEADLESS);
}
