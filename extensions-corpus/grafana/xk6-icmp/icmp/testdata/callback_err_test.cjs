const icmp = require("k6/x/icmp")
const assert = require("k6/x/assert")

var callbackCalled = false

module.exports = async () => {
  const result = await icmp.pingAsync("192.0.2.1", { timeout: "500ms" }, (err, { alive }) => {
    assert.true(err, "Error should not be null")
    assert.false(alive, "TEST-NET-1 IP should be unreachable")

    callbackCalled = true
  })

  assert.false(result, "TEST-NET-1 IP should be unreachable (from promise)")
}

module.exports.teardown = () => {
  assert.true(callbackCalled, "Callback was not called")
}
