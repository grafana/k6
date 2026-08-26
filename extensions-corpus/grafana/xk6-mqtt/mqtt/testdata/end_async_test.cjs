const mqtt = require("k6/x/mqtt")
const assert = require("k6/x/assert")

var endHandlerCalled = false

module.exports = () => {
  const client = new mqtt.Client()

  client.on("connect", () => {
    assert.true(client.connected, "Client should be connected after connect event")

    client.endAsync().then(() => {
      assert.false(client.connected, "Client should not be connected after endAsync call finishes")
    })
  })

  client.on("end", () => {
    endHandlerCalled = true
  })

  assert.false(client.connected, "Client should not be connected initially")

  client.connect(__ENV.MQTT_BROKER_ADDRESS)

  assert.true(client.connected, "Client should be connected after connect call")
}

module.exports.teardown = () => {
  assert.true(endHandlerCalled, "End handler was not called")
}
