import { check } from "k6"
import { Counter } from 'k6/metrics'
import { Client } from "k6/x/mqtt"

export const options = {
  thresholds: {
    checks: ["rate==1"],
    handler_called: ["count==1"],
    mqtt_calls: ["count==5"],
    mqtt_errors: ["count==0"],
    mqtt_messages_sent: ["count==1"],
    mqtt_messages_received: ["count==1"],
    data_received: ["count==12"],
    data_sent: ["count==12"],
  },
}

const handlerCalled = new Counter("handler_called")

const testTopic = "test/topic"

export default function () {
  const client = new Client()

  client.on("connect", () => {
    client.subscribe(testTopic)

    client.publish(testTopic, "Hello, MQTT!")
  })

  client.on("message", (topic, message) => {
    const str = String.fromCharCode.apply(null, new Uint8Array(message));

    check(topic, {
      'Topic match': (t) => t == testTopic,
    })

    check(str, {
      'Message match': (m) => m == "Hello, MQTT!",
    })

    handlerCalled.add(1)

    client.end()
  })

  client.connect(__ENV.MQTT_BROKER_ADDRESS)
}
