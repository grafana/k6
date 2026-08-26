// Compatibility fixture: modernized port of mostafa/xk6-kafka
// scripts/test_avro_with_schema_registry.js.
//
// Community v1 Avro serdes with Schema Registry, using both TOPIC_NAME_STRATEGY
// (key) and RECORD_NAME_STRATEGY (value) subject naming. Broker/registry from
// env; loop trimmed; original check() assertions preserved; `checks` threshold
// added. Requires SCHEMA_REGISTRY_URL (set by `make integration` / CI).
import { check, sleep } from "k6";
import {
  Writer, Reader, Connection, SchemaRegistry,
  KEY, VALUE, TOPIC_NAME_STRATEGY, RECORD_NAME_STRATEGY, SCHEMA_TYPE_AVRO,
} from "k6/x/kafka";
import { thresholds, runTest, getBroker, getSchemaRegistry } from "../lib/common.js";

const brokers = [getBroker()];
const registryURL = getSchemaRegistry();
if (!registryURL) {
  throw new Error("compat/avro requires SCHEMA_REGISTRY_URL");
}
const topic = "com.example.person";

const writer = new Writer({ brokers, topic });
const reader = new Reader({ brokers, topic });
const schemaRegistry = new SchemaRegistry({ url: registryURL });

export const options = {
  thresholds: {
    ...thresholds,
    kafka_writer_error_count: ["count == 0"],
    kafka_reader_error_count: ["count == 0"],
  },
};

const keySchema = `{"name":"KeySchema","type":"record","namespace":"com.example.key","fields":[{"name":"ssn","type":"string"}]}`;
const valueSchema = `{"name":"ValueSchema","type":"record","namespace":"com.example.value","fields":[{"name":"firstName","type":"string"},{"name":"lastName","type":"string"}]}`;

export function setup() {
  const connection = new Connection({ address: brokers[0] });
  if (connection.listTopics().includes(topic)) {
    connection.deleteTopic(topic);
    sleep(2);
  }
  connection.createTopic({ topic });
  connection.close();
  sleep(2);
}

const keySubject = schemaRegistry.getSubjectName({ topic, element: KEY, subjectNameStrategy: TOPIC_NAME_STRATEGY, schema: keySchema });
const valueSubject = schemaRegistry.getSubjectName({ topic, element: VALUE, subjectNameStrategy: RECORD_NAME_STRATEGY, schema: valueSchema });

const keySchemaObject = schemaRegistry.createSchema({ subject: keySubject, schema: keySchema, schemaType: SCHEMA_TYPE_AVRO });
const valueSchemaObject = schemaRegistry.createSchema({ subject: valueSubject, schema: valueSchema, schemaType: SCHEMA_TYPE_AVRO });

export default function () {
  runTest(() => {
    for (let index = 0; index < 20; index++) {
      writer.produce({
        messages: [
          {
            key: schemaRegistry.serialize({ data: { ssn: "ssn-" + index }, schema: keySchemaObject, schemaType: SCHEMA_TYPE_AVRO }),
            value: schemaRegistry.serialize({
              data: { firstName: "firstName-" + index, lastName: "lastName-" + index },
              schema: valueSchemaObject, schemaType: SCHEMA_TYPE_AVRO,
            }),
          },
        ],
      });
    }

    const messages = reader.consume({ limit: 20 });
    check(messages, {
      "20 message returned": (msgs) => msgs.length == 20,
      "key starts with 'ssn-' string": (msgs) =>
        schemaRegistry.deserialize({ data: msgs[0].key, schema: keySchemaObject, schemaType: SCHEMA_TYPE_AVRO }).ssn.startsWith("ssn-"),
      "value contains 'firstName-' and 'lastName-' strings": (msgs) =>
        schemaRegistry.deserialize({ data: msgs[0].value, schema: valueSchemaObject, schemaType: SCHEMA_TYPE_AVRO }).firstName.startsWith("firstName-") &&
        schemaRegistry.deserialize({ data: msgs[0].value, schema: valueSchemaObject, schemaType: SCHEMA_TYPE_AVRO }).lastName.startsWith("lastName-"),
    });
  });
}

export function teardown() {
  const connection = new Connection({ address: brokers[0] });
  connection.deleteTopic(topic);
  connection.close();
  writer.close();
  reader.close();
}
