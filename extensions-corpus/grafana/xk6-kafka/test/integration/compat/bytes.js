// Compatibility fixture: modernized port of mostafa/xk6-kafka scripts/test_bytes.js.
// Community v1 BYTES serdes produce/consume. Broker from KAFKA_BROKER; loop
// trimmed; original check() assertions preserved; `checks` threshold added.
import { check, sleep } from "k6";
import { Writer, Reader, Connection, SchemaRegistry, SCHEMA_TYPE_BYTES } from "k6/x/kafka";
import { thresholds, runTest, getBroker } from "../lib/common.js";

const brokers = [getBroker()];
const topic = "xk6_kafka_byte_array_topic";

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

const toBytes = (s) => Array.from(s, (c) => c.charCodeAt(0));

export default function () {
  runTest(() => {
    for (let index = 0; index < 10; index++) {
      writer.produce({
        messages: [
          {
            key: schemaRegistry.serialize({ data: toBytes("test-id-abc-" + index), schemaType: SCHEMA_TYPE_BYTES }),
            value: schemaRegistry.serialize({ data: toBytes("byte array payload"), schemaType: SCHEMA_TYPE_BYTES }),
          },
        ],
      });
    }

    const messages = reader.consume({ limit: 10 });
    check(messages, {
      "10 messages are received": (messages) => messages.length == 10,
    });
    check(messages[0], {
      "Key is a byte array and starts with 'test-id-'": (msg) =>
        String.fromCharCode(...schemaRegistry.deserialize({ data: msg.key, schemaType: SCHEMA_TYPE_BYTES })).startsWith("test-id-"),
      "Value is a byte array and is correct": (msg) =>
        String.fromCharCode(...schemaRegistry.deserialize({ data: msg.value, schemaType: SCHEMA_TYPE_BYTES })) == "byte array payload",
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
