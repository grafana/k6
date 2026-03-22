// Three scenarios: api has exec, ui has exec+browser, smoke has no exec.
// All have env and tags. Default is exported.
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

export function ui() {
  console.log("ui ran, HEADLESS=" + __ENV.HEADLESS);
}

export default function () {
  console.log("default ran, SMOKE=" + __ENV.SMOKE);
}
