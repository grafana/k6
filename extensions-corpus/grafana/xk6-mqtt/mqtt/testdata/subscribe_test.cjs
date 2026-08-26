const mqtt = require("k6/x/mqtt")
const assert = require("k6/x/assert")

const testTopic = "test/topic"
var handlerCalled = false

module.exports = () => {
  const client = new mqtt.Client()

  client.on("connect", () => {
    client.subscribe(testTopic)

    client.publish(testTopic, "Hello, MQTT!")
  })

  client.on("message", (topic, message) => {
    const str = String.fromCharCode.apply(null, new Uint8Array(message));
    assert.equal(testTopic, topic, "Unexpected topic")
    assert.equal("Hello, MQTT!", str, "Unexpected message")

    handlerCalled = true
    client.end()
  })

  client.connect(__ENV.MQTT_BROKER_ADDRESS)
}

module.exports.teardown = () => {
  assert.true(handlerCalled, "Message handler was not called")
}
