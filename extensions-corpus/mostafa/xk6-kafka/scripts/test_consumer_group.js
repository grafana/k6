/*

This is a k6 test script that imports the xk6-kafka and
tests Kafka with a 200 JSON messages per iteration.

*/

import { check, sleep } from "k6";
// import * as kafka from "k6/x/kafka";
import {
  Writer,
  Reader,
  Connection,
  SchemaRegistry,
  CODEC_SNAPPY,
  SCHEMA_TYPE_JSON,
} from "k6/x/kafka"; // import kafka extension

// Prints module-level constants
// console.log(kafka);

const brokers = ["localhost:9092"];
const topic = "xk6_kafka_consumer_group_topic";
const groupID = "my-group";

const writer = new Writer({
  brokers: brokers,
  topic: topic,
  compression: CODEC_SNAPPY,
});
const reader = new Reader({
  brokers: brokers,
  groupID: groupID,
  groupTopics: [topic],
});
const schemaRegistry = new SchemaRegistry();

export function setup() {
  const connection = new Connection({
    address: brokers[0],
  });

  connection.createTopic({
    topic: topic,
    numPartitions: 3,
    replicationFactor: 1,
    configEntries: [
      {
        configName: "compression.type",
        configValue: CODEC_SNAPPY,
      },
    ],
  });

  // Verify topic was created
  const topics = connection.listTopics();
  if (!topics.includes(topic)) {
    throw new Error(`Topic ${topic} was not created successfully`);
  }

  connection.close();

  // Wait for Kafka metadata to propagate to all brokers
  sleep(2);
}

export const options = {
  thresholds: {
    // Base thresholds to see if the writer or reader is working
    kafka_writer_error_count: ["count == 0"],
    kafka_reader_error_count: ["count == 0"],
  },
};

export default function () {
  let messages = [];
  for (let i = 0; i < 100; i++) {
    for (let partition = 0; partition < 3; partition++) {
      messages.push({
        // The data type of the key is JSON
        key: schemaRegistry.serialize({
          data: {
            key: "value",
          },
          schemaType: SCHEMA_TYPE_JSON,
        }),
        // The data type of the value is JSON
        value: schemaRegistry.serialize({
          data: {
            key: "value",
          },
          schemaType: SCHEMA_TYPE_JSON,
        }),
        parition: partition,
      });
    }
  }

  writer.produce({ messages: messages });

  // Read one message only
  messages = reader.consume({ limit: 10 });

  check(messages, {
    "10 messages is received": (messages) => messages.length == 10,
  });

  check(messages[0], {
    "Topic equals to xk6_kafka_consumer_group_topic": (msg) =>
      msg["topic"] == topic,
    "Key contains key/value and is JSON": (msg) =>
      schemaRegistry.deserialize({
        data: msg.key,
        schemaType: SCHEMA_TYPE_JSON,
      }).key == "value",
    "Value contains key/value and is JSON": (msg) =>
      typeof schemaRegistry.deserialize({
        data: msg.value,
        schemaType: SCHEMA_TYPE_JSON,
      }) == "object" &&
      schemaRegistry.deserialize({
        data: msg.value,
        schemaType: SCHEMA_TYPE_JSON,
      }).key == "value",
  });
}

export function teardown(data) {
  const connection = new Connection({
    address: brokers[0],
  });
  connection.deleteTopic(topic);
  connection.close();
  writer.close();
  reader.close();
}
