// Compatibility fixture: modernized port of mostafa/xk6-kafka scripts/test_string.js.
// Community v1 STRING serdes produce/consume. Broker from KAFKA_BROKER; loop
// trimmed; original check() assertions preserved; `checks` threshold added.
import { check, sleep } from "k6";
import { Writer, Reader, Connection, SchemaRegistry, SCHEMA_TYPE_STRING } from "k6/x/kafka";
import { thresholds, runTest, getBroker } from "../lib/common.js";

const brokers = [getBroker()];
const topic = "xk6_kafka_string_topic";

const writer = new Writer({ brokers, topic });
const reader = new Reader({ brokers, topic });
const schemaRegistry = new SchemaRegistry();

export const options = {
  thresholds: {
    ...thresholds,
    kafka_writer_error_count: ["count == 0"],
    kafka_reader_error_count: ["count == 0"],
  },
};

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

export default function () {
  runTest(() => {
    for (let index = 0; index < 10; index++) {
      writer.produce({
        messages: [
          {
            key: schemaRegistry.serialize({ data: "test-key-string", schemaType: SCHEMA_TYPE_STRING }),
            value: schemaRegistry.serialize({ data: "test-value-string", schemaType: SCHEMA_TYPE_STRING }),
            headers: { mykey: "myvalue" },
            time: new Date(),
          },
        ],
      });
    }

    const messages = reader.consume({ limit: 10 });
    check(messages, {
      "10 messages are received": (messages) => messages.length == 10,
    });
    check(messages[0], {
      "Key is a string and is correct": (msg) =>
        schemaRegistry.deserialize({ data: msg.key, schemaType: SCHEMA_TYPE_STRING }) == "test-key-string",
      "Value is a string and is correct": (msg) =>
        typeof schemaRegistry.deserialize({ data: msg.value, schemaType: SCHEMA_TYPE_STRING }) == "string" &&
        schemaRegistry.deserialize({ data: msg.value, schemaType: SCHEMA_TYPE_STRING }) == "test-value-string",
    });
  });
}

export function teardown() {
  reader.close();
  writer.close();
  const connection = new Connection({ address: brokers[0] });
  connection.deleteTopic(topic);
  connection.close();
}
