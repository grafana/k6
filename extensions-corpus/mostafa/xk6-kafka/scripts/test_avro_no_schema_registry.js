/*

This is a k6 test script that imports the xk6-kafka and
tests Kafka by sending 200 Avro messages per iteration
without any associated key.
*/

import { check, sleep } from "k6";
import {
  Writer,
  Reader,
  Connection,
  SchemaRegistry,
  SCHEMA_TYPE_AVRO,
} from "k6/x/kafka"; // import kafka extension

const brokers = ["localhost:9092"];
const topic = "xk6_kafka_avro_topic";

const writer = new Writer({
  brokers: brokers,
  topic: topic,
});
const reader = new Reader({
  brokers: brokers,
  topic: topic,
});
const schemaRegistry = new SchemaRegistry();

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

const valueSchema = JSON.stringify({
  type: "record",
  name: "Value",
  namespace: "dev.mostafa.xk6.kafka",
  fields: [
    {
      name: "name",
      type: "string",
    },
  ],
});

export default function () {
  for (let index = 0; index < 100; index++) {
    let messages = [
      {
        value: schemaRegistry.serialize({
          data: {
            name: "xk6-kafka",
          },
          schema: { schema: valueSchema },
          schemaType: SCHEMA_TYPE_AVRO,
        }),
      },
    ];
    writer.produce({ messages: messages });
  }

  // Read 10 messages only
  let messages = reader.consume({ limit: 10 });
  check(messages, {
    "10 messages returned": (msgs) => msgs.length == 10,
    "value is correct": (msgs) =>
      schemaRegistry.deserialize({
        data: msgs[0].value,
        schema: { schema: valueSchema },
        schemaType: SCHEMA_TYPE_AVRO,
      }).name == "xk6-kafka",
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
