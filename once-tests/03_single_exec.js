// Single scenario with exec, env, tags. Default also exported.
export const options = {
  scenarios: {
    api: {
      executor: "constant-vus",
      vus: 2,
      duration: "2s",
      exec: "api",
      env: { BASE_URL: "http://test.k6.io" },
      tags: { type: "api" },
    },
  },
};

export function api() {
  console.log("api ran, BASE_URL=" + __ENV.BASE_URL);
}

export default function () {
  console.log("default ran (should not happen)");
}
