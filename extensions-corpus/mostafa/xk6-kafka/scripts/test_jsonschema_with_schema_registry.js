/*
This is a k6 test script that imports the xk6-kafka and
tests Kafka with a 100 Avro messages per iteration.
*/

import { check, sleep } from "k6";
import {
  Writer,
  Reader,
  Connection,
  SchemaRegistry,
  KEY,
  VALUE,
  SCHEMA_TYPE_JSON,
  TOPIC_NAME_STRATEGY,
} from "k6/x/kafka"; // import kafka extension

const brokers = ["localhost:9092"];
const topic = "xk6_jsonschema_test";

const writer = new Writer({
  brokers: brokers,
  topic: topic,
});
const reader = new Reader({
  brokers: brokers,
  topic: topic,
});
const schemaRegistry = new SchemaRegistry({
  url: "http://localhost:8081",
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

const keySchema = JSON.stringify({
  title: "Key",
  type: "object",
  properties: {
    key: {
      type: "string",
      description: "A key.",
    },
  },
});

const valueSchema = JSON.stringify({
  title: "Value",
  type: "object",
  properties: {
    firstName: {
      type: "string",
      description: "First name.",
    },
    lastName: {
      type: "string",
      description: "Last name.",
    },
  },
});

const keySubjectName = schemaRegistry.getSubjectName({
  topic: topic,
  element: KEY,
  subjectNameStrategy: TOPIC_NAME_STRATEGY,
  schema: keySchema,
});

const valueSubjectName = schemaRegistry.getSubjectName({
  topic: topic,
  element: VALUE,
  subjectNameStrategy: TOPIC_NAME_STRATEGY,
  schema: valueSchema,
});

const keySchemaObject = schemaRegistry.createSchema({
  subject: keySubjectName,
  schema: keySchema,
  schemaType: SCHEMA_TYPE_JSON,
});

const valueSchemaObject = schemaRegistry.createSchema({
  subject: valueSubjectName,
  schema: valueSchema,
  schemaType: SCHEMA_TYPE_JSON,
});

export default function () {
  for (let index = 0; index < 100; index++) {
    let messages = [
      {
        key: schemaRegistry.serialize({
          data: {
            key: "key-" + index,
          },
          schema: keySchemaObject,
          schemaType: SCHEMA_TYPE_JSON,
        }),
        value: schemaRegistry.serialize({
          data: {
            firstName: "firstName-" + index,
            lastName: "lastName-" + index,
          },
          schema: valueSchemaObject,
          schemaType: SCHEMA_TYPE_JSON,
        }),
      },
    ];
    writer.produce({ messages: messages });
  }

  let messages = reader.consume({ limit: 20 });
  check(messages, {
    "20 message returned": (msgs) => msgs.length == 20,
  });

  check(messages, {
    "20 message returned": (msgs) => msgs.length == 20,
    "key starts with 'key-' string": (msgs) =>
      schemaRegistry
        .deserialize({
          data: msgs[0].key,
          schema: keySchemaObject,
          schemaType: SCHEMA_TYPE_JSON,
        })
        .key.startsWith("key-"),
    "value contains 'firstName-' and 'lastName-' strings": (msgs) =>
      schemaRegistry
        .deserialize({
          data: msgs[0].value,
          schema: valueSchemaObject,
          schemaType: SCHEMA_TYPE_JSON,
        })
        .firstName.startsWith("firstName-") &&
      schemaRegistry
        .deserialize({
          data: msgs[0].value,
          schema: valueSchemaObject,
          schemaType: SCHEMA_TYPE_JSON,
        })
        .lastName.startsWith("lastName-"),
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
