import { check } from "k6";
import {
  AdminClient,
  Consumer,
  KEY,
  Producer,
  RECORD_NAME_STRATEGY,
  SCHEMA_TYPE_AVRO,
  SchemaRegistry,
  TOPIC_NAME_STRATEGY,
  VALUE,
} from "k6/x/kafka";

import {
  brokers,
  createTopic,
  deleteTopic,
  schemaRegistryURL,
  topicName,
} from "../common.js";

const topic = topicName("integration-avro-schema-registry");

const adminClient = new AdminClient({ brokers });
const producer = new Producer({ brokers, topic });
const consumer = new Consumer({ brokers, topic });
const schemaRegistry = new SchemaRegistry({ url: schemaRegistryURL });

const keySchema = `{
  "name": "V2KeySchema",
  "type": "record",
  "namespace": "com.example.key",
  "fields": [
    {
      "name": "id",
      "type": "string"
    }
  ]
}`;

const valueSchema = `{
  "name": "V2ValueSchema",
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

const keySubject = schemaRegistry.getSubjectName({
  topic,
  element: KEY,
  subjectNameStrategy: TOPIC_NAME_STRATEGY,
  schema: keySchema,
});

const valueSubject = schemaRegistry.getSubjectName({
  topic,
  element: VALUE,
  subjectNameStrategy: RECORD_NAME_STRATEGY,
  schema: valueSchema,
});

const keySchemaObject = schemaRegistry.createSchema({
  subject: keySubject,
  schema: keySchema,
  schemaType: SCHEMA_TYPE_AVRO,
});

const valueSchemaObject = schemaRegistry.createSchema({
  subject: valueSubject,
  schema: valueSchema,
  schemaType: SCHEMA_TYPE_AVRO,
});

export const options = {
  vus: 1,
  iterations: 1,
};

export function setup() {
  createTopic(adminClient, topic);
}

export default function () {
  producer.produce({
    messages: [
      {
        key: schemaRegistry.serialize({
          data: { id: "avro-key-1" },
          schema: keySchemaObject,
          schemaType: SCHEMA_TYPE_AVRO,
        }),
        value: schemaRegistry.serialize({
          data: { firstName: "Ada", lastName: "Lovelace" },
          schema: valueSchemaObject,
          schemaType: SCHEMA_TYPE_AVRO,
        }),
      },
    ],
  });

  const messages = consumer.consume({ maxMessages: 1 });

  check(messages, {
    "avro script receives one message": (received) => received.length === 1,
    "avro key deserializes": (received) =>
      received.length === 1 &&
      schemaRegistry.deserialize({
        data: received[0].key,
        schema: keySchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      }).id === "avro-key-1",
    "avro value deserializes": (received) => {
      if (received.length !== 1) {
        return false;
      }

      const value = schemaRegistry.deserialize({
        data: received[0].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_AVRO,
      });
      return value.firstName === "Ada" && value.lastName === "Lovelace";
    },
  });
}

export function teardown() {
  deleteTopic(adminClient, topic);
  producer.close();
  consumer.close();
  adminClient.close();
}
