import { check } from "k6";
import { AdminClient, Consumer, Producer } from "k6/x/kafka";

import {
  brokers,
  createTopic,
  decodeBytes,
  deleteTopic,
  topicName,
} from "../common.js";

const topic = topicName("smoke-basic");

const adminClient = new AdminClient({ brokers });
const producer = new Producer({ brokers, topic });
const consumer = new Consumer({ brokers, topic });

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    kafka_writer_error_count: ["count == 0"],
    kafka_reader_error_count: ["count == 0"],
  },
};

export function setup() {
  createTopic(adminClient, topic, {
    numPartitions: 1,
    replicationFactor: 1,
  });
}

export default function () {
  producer.produce({
    messages: [
      {
        key: JSON.stringify({ correlationId: "smoke-key-1" }),
        value: JSON.stringify({ name: "xk6-kafka", phase: "v2", index: 1 }),
        headers: { source: "scripts/v2/smoke/basic" },
      },
      {
        key: JSON.stringify({ correlationId: "smoke-key-2" }),
        value: JSON.stringify({ name: "xk6-kafka", phase: "v2", index: 2 }),
      },
    ],
  });

  const messages = consumer.consume({ maxMessages: 2 });

  check(messages, {
    "received two messages": (received) => received.length === 2,
    "topic matches v2 smoke topic": (received) =>
      received.every((message) => message.topic === topic),
    "JSON payloads round-trip": (received) =>
      received.length === 2 &&
      received.every((message) => {
        const key = JSON.parse(String.fromCharCode(...message.key));
        const value = JSON.parse(String.fromCharCode(...message.value));
        return (
          key.correlationId.startsWith("smoke-key-") &&
          value.name === "xk6-kafka" &&
          value.phase === "v2"
        );
      }),
    "header round-trips": (received) =>
      received.length > 0 &&
      decodeBytes(received[0].headers.source) === "scripts/v2/smoke/basic",
  });
}

export function teardown() {
  deleteTopic(adminClient, topic);
  producer.close();
  consumer.close();
  adminClient.close();
}
