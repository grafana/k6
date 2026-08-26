import { check } from "k6";

export const options = {
  thresholds: {
    checks: ["rate==1"],
  },
};

export default function () {
  check(__ENV, {
    'MQTT_BROKER_ADDRESS': (env) => env.MQTT_BROKER_ADDRESS != undefined,
  });
}
