/*

This is a k6 test script that imports the xk6-kafka and
tests Kafka with a 200 JSON messages per iteration.

*/

import { check, sleep } from "k6";
// import * as kafka from "k6/x/kafka";
import {
  Connection,
  Reader,
  SCHEMA_TYPE_STRING,
  SchemaRegistry,
  Writer,
} from "k6/x/kafka"; // import kafka extension

// Prints module-level constants
// console.log(kafka);

const brokers = ["localhost:9092"];
const topic = "xk6_kafka_custom_writer_balancer_topic";

const writer = new Writer({
  brokers: brokers,
  topic: topic,
  balancer: function (bytes, partitionCount) {
    return 7;
  },
});
const reader = new Reader({
  brokers: brokers,
  topic: topic,
  partition: 7,
});
const schemaRegistry = new SchemaRegistry();

export const options = {
  thresholds: {
    // Base thresholds to see if the writer or reader is working
    kafka_writer_error_count: ["count == 0"],
    kafka_reader_error_count: ["count == 0"],
  },
};

export function setup() {
  // Create connection inside setup() to ensure proper VU context
  const connection = new Connection({
    address: brokers[0],
  });
  connection.createTopic({ topic: topic, numPartitions: 10 });

  // Verify topic exists and has the correct number of partitions
  // This ensures metadata has propagated before VUs start
  const topics = connection.listTopics();
  if (!topics.includes(topic)) {
    throw new Error(`Topic ${topic} was not created successfully`);
  }

  connection.close();

  // Wait for Kafka metadata to propagate to all brokers
  // This ensures Writer/Reader can see all partitions
  sleep(2);
}

export default function () {
  for (let index = 0; index < 100; index++) {
    let messages = [
      {
        key: schemaRegistry.serialize({
          data: "test-key-string",
          schemaType: SCHEMA_TYPE_STRING,
        }),
        value: schemaRegistry.serialize({
          data: "test-value-string",
          schemaType: SCHEMA_TYPE_STRING,
        }),
      },
    ];

    writer.produce({ messages: messages });
  }

  // Read 10 messages only
  let messages = reader.consume({ limit: 10 });

  check(messages, {
    "10 messages are received": (messages) => messages.length == 10,
    "messages are all in partition 7": (messages) =>
      messages.every((message) => message.partition == 7),
  });
}

export function teardown(data) {
  // Create connection inside teardown() to ensure proper VU context
  const connection = new Connection({
    address: brokers[0],
  });
  connection.deleteTopic(topic);
  connection.close();
  writer.close();
  reader.close();
}
