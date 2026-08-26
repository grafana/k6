// Integration test: JSON serdes with Schema Registry.
// Registers a JSON schema, produces with it, and consumes to verify round-trip.

import { Writer, Reader, Connection, SchemaRegistry, SCHEMA_TYPE_JSON } from "k6/x/kafka";
import { thresholds, getBroker, getSchemaRegistry, verify, runTest, uniqueTopic } from "./lib/common.js";

export const options = { thresholds };

export default function () {
  runTest(() => {
    const srURL = getSchemaRegistry();
    if (!srURL) {
      verify("schema registry available", false);
      return;
    }

    const broker = getBroker();
    const topic = uniqueTopic("xk6_json");
    const jsonSchema = JSON.stringify({
      type: "object",
      properties: {
        id: { type: "integer" },
        message: { type: "string" },
      },
      required: ["id", "message"],
    });

    const connection = new Connection({ address: broker });
    try {
      connection.createTopic({ topic, numPartitions: 1, replicationFactor: 1 });

      const sr = new SchemaRegistry({ url: srURL });
      const schema = sr.createSchema({
        subject: `${topic}-value`,
        schema: jsonSchema,
        schemaType: SCHEMA_TYPE_JSON,
      });
      verify("created json schema", schema && schema.id > 0);

      const writer = new Writer({ brokers: [broker], topic });
      try {
        writer.produce({
          messages: [
            {
              value: sr.serialize({
                data: { id: 42, message: "hello" },
                schemaType: SCHEMA_TYPE_JSON,
                schema: schema,
              }),
            },
          ],
        });
        verify("produced json message", true);

        const reader = new Reader({ brokers: [broker], topic, groupID: "xk6_json" });
        try {
          const messages = reader.consume({ limit: 1 });
          verify("consumed message", messages.length === 1);
          if (messages.length > 0) {
            const decoded = sr.deserialize({
              data: messages[0].value,
              schemaType: SCHEMA_TYPE_JSON,
              schema: schema,
            });
            verify("decoded json", decoded && decoded.id === 42 && decoded.message === "hello");
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
