const mqtt = require("k6/x/mqtt")
const assert = require("k6/x/assert")

var connectHandlerCalled = false
var endHandlerCalled = false

module.exports = () => {
  const client = new mqtt.Client()

  client.on("connect", async () => {
    assert.true(client.connected, "Client should be connected after connect event")

    connectHandlerCalled = true

    await client.endAsync()
  })

  client.on("end", () => {
    endHandlerCalled = true
  })

  assert.false(client.connected, "Client should not be connected initially")

  client.connect(__ENV.MQTT_BROKER_ADDRESS)

  assert.true(client.connected, "Client should be connected after connect call")
}

module.exports.teardown = () => {
  assert.true(connectHandlerCalled, "Connect handler was not called")
  assert.true(endHandlerCalled, "End handler was not called")
}
