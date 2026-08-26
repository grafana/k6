// Integration test: STRING and BYTES serdes.
// Tests simple serdes without schemas.

import { Writer, Reader, Connection, SchemaRegistry, SCHEMA_TYPE_STRING, SCHEMA_TYPE_BYTES } from "k6/x/kafka";
import { thresholds, getBroker, verify, runTest, uniqueTopic, toStr } from "./lib/common.js";

export const options = { thresholds };

export default function () {
  runTest(() => {
    const broker = getBroker();
    const topicStr = uniqueTopic("xk6_string");
    const topicBytes = uniqueTopic("xk6_bytes");

    const connection = new Connection({ address: broker });
    try {
      connection.createTopic({ topic: topicStr, numPartitions: 1, replicationFactor: 1 });
      connection.createTopic({ topic: topicBytes, numPartitions: 1, replicationFactor: 1 });

      const sr = new SchemaRegistry();

      // Test STRING
      const writer = new Writer({ brokers: [broker], topic: topicStr });
      try {
        writer.produce({
          messages: [
            {
              value: sr.serialize({
                data: "hello world",
                schemaType: SCHEMA_TYPE_STRING,
              }),
            },
          ],
        });
        verify("produced string message", true);
      } finally {
        writer.close();
      }

      const readerStr = new Reader({ brokers: [broker], topic: topicStr, groupID: "xk6_string" });
      try {
        const messages = readerStr.consume({ limit: 1 });
        verify("consumed string message", messages.length === 1);
        if (messages.length > 0) {
          const decoded = sr.deserialize({
            data: messages[0].value,
            schemaType: SCHEMA_TYPE_STRING,
          });
          verify("decoded string", decoded === "hello world");
        }
      } finally {
        readerStr.close();
      }

      // Test BYTES
      const testBytes = new Uint8Array([0x48, 0x65, 0x6c, 0x6c, 0x6f]); // "Hello"
      const writerBytes = new Writer({ brokers: [broker], topic: topicBytes });
      try {
        writerBytes.produce({
          messages: [
            {
              value: sr.serialize({
                data: testBytes,
                schemaType: SCHEMA_TYPE_BYTES,
              }),
            },
          ],
        });
        verify("produced bytes message", true);
      } finally {
        writerBytes.close();
      }

      const readerBytes = new Reader({ brokers: [broker], topic: topicBytes, groupID: "xk6_bytes" });
      try {
        const messages = readerBytes.consume({ limit: 1 });
        verify("consumed bytes message", messages.length === 1);
        if (messages.length > 0) {
          const decoded = sr.deserialize({
            data: messages[0].value,
            schemaType: SCHEMA_TYPE_BYTES,
          });
          verify("decoded bytes", toStr(decoded) === "Hello");
        }
      } finally {
        readerBytes.close();
      }

      try {
        connection.deleteTopic(topicStr);
        connection.deleteTopic(topicBytes);
      } catch (e) {
        // Ignore topic deletion errors
      }
    } finally {
      connection.close();
    }
  });
}
