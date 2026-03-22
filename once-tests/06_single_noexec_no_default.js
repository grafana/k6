// Single scenario without exec, no default export.
// The scenario targets default but default doesn't exist.
export const options = {
  scenarios: {
    ui: {
      executor: "constant-vus",
      vus: 2,
      duration: "2s",
      env: { BASE_URL: "http://test.k6.io" },
      tags: { type: "ui" },
    },
  },
};

export function ui() {
  console.log("ui ran (should not happen)");
}
