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
  TOPIC_NAME_STRATEGY,
  RECORD_NAME_STRATEGY,
  SCHEMA_TYPE_AVRO,
} from "k6/x/kafka"; // import kafka extension

const brokers = ["localhost:9092"];
const topic = "com.example.person";

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

const keySchema = `{
  "name": "KeySchema",
  "type": "record",
  "namespace": "com.example.key",
  "fields": [
    {
      "name": "ssn",
      "type": "string"
    }
  ]
}
`;
const valueSchema = `{
  "name": "ValueSchema",
  "type": "record",
  "namespace": "com.example.value",
  "fields": [
    {
      "name": "firstName",
      "type": "string"
    },
    {
      "name": "lastName",
      "type": "string"
    }
  ]
}`;

const keySubjectName = schemaRegistry.getSubjectName({
  topic: topic,
  element: KEY,
  subjectNameStrategy: TOPIC_NAME_STRATEGY,
  schema: keySchema,
});

const valueSubjectName = schemaRegistry.getSubjectName({
  topic: topic,
  element: VALUE,
  subjectNameStrategy: RECORD_NAME_STRATEGY,
  schema: valueSchema,
});

const keySchemaObject = schemaRegistry.createSchema({
  subject: keySubjectName,
  schema: keySchema,
  schemaType: SCHEMA_TYPE_AVRO,
});

const valueSchemaObject = schemaRegistry.createSchema({
  subject: valueSubjectName,
  schema: valueSchema,
  schemaType: SCHEMA_TYPE_AVRO,
});

export default function () {
  for (let index = 0; index < 100; index++) {
    let messages = [
      {
        key: schemaRegistry.serialize({
          data: {
            ssn: "ssn-" + index,
          },
          schema: keySchemaObject,
          schemaType: SCHEMA_TYPE_AVRO,
        }),
        value: schemaRegistry.serialize({
          data: {
            firstName: "firstName-" + index,
            lastName: "lastName-" + index,
          },
          schema: valueSchemaObject,
          schemaType: SCHEMA_TYPE_AVRO,
        }),
      },
    ];
    writer.produce({ messages: messages });
  }

  let messages = reader.consume({ limit: 20 });
  check(messages, {
    "20 message returned": (msgs) => msgs.length == 20,
    "key starts with 'ssn-' string": (msgs) =>
      schemaRegistry
        .deserialize({
          data: msgs[0].key,
          schema: keySchemaObject,
          schemaType: SCHEMA_TYPE_AVRO,
        })
        .ssn.startsWith("ssn-"),
    "value contains 'firstName-' and 'lastName-' strings": (msgs) =>
      schemaRegistry
        .deserialize({
          data: msgs[0].value,
          schema: valueSchemaObject,
          schemaType: SCHEMA_TYPE_AVRO,
        })
        .firstName.startsWith("firstName-") &&
      schemaRegistry
        .deserialize({
          data: msgs[0].value,
          schema: valueSchemaObject,
          schemaType: SCHEMA_TYPE_AVRO,
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
