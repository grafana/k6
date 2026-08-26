import { check } from "k6";
import icmp from "k6/x/icmp";

export const options = {
  thresholds: {
    checks: ["rate==1"],
  },
}

export default function () {
  check(icmp, {
    'ping': (mod) => typeof mod.ping === 'function',
    'pingAsync': (mod) => typeof mod.pingAsync === 'function',
  });
}
