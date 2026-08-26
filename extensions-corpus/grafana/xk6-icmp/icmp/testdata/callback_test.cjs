const icmp = require("k6/x/icmp")
const assert = require("k6/x/assert")

var callbackCalled = false

module.exports = async () => {
  const result = await icmp.pingAsync("127.0.0.1", (err, { alive }) => {
    assert.false(err, "Error should be null")
    assert.true(alive, "Loopback host should be alive")

    callbackCalled = true
  })

  assert.true(result, "Loopback host should be alive (from promise)")
}

module.exports.teardown = () => {
  assert.true(callbackCalled, "Callback was not called")
}
