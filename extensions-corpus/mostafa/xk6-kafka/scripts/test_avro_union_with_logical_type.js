/*
This is a k6 test script that imports the xk6-kafka and
tests Avro union types with logical types (e.g., int with logicalType "date").
This test verifies the fix for issue #376 where wrapped primitive types
in unions (e.g., {"int": value} or {"int.date": value}) are correctly handled.
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
const topic = "com.example.document";

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
  "name": "DocumentKey",
  "type": "record",
  "namespace": "com.example",
  "fields": [
    {
      "name": "id",
      "type": "string"
    }
  ]
}`;

const valueSchema = `{
  "name": "Document",
  "type": "record",
  "namespace": "com.example",
  "fields": [
    {
      "name": "documentValidTo",
      "type": [
        "null",
        {
          "type": "int",
          "logicalType": "date"
        }
      ]
    },
    {
      "name": "documentValidFrom",
      "type": [
        "null",
        {
          "type": "int",
          "logicalType": "date"
        }
      ]
    },
    {
      "name": "status",
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
  // Test case 1: Wrapped int format {"int": value} - should work after fix
  let messages1 = [
    {
      key: schemaRegistry.serialize({
        data: {
          id: "doc-1",
        },
        schema: keySchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      }),
      value: schemaRegistry.serialize({
        data: {
          documentValidTo: { int: 20474 }, // Wrapped format
          documentValidFrom: null,
          status: "active",
        },
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      }),
    },
  ];
  writer.produce({ messages: messages1 });

  // Test case 2: Wrapped int with logical type suffix {"int.date": value} - should work after fix
  let messages2 = [
    {
      key: schemaRegistry.serialize({
        data: {
          id: "doc-2",
        },
        schema: keySchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      }),
      value: schemaRegistry.serialize({
        data: {
          documentValidTo: { "int.date": 20475 }, // Wrapped with logical type suffix
          documentValidFrom: { int: 20400 },
          status: "pending",
        },
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      }),
    },
  ];
  writer.produce({ messages: messages2 });

  // Test case 3: Null values - should work (this was already working)
  let messages3 = [
    {
      key: schemaRegistry.serialize({
        data: {
          id: "doc-3",
        },
        schema: keySchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      }),
      value: schemaRegistry.serialize({
        data: {
          documentValidTo: null,
          documentValidFrom: null,
          status: "draft",
        },
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      }),
    },
  ];
  writer.produce({ messages: messages3 });

  // Test case 4: Unwrapped int format (direct value) - should also work
  let messages4 = [
    {
      key: schemaRegistry.serialize({
        data: {
          id: "doc-4",
        },
        schema: keySchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      }),
      value: schemaRegistry.serialize({
        data: {
          documentValidTo: 20476, // Direct unwrapped value
          documentValidFrom: { int: 20401 },
          status: "archived",
        },
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      }),
    },
  ];
  writer.produce({ messages: messages4 });

  // Consume and verify messages
  sleep(1);
  let messages = reader.consume({ limit: 4 });

  check(messages, {
    "4 messages returned": (msgs) => msgs.length == 4,
    "first message has wrapped int format": (msgs) => {
      const deserialized = schemaRegistry.deserialize({
        data: msgs[0].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      return (
        deserialized.status === "active" &&
        deserialized.documentValidTo !== null
      );
    },
    "second message has wrapped int.date format": (msgs) => {
      const deserialized = schemaRegistry.deserialize({
        data: msgs[1].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      return (
        deserialized.status === "pending" &&
        deserialized.documentValidTo !== null
      );
    },
    "third message has null values": (msgs) => {
      const deserialized = schemaRegistry.deserialize({
        data: msgs[2].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      return (
        deserialized.status === "draft" &&
        deserialized.documentValidTo === null &&
        deserialized.documentValidFrom === null
      );
    },
    "fourth message has unwrapped int format": (msgs) => {
      const deserialized = schemaRegistry.deserialize({
        data: msgs[3].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      return (
        deserialized.status === "archived" &&
        deserialized.documentValidTo !== null
      );
    },
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
