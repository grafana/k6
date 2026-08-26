import { check } from "k6"
import { Counter } from 'k6/metrics'
import { pingAsync } from "k6/x/icmp"

export const options = {
  thresholds: {
    checks: ["rate==1"],
    callback_called: ["count==1"],
    icmp_errors: ["count==0"],
    icmp_packets_sent: ["count==1"],
    icmp_packets_received: ["count==1"],
    icmp_reply_ttl: ["value==64"],
    icmp_rtt: ["avg<0.1"],
    icmp_setup: ["avg<0.1"],
    icmp_resolve: ["avg<0.1"],
    data_received: ["count==64"],
    data_sent: ["count==64"],
  },
}

const callbackCalled = new Counter("callback_called")

export default async function () {
  const result = await pingAsync("127.0.0.1", (error, details) => {
    check(details.alive, {
      'Loopback address is reachable': (alive) => alive,
    })

    callbackCalled.add(1)
  })

  check(result, {
    'Loopback address is reachable (from promise)': (alive) => alive,
  })
}
