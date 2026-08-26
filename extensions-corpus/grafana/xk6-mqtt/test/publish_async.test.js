import { check } from "k6"
import { Counter } from 'k6/metrics'
import { Client } from "k6/x/mqtt"

export const options = {
  thresholds: {
    checks: ["rate==1"],
    handler_called: ["count==2"],
    mqtt_calls: ["count==7"],
    mqtt_errors: ["count==0"],
    mqtt_messages_sent: ["count==2"],
    mqtt_messages_received: ["count==2"],
    data_received: ["count==30"],
    data_sent: ["count==30"], // "Hello, Async MQTT!" + "Hello, MQTT!"
  },
}

const handlerCalled = new Counter("handler_called")

const testTopic = "test/topic"

export default function () {
  const client = new Client()
  var handlerCall = 0

  client.on("connect", () => {
    client.subscribe(testTopic)

    client.publishAsync(testTopic, "Hello, Async MQTT!").then(() => {
      client.publish(testTopic, "Hello, MQTT!")
    })
  })

  client.on("message", (topic, message) => {
    const str = String.fromCharCode.apply(null, new Uint8Array(message));

    check(topic, {
      'Topic match': (t) => t == testTopic,
    })

    var expectedMessage = "Hello, Async MQTT!";

    if (handlerCall == 1) {
      expectedMessage = "Hello, MQTT!";
    }

    check(str, {
      'Message match': (m) => m == expectedMessage,
    })

    handlerCalled.add(1)

    if (++handlerCall === 2) {
      client.end()
    }
  })

  client.connect(__ENV.MQTT_BROKER_ADDRESS)
}
