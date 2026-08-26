import { Client } from "k6/x/mqtt";

export default function () {
  const client = new Client()

  client.on("connect", () => {
    console.log("Connected to MQTT broker")
    client.subscribe("greeting")
    client.publish("greeting", "Hello MQTT!")
  })

  client.on("message", (topic, message) => {
    const str = String.fromCharCode.apply(null, new Uint8Array(message))
    console.info("topic:", topic, "message:", str)
    client.end()
  })

  client.on("end", () => {
    console.log("Disconnected from MQTT broker")
  })

  client.connect(__ENV["MQTT_BROKER_ADDRESS"] || "mqtt://broker.emqx.io:1883")
}
