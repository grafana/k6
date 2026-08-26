// Integration test: inline Avro schemas without Schema Registry.
// Creates a SchemaRegistry in standalone mode and uses inline schemas.

import { Writer, Reader, Connection, SchemaRegistry, SCHEMA_TYPE_AVRO } from "k6/x/kafka";
import { thresholds, getBroker, verify, runTest, uniqueTopic } from "./lib/common.js";

export const options = { thresholds };

export default function () {
  runTest(() => {
    const broker = getBroker();
    const topic = uniqueTopic("xk6_standalone");
    const avroSchema = JSON.stringify({
      type: "record",
      name: "Event",
      fields: [
        { name: "timestamp", type: "long" },
        { name: "event", type: "string" },
      ],
    });

    const connection = new Connection({ address: broker });
    try {
      connection.createTopic({ topic, numPartitions: 1, replicationFactor: 1 });

      // Create SchemaRegistry in standalone mode (no registry URL)
      const sr = new SchemaRegistry();

      // Use inline schema (no registry ID needed)
      const schema = {
        schema: avroSchema,
        schemaType: SCHEMA_TYPE_AVRO,
      };

      const writer = new Writer({ brokers: [broker], topic });
      try {
        writer.produce({
          messages: [
            {
              value: sr.serialize({
                data: { timestamp: 1234567890, event: "startup" },
                schemaType: SCHEMA_TYPE_AVRO,
                schema: schema,
              }),
            },
          ],
        });
        verify("produced inline avro message", true);

        const reader = new Reader({ brokers: [broker], topic, groupID: "xk6_standalone" });
        try {
          const messages = reader.consume({ limit: 1 });
          verify("consumed message", messages.length === 1);
          if (messages.length > 0) {
            const decoded = sr.deserialize({
              data: messages[0].value,
              schemaType: SCHEMA_TYPE_AVRO,
              schema: schema,
            });
            verify("decoded inline avro", decoded && decoded.event === "startup");
          }
        } finally {
          reader.close();
        }
      } finally {
        writer.close();
      }

      try {
        connection.deleteTopic(topic);
      } catch (e) {
        // Ignore topic deletion errors
      }
    } finally {
      connection.close();
    }
  });
}
