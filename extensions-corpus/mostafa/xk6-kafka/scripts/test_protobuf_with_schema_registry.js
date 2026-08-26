/*
This is a k6 test script that imports xk6-kafka and
tests Kafka with Protobuf messages via Schema Registry.
*/

import { check, sleep } from "k6";
import {
  Writer,
  Reader,
  Connection,
  SchemaRegistry,
  VALUE,
  RECORD_NAME_STRATEGY,
  SCHEMA_TYPE_PROTOBUF,
} from "k6/x/kafka";

const brokers = ["localhost:9092"];
const topic = "xk6_kafka_protobuf_topic";

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

  const topics = connection.listTopics();
  if (!topics.includes(topic)) {
    throw new Error(`Topic ${topic} was not created successfully`);
  }

  connection.close();
  sleep(2);
}

const valueSchema = `
syntax = "proto3";
package com.example.protobuf;

message Person {
  string first_name = 1;
  string last_name = 2;
  int32 age = 3;
}
`;

const messageName = "com.example.protobuf.Person";

const valueSubjectName = schemaRegistry.getSubjectName({
  topic: topic,
  element: VALUE,
  subjectNameStrategy: RECORD_NAME_STRATEGY,
  schema: valueSchema,
  messageName: messageName,
});

const valueSchemaObject = schemaRegistry.createSchema({
  subject: valueSubjectName,
  schema: valueSchema,
  schemaType: SCHEMA_TYPE_PROTOBUF,
  messageName: messageName,
});

export default function () {
  const messages = [
    {
      value: schemaRegistry.serialize({
        data: {
          firstName: "mostafa",
          lastName: "moradian",
          age: 33,
        },
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_PROTOBUF,
      }),
    },
  ];

  writer.produce({ messages: messages });

  const consumed = reader.consume({ limit: 1 });
  check(consumed, {
    "1 message returned": (msgs) => msgs.length === 1,
    "protobuf payload is decoded": (msgs) => {
      const value = schemaRegistry.deserialize({
        data: msgs[0].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_PROTOBUF,
      });

      return (
        value.firstName === "mostafa" &&
        value.lastName === "moradian" &&
        value.age === 33
      );
    },
  });
}

export function teardown() {
  const connection = new Connection({
    address: brokers[0],
  });
  connection.deleteTopic(topic);
  connection.close();
  writer.close();
  reader.close();
}
