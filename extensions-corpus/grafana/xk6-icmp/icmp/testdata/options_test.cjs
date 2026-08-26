const icmp = require("k6/x/icmp")
const assert = require("k6/x/assert")

var callbackCount = 0

module.exports = async () => {
  const opts = {
    id: 42,
    seq: 1,
    size: 80,
    ttl: 72,
    interval: "200ms",
    timeout: "300ms",
    deadline: "2s",
    count: 3,
    threshold: 95,
    preferred_ip_version: "ip4",
  }

  var lastSentAt = 0

  await icmp.pingAsync("127.0.0.1", opts, (err, { sent_at, options }) => {
    assert.false(err, "Error should be null")
    assert.equal(options.id, opts.id, "'id' options should match the ones passed")
    assert.equal(options.seq, opts.seq, "'seq' options should match the ones passed")
    assert.equal(options.size, opts.size, "'size' options should match the ones passed")
    assert.equal(options.ttl, opts.ttl, "'ttl' options should match the ones passed")
    assert.equal(options.interval, opts.interval, "'interval' options should match the ones passed")
    assert.equal(options.timeout, opts.timeout, "'timeout' options should match the ones passed")
    assert.equal(options.deadline, opts.deadline, "'deadline' options should match the ones passed")
    assert.equal(options.count, opts.count, "'count' options should match the ones passed")
    assert.equal(options.threshold, opts.threshold, "'threshold' options should match the ones passed")
    assert.equal(options.preferred_ip_version, opts.preferred_ip_version, "'preferred_ip_version' options should match the ones passed")

    callbackCount++

    if (lastSentAt) {
      const interval = sent_at - lastSentAt

      assert.true(interval >= 150, `Interval between pings should be at least ${options.interval}, got ${interval}`)
      assert.true(interval <= 250, `Interval between pings should be at most ${options.interval}, got ${interval}`)
    }

    lastSentAt = sent_at
  })

}

module.exports.teardown = () => {
  assert.equal(callbackCount, 3, "Callback was not called expected number of times")
}
