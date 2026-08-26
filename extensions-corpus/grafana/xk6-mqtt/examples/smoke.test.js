import { check } from "k6";
import mqtt from "k6/x/mqtt";

export const options = {
  thresholds: {
    checks: ["rate==1"],
  },
}

export default function () {
  check(mqtt, {
    'Client': (mod) => typeof mod.Client === 'function',
  });
}
