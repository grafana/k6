import { check } from "k6"
import { Counter } from 'k6/metrics'
import { Client } from "k6/x/mqtt"

export const options = {
  thresholds: {
    checks: ["rate==1"],
    connect_handler_called: ["count==1"],
    end_handler_called: ["count==1"],
    mqtt_calls: ["count==2"],
  },
}

const connectHandlerCalled = new Counter("connect_handler_called")
const endHandlerCalled = new Counter("end_handler_called")

export default function () {
  const client = new Client()

  client.on("connect", () => {
    check(client.connected, {
      'Client connected after connect event': (connected) => connected,
    })

    connectHandlerCalled.add(1)

    client.end()
  })

  client.on("end", () => {
    check(client.connected, {
      'Client not connected after end event': (connected) => !connected,
    })

    endHandlerCalled.add(1)
  })

  check(client.connected, {
    'Client not connected initially': (connected) => !connected,
  })

  client.connect(__ENV.MQTT_BROKER_ADDRESS)

  check(client.connected, {
    'Client connected after connect call': (connected) => connected,
  })
}
