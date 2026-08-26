// Compatibility fixture (MIGRATED, not verbatim parity): based on
// mostafa/xk6-kafka scripts/test_json.js.
//
// Unlike the other compat fixtures — which run the community script essentially
// unchanged (env wiring only) as parity evidence — this one required a
// migration: the community script does SCHEMALESS JSON serdes, but this
// extension's JSON serdes require a schema object (standalone = a schema whose
// `id` is absent, used for validation + pure-JSON encoding; see
// openspec/specs/json-serdes and compat/README.md). A standalone JSON schema is
// therefore added. It documents "how a v1 JSON script migrates", not "the v1
// JSON script runs unchanged". The original check() assertions are preserved;
// broker is from env; the produce loop is trimmed; the body is wrapped in
// runTest so an exception fails the run (a bare checks-rate threshold passes
// vacuously on zero checks).
import { check, sleep } from "k6";
import { Writer, Reader, Connection, SchemaRegistry, CODEC_SNAPPY, SCHEMA_TYPE_JSON } from "k6/x/kafka";
import { thresholds, runTest, getBroker } from "../lib/common.js";

const brokers = [getBroker()];
const topic = "xk6_kafka_json_topic";

const writer = new Writer({ brokers, topic, compression: CODEC_SNAPPY });
const reader = new Reader({ brokers, topic });
const schemaRegistry = new SchemaRegistry();

const keySchema = {
  schema: JSON.stringify({ type: "object", properties: { correlationId: { type: "string" } }, required: ["correlationId"] }),
  schemaType: SCHEMA_TYPE_JSON,
};
const valueSchema = {
  schema: JSON.stringify({ type: "object", properties: { name: { type: "string" } }, required: ["name"] }),
  schemaType: SCHEMA_TYPE_JSON,
};

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
  connection.createTopic({
    topic,
    configEntries: [{ configName: "compression.type", configValue: CODEC_SNAPPY }],
  });
  connection.close();
  sleep(2);
}

export default function () {
  runTest(() => {
    for (let index = 0; index < 10; index++) {
      writer.produce({
        messages: [
          {
            key: schemaRegistry.serialize({ data: { correlationId: "test-id-abc-" + index }, schema: keySchema, schemaType: SCHEMA_TYPE_JSON }),
            value: schemaRegistry.serialize({ data: { name: "xk6-kafka" }, schema: valueSchema, schemaType: SCHEMA_TYPE_JSON }),
            headers: { mykey: "myvalue" },
            time: new Date(),
          },
          {
            key: schemaRegistry.serialize({ data: { correlationId: "test-id-def-" + index }, schema: keySchema, schemaType: SCHEMA_TYPE_JSON }),
            value: schemaRegistry.serialize({ data: { name: "xk6-kafka" }, schema: valueSchema, schemaType: SCHEMA_TYPE_JSON }),
            headers: { mykey: "myvalue" },
          },
        ],
      });
    }

    const messages = reader.consume({ limit: 10 });
    check(messages, {
      "10 messages are received": (messages) => messages.length == 10,
    });
    check(messages[0], {
      "Topic equals to xk6_kafka_json_topic": (msg) => msg["topic"] == topic,
      "Key contains key/value and is JSON": (msg) =>
        schemaRegistry.deserialize({ data: msg.key, schema: keySchema, schemaType: SCHEMA_TYPE_JSON }).correlationId.startsWith("test-id-"),
      "Value contains key/value and is JSON": (msg) =>
        schemaRegistry.deserialize({ data: msg.value, schema: valueSchema, schemaType: SCHEMA_TYPE_JSON }).name == "xk6-kafka",
      "Header equals {'mykey': 'myvalue'}": (msg) =>
        "mykey" in msg.headers && String.fromCharCode(...msg.headers["mykey"]) == "myvalue",
      "Time is past": (msg) => new Date(msg["time"]) < new Date(),
      "Partition is zero": (msg) => msg["partition"] == 0,
      "Offset is gte zero": (msg) => msg["offset"] >= 0,
      "High watermark is gte zero": (msg) => msg["highWaterMark"] >= 0,
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
