/*

This is a k6 test script that imports the xk6-kafka and


*/

import { check, sleep } from "k6";
// import * as kafka from "k6/x/kafka";
import { Reader, Connection } from "k6/x/kafka"; // import kafka extension

// Prints module-level constants
// console.log(kafka);

const brokers = ["localhost:9092"];
const topic = "xk6_kafka_json_topic";

const reader = new Reader({
  brokers: brokers,
  topic: topic,
  maxWait: "5s",
});

export function setup() {
  const connection = new Connection({
    address: brokers[0],
  });

  connection.createTopic({ topic: topic });

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
  duration: "11s",
};

export default function () {
  // Read 10 messages only
  let messages = reader.consume({ limit: 10 });

  console.log("continuing execution");

  check(messages, {
    "10 messages are received": (messages) => messages.length === 10,
  });
}

export function teardown(data) {
  const connection = new Connection({
    address: brokers[0],
  });
  connection.deleteTopic(topic);
  connection.close();
  reader.close();
}
