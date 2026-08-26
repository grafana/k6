/*

This is a k6 test script that imports the xk6-kafka and
list topics on all Kafka partitions and creates a topic.

*/

import { sleep } from "k6";
import { Connection } from "k6/x/kafka"; // import kafka extension

const address = "localhost:9092";
const topic = "xk6_kafka_test_topic";

let topicList = [];

export function setup() {
  const connection = new Connection({
    address: address,
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

  // Return topic list for use in default function
  return { topics: topics };
}

export default function (data) {
  data.topics.forEach((topic) => console.log(topic));
}

export function teardown(data) {
  const connection = new Connection({
    address: address,
  });
  connection.deleteTopic(topic);
  connection.close();
}
