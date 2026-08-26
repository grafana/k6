import { check } from "k6";
import {
  AdminClient,
  Consumer,
  KEY,
  Producer,
  SCHEMA_TYPE_JSON,
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

const topic = topicName("integration-json-schema-registry");

const adminClient = new AdminClient({ brokers });
const producer = new Producer({ brokers, topic });
const consumer = new Consumer({ brokers, topic });
const schemaRegistry = new SchemaRegistry({ url: schemaRegistryURL });

const keySchema = JSON.stringify({
  title: "V2Key",
  type: "object",
  properties: {
    key: {
      type: "string",
    },
  },
});

const valueSchema = JSON.stringify({
  title: "V2Value",
  type: "object",
  properties: {
    firstName: {
      type: "string",
    },
    lastName: {
      type: "string",
    },
  },
});

const keySubject = schemaRegistry.getSubjectName({
  topic,
  element: KEY,
  subjectNameStrategy: TOPIC_NAME_STRATEGY,
  schema: keySchema,
});

const valueSubject = schemaRegistry.getSubjectName({
  topic,
  element: VALUE,
  subjectNameStrategy: TOPIC_NAME_STRATEGY,
  schema: valueSchema,
});

const keySchemaObject = schemaRegistry.createSchema({
  subject: keySubject,
  schema: keySchema,
  schemaType: SCHEMA_TYPE_JSON,
});

const valueSchemaObject = schemaRegistry.createSchema({
  subject: valueSubject,
  schema: valueSchema,
  schemaType: SCHEMA_TYPE_JSON,
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
          data: { key: "json-key-1" },
          schema: keySchemaObject,
          schemaType: SCHEMA_TYPE_JSON,
        }),
        value: schemaRegistry.serialize({
          data: { firstName: "Grace", lastName: "Hopper" },
          schema: valueSchemaObject,
          schemaType: SCHEMA_TYPE_JSON,
        }),
      },
    ],
  });

  const messages = consumer.consume({ maxMessages: 1 });

  check(messages, {
    "json schema script receives one message": (received) =>
      received.length === 1,
    "json schema key deserializes": (received) =>
      received.length === 1 &&
      schemaRegistry.deserialize({
        data: received[0].key,
        schema: keySchemaObject,
        schemaType: SCHEMA_TYPE_JSON,
      }).key === "json-key-1",
    "json schema value deserializes": (received) => {
      if (received.length !== 1) {
        return false;
      }

      const value = schemaRegistry.deserialize({
        data: received[0].value,
        schema: valueSchemaObject,
        schemaType: SCHEMA_TYPE_JSON,
      });
      return value.firstName === "Grace" && value.lastName === "Hopper";
    },
  });
}

export function teardown() {
  deleteTopic(adminClient, topic);
  producer.close();
  consumer.close();
  adminClient.close();
}
