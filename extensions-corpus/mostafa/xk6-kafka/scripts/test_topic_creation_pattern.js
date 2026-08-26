/*

This is an example k6 test script that demonstrates the CORRECT pattern
for topic creation to avoid race conditions with multiple VUs.

*/

import { check, sleep } from "k6";
import {
  Connection,
  Reader,
  SCHEMA_TYPE_STRING,
  SchemaRegistry,
  Writer,
} from "k6/x/kafka";

const brokers = ["localhost:9092"];
const topic = "xk6_kafka_example_topic";

// Create Writer and Reader objects at module level (init context)
const writer = new Writer({
  brokers: brokers,
  topic: topic,
  // Option 1: Let Writer auto-create topic (uses broker defaults)
  // autoCreateTopic: true,
});

const reader = new Reader({
  brokers: brokers,
  topic: topic,
});

const schemaRegistry = new SchemaRegistry();

export const options = {
  thresholds: {
    kafka_writer_error_count: ["count == 0"],
    kafka_reader_error_count: ["count == 0"],
  },
};

// Create topic in setup() function
// This runs once before any VU iterations begin, avoiding race conditions
export function setup() {
  // IMPORTANT: Create Connection inside setup(), not at module level
  const connection = new Connection({
    address: brokers[0],
  });

  // Create topic with specific configuration
  connection.createTopic({
    topic: topic,
    numPartitions: 3,
    replicationFactor: 1,
  });

  // Verify topic was created successfully
  const topics = connection.listTopics();
  if (!topics.includes(topic)) {
    throw new Error(`Topic ${topic} was not created successfully`);
  }

  connection.close();

  // Wait for Kafka metadata to propagate to all brokers
  // This ensures Writer/Reader can see all partitions
  sleep(2);
}

// This is the main VU function that runs for each iteration
export default function () {
  // Produce messages
  let messages = [
    {
      key: schemaRegistry.serialize({
        data: "test-key",
        schemaType: SCHEMA_TYPE_STRING,
      }),
      value: schemaRegistry.serialize({
        data: "test-value",
        schemaType: SCHEMA_TYPE_STRING,
      }),
    },
  ];

  writer.produce({ messages: messages });

  // Consume messages
  let consumedMessages = reader.consume({ limit: 1 });

  check(consumedMessages, {
    "1 message received": (msgs) => msgs.length == 1,
  });
}

// Cleanup after all VU iterations complete
export function teardown(data) {
  // IMPORTANT: Create Connection inside teardown(), not at module level
  const connection = new Connection({
    address: brokers[0],
  });

  connection.deleteTopic(topic);
  connection.close();

  writer.close();
  reader.close();
}
