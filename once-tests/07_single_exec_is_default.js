// Single scenario with explicit exec: 'default'. Has env, tags.
export const options = {
  scenarios: {
    smoke: {
      executor: "constant-vus",
      vus: 2,
      duration: "2s",
      exec: "default",
      env: { SMOKE: "true" },
      tags: { type: "smoke" },
    },
  },
};

export default function () {
  console.log("default ran via explicit exec, SMOKE=" + __ENV.SMOKE);
}
