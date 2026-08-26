const mqtt = require("k6/x/mqtt")
const assert = require("k6/x/assert")

const testTopic = "test/topic"
var handlerCall = 0

module.exports = () => {
  const client = new mqtt.Client()

  client.on("connect", () => {
    client.subscribe(testTopic)

    client.publishAsync(testTopic, "Hello, Async MQTT!").then(() => {
      client.publish(testTopic, "Hello, MQTT!")
    })
  })

  client.on("message", (topic, message) => {
    assert.equal(testTopic, topic, "Unexpected topic")

    const str = String.fromCharCode.apply(null, new Uint8Array(message));

    switch (handlerCall) {
      case 0:
        assert.equal("Hello, Async MQTT!", str, "Unexpected message");
        break;
      case 1:
        assert.equal("Hello, MQTT!", str, "Unexpected message");
        break;
    }

    if (++handlerCall === 2) {
      client.end()
    }

  })

  client.connect(__ENV.MQTT_BROKER_ADDRESS)
}

module.exports.teardown = () => {
  assert.equal(handlerCall, 2, "Message handler was not called the expected number of times")
}
