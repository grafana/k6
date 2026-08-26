import { check } from "k6";
import {
  AdminClient,
  Consumer,
  Producer,
  SCHEMA_TYPE_JSON,
  SchemaRegistry,
} from "k6/x/kafka";

import { brokers, createTopic, deleteTopic, topicName } from "../common.js";

const topic = topicName("integration-consumer-group");
const groupId = `${topic}-group`;

const adminClient = new AdminClient({ brokers });
const producer = new Producer({ brokers, topic });
const schemaRegistry = new SchemaRegistry();

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
    numPartitions: 3,
    replicationFactor: 1,
  });
}

export default function () {
  const consumer = new Consumer({
    brokers,
    groupId,
    groupTopics: [topic],
  });
  const messages = [];
  for (let index = 0; index < 6; index++) {
    messages.push({
      key: schemaRegistry.serialize({
        data: { correlationId: `group-key-${index}` },
        schemaType: SCHEMA_TYPE_JSON,
      }),
      value: schemaRegistry.serialize({
        data: { kind: "consumer-group", index },
        schemaType: SCHEMA_TYPE_JSON,
      }),
    });
  }

  try {
    producer.produce({ messages });

    const consumed = consumer.consume({ maxMessages: 6 });

    check(consumed, {
      "consumer group receives six messages": (received) =>
        received.length === 6,
      "consumer group topic matches": (received) =>
        received.every((message) => message.topic === topic),
      "consumer group payloads deserialize": (received) =>
        received.length === 6 &&
        received.every((message) => {
          const key = schemaRegistry.deserialize({
            data: message.key,
            schemaType: SCHEMA_TYPE_JSON,
          });
          const value = schemaRegistry.deserialize({
            data: message.value,
            schemaType: SCHEMA_TYPE_JSON,
          });
          return (
            key.correlationId.startsWith("group-key-") &&
            value.kind === "consumer-group"
          );
        }),
    });
  } finally {
    consumer.close();
  }
}

export function teardown() {
  producer.close();
  deleteTopic(adminClient, topic);
  adminClient.close();
}
