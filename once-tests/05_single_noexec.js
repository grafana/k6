// Single scenario without exec (targets default). Has env, tags, browser options.
export const options = {
  scenarios: {
    ui: {
      executor: "constant-vus",
      vus: 2,
      duration: "2s",
      env: { BASE_URL: "http://test.k6.io" },
      tags: { type: "ui" },
      options: { browser: { type: "chromium" } },
    },
  },
};

export default function () {
  console.log("default ran, BASE_URL=" + __ENV.BASE_URL);
}
