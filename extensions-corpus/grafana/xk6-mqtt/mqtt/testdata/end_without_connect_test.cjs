const mqtt = require("k6/x/mqtt")
const assert = require("k6/x/assert")

var endHandlerCalled = false

module.exports = () => {
  const client = new mqtt.Client()

  client.on("end", () => {
    endHandlerCalled = true
  })

  assert.false(client.connected, "Client should not be connected initially")

  client.end()

  assert.false(client.connected, "Client should not be connected after end call")
}

module.exports.teardown = () => {
  assert.true(endHandlerCalled, "End handler was not called")
}
