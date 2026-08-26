import { check } from "k6"
import { Counter } from 'k6/metrics'
import { pingAsync } from "k6/x/icmp"

export const options = {
  thresholds: {
    checks: ["rate==1"],
    callback_called: ["count==3"],
    icmp_errors: ["count==0"],
    icmp_packets_sent: ["count==3"],
    icmp_packets_received: ["count==3"],
    icmp_reply_ttl: ["value==64"],
    icmp_rtt: ["avg<0.1"],
    icmp_setup: ["avg<0.1"],
    icmp_resolve: ["avg<0.1"],
    data_received: ["count==264"],
    data_sent: ["count==264"],
  },
}

const callbackCalled = new Counter("callback_called")

export default async function () {
  const opts = {
    id: 42,
    seq: 1,
    size: 80,
    ttl: 72,
    interval: "500ms",
    timeout: "1s",
    deadline: "4s",
    count: 3,
    threshold: 95,
    preferred_ip_version: "ip4",
  }

  var lastSentAt = 0

  const result = await pingAsync("127.0.0.1", opts, (err, { alive, sent_at, options }) => {
    check(alive, {
      'Loopback address is reachable': (alive) => alive,
    })

    check(err, {
      'No error': (err) => !err,
    })

    check(options, {
      "'id' options should match the ones passed": (o) => o.id === opts.id,
      "'seq' options should match the ones passed": (o) => o.seq === opts.seq,
      "'size' options should match the ones passed": (o) => o.size === opts.size,
      "'ttl' options should match the ones passed": (o) => o.ttl === opts.ttl,
      "'interval' options should match the ones passed": (o) => o.interval === opts.interval,
      "'timeout' options should match the ones passed": (o) => o.timeout === opts.timeout,
      "'deadline' options should match the ones passed": (o) => o.deadline === opts.deadline,
      "'count' options should match the ones passed": (o) => o.count === opts.count,
      "'threshold' options should match the ones passed": (o) => o.threshold === opts.threshold,
      "'preferred_ip_version' options should match the ones passed": (o) => o.preferred_ip_version === opts.preferred_ip_version,
    })

    if (err) {
      console.error(err)
    }

    callbackCalled.add(1)

    if (lastSentAt) {
      const interval = sent_at - lastSentAt

      check(interval, {
        [`Interval between pings should be at least ${options.interval}`]: (v) => v >= 400,
        [`Interval between pings should be at most ${options.interval}`]: (v) => v <= 600,
      })
    }

    lastSentAt = sent_at
  })

  check(result, {
    'Loopback address is reachable (from promise)': (alive) => alive,
  })
}
