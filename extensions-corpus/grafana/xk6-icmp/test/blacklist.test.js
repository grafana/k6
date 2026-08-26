import { check } from "k6"
import { ping } from "k6/x/icmp"

export const options = {
  thresholds: {
    checks: ["rate==1"],
    icmp_errors: ["count==1"],           // 1 because of blacklist
    icmp_packets_sent: ["count==0"],     // 0 becasue of blacklist
    icmp_packets_received: ["count==0"], // 0 becasue of blacklist
    icmp_reply_ttl: ["value==0"],        // 0 becasue of blacklist
    icmp_rtt: ["avg<0.1"],
    icmp_setup: ["avg<0.1"],
    icmp_resolve: ["avg<0.1"],
    data_received: ["count==0"],         // 0 becasue of blacklist
    data_sent: ["count==0"],             // 0 becasue of blacklist
  },
  blacklistIPs: ['127.0.0.1/24'],
}

export default function () {
  const result = ping("127.0.0.1")

  check(result, {
    'Loopback address is reachable': (alive) => alive,
  })
}
